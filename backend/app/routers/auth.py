from datetime import datetime, timezone
from fastapi import APIRouter, Depends, HTTPException, status, Response, Request
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy import select

from app.dependencies.db import get_db
from app.dependencies.auth import get_current_user, require_admin
from app.models import AdminUser, UserStatus, UserRole, AuditLog
from app.schemas.auth import (
    LoginRequest, TokenResponse, UserProfileSchema,
    UpdateCredentialsRequest, ChangePasswordRequest, UpdateProfileRequest,
    MFAEnrollResponse, Verify2FARequest, MFAVerifyResponse
)
from app.utils.security import (
    verify_password, get_password_hash, create_access_token,
    create_refresh_token, decode_token
)
from app.services.auth_service import AuthService
from app.services.go_client import GoServiceClient
from app.config import settings
from app.dependencies.rate_limit import rate_limit_auth

router = APIRouter()

@router.post("/login", response_model=TokenResponse, dependencies=[Depends(rate_limit_auth(10, 60))])
async def login(
    payload: LoginRequest,
    request: Request,
    response: Response,
    db: AsyncSession = Depends(get_db)
):
    """
    Authenticate Admin or District Manager.
    Accepts email or DMID in the 'email' field.
    Checks EODDB_Users via Go service, with fallback to AdminUser.
    """
    identifier = payload.email.strip()
    user = None

    # Authenticate strictly against DMInfo in CLAB-EOD via Go microservice
    go_user = await GoServiceClient.verify_user(identifier, payload.password)
    if not go_user:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid credentials. Please check your username/ID and password."
        )

    # Determine role based on DMInfo Role field
    role = UserRole.ADMIN if "admin" in go_user.get("role", "").lower() else UserRole.DISTRICT_MANAGER

    # Sync user session record
    stmt = select(AdminUser).where(
        (AdminUser.dmid == go_user["id"]) | 
        (AdminUser.name == go_user["district_manager"]) |
        (AdminUser.email == identifier.lower())
    )
    res = await db.execute(stmt)
    user = res.scalar_one_or_none()

    if not user:
        user = AdminUser(
            dmid=go_user["id"],
            name=go_user["district_manager"],
            email=f"{go_user['id'].lower()}@eod.internal",
            password_hash=get_password_hash(payload.password),
            role=role,
            status=UserStatus.ACTIVE,
            is_demo_creds=False
        )
        db.add(user)
        await db.commit()
        await db.refresh(user)
    else:
        # Keep role and name in sync with DMInfo in CLAB-EOD
        user.role = role
        user.name = go_user["district_manager"]
        user.status = UserStatus.ACTIVE
        user.is_demo_creds = False
        await db.commit()
        await db.refresh(user)

    if user.status != UserStatus.ACTIVE:
        raise HTTPException(
            status_code=status.HTTP_403_FORBIDDEN,
            detail="Your account has been deactivated. Please contact your system administrator."
        )

    # Check if 2FA is required for Admin
    mfa_required = user.role.value == "admin" and user.is_2fa_enabled

    token_data = {
        "sub": str(user.id),
        "role": user.role.value,
        "mfa_verified": not mfa_required,
    }

    access_token = create_access_token(token_data)
    refresh_token = create_refresh_token(token_data)

    # Set httpOnly refresh cookie
    secure_cookie = settings.ENVIRONMENT.lower() in ["production", "prod"] or not settings.DEBUG
    response.set_cookie(
        key="refresh_token",
        value=refresh_token,
        httponly=True,
        samesite="lax",
        secure=secure_cookie,
    )

    # Record login timestamp & audit log
    user.last_login_at = datetime.now(timezone.utc).replace(tzinfo=None)
    log = AuditLog(
        user_id=user.id,
        action="USER_LOGIN",
        resource_type="AdminUser",
        resource_id=str(user.id),
        details={"mfa_required": mfa_required},
        ip_address=request.client.host if request.client else None
    )
    db.add(log)
    await db.commit()

    user_profile = UserProfileSchema.model_validate(user)

    return TokenResponse(
        access_token=access_token,
        mfa_required=mfa_required,
        is_demo_creds=user.is_demo_creds,
        user=user_profile,
    )

@router.get("/me", response_model=UserProfileSchema)
async def get_me(current_user: AdminUser = Depends(get_current_user)):
    """Return currently authenticated user profile."""
    return UserProfileSchema.model_validate(current_user)

@router.put("/me", response_model=UserProfileSchema)
async def update_my_profile(
    payload: UpdateProfileRequest,
    current_user: AdminUser = Depends(get_current_user),
    db: AsyncSession = Depends(get_db)
):
    """Update current user profile information (Name, Email, DMID)."""
    if payload.name is not None:
        new_name = payload.name.strip()
        if new_name:
            current_user.name = new_name
            
    if payload.email is not None:
        new_email = payload.email.strip().lower()
        if new_email and new_email != current_user.email:
            stmt = select(AdminUser).where(AdminUser.email == new_email, AdminUser.id != current_user.id)
            res = await db.execute(stmt)
            if res.scalar_one_or_none():
                raise HTTPException(
                    status_code=status.HTTP_400_BAD_REQUEST,
                    detail=f"Email address '{new_email}' is already registered to another account."
                )
            current_user.email = new_email

    if payload.dmid is not None:
        new_dmid = payload.dmid.strip()
        if new_dmid and new_dmid != current_user.dmid:
            stmt = select(AdminUser).where(AdminUser.dmid == new_dmid, AdminUser.id != current_user.id)
            res = await db.execute(stmt)
            if res.scalar_one_or_none():
                raise HTTPException(
                    status_code=status.HTTP_400_BAD_REQUEST,
                    detail=f"DM ID '{new_dmid}' is already assigned to another account."
                )
            current_user.dmid = new_dmid

    await db.commit()
    await db.refresh(current_user)

    return UserProfileSchema.model_validate(current_user)

@router.post("/logout")
async def logout(response: Response):
    """Logout user by clearing refresh cookie."""
    response.delete_cookie("refresh_token")
    return {"message": "Successfully logged out"}

@router.post("/refresh", response_model=TokenResponse)
async def refresh_token(
    request: Request,
    response: Response,
    db: AsyncSession = Depends(get_db)
):
    """Obtain a new access token using the httpOnly refresh cookie."""
    cookie_token = request.cookies.get("refresh_token")
    if not cookie_token:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Refresh token cookie missing",
        )
    
    payload = decode_token(cookie_token)
    if not payload or payload.get("token_type") != "refresh":
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid or expired refresh token",
        )
    
    user_id_str = payload.get("sub")
    stmt = select(AdminUser).where(AdminUser.id == user_id_str)
    res = await db.execute(stmt)
    user = res.scalar_one_or_none()

    if not user or user.status != UserStatus.ACTIVE:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="User account is invalid or deactivated",
        )
    
    # Preserve previous mfa_verified status from the decoded refresh token
    was_mfa_verified = payload.get("mfa_verified", False)
    mfa_required = (user.role.value == "admin" and user.is_2fa_enabled and not was_mfa_verified)
    
    # If 2FA is not enabled or user already verified MFA in session, mark mfa_verified=True
    is_mfa_valid = was_mfa_verified if (user.role.value == "admin" and user.is_2fa_enabled) else True

    new_token_data = {
        "sub": str(user.id),
        "role": user.role.value,
        "mfa_verified": is_mfa_valid,
    }
    
    new_access_token = create_access_token(new_token_data)
    new_refresh_token = create_refresh_token(new_token_data)

    secure_cookie = settings.ENVIRONMENT.lower() in ["production", "prod"] or not settings.DEBUG
    response.set_cookie(
        key="refresh_token",
        value=new_refresh_token,
        httponly=True,
        samesite="lax",
        secure=secure_cookie,
    )

    return TokenResponse(
        access_token=new_access_token,
        mfa_required=mfa_required,
        is_demo_creds=user.is_demo_creds,
        user=UserProfileSchema.model_validate(user),
    )

@router.post("/admin/update-credentials", response_model=UserProfileSchema)
async def update_admin_credentials(
    payload: UpdateCredentialsRequest,
    request: Request,
    current_user: AdminUser = Depends(require_admin),
    db: AsyncSession = Depends(get_db)
):
    """
    Admin Only: Update email & password from initial demo credentials.
    Sets is_demo_creds = False and syncs with Supabase Auth.
    """
    updated_user = await AuthService.update_admin_credentials(
        db=db,
        user=current_user,
        new_email=payload.new_email,
        new_password=payload.new_password,
        ip_address=request.client.host if request.client else None
    )
    return UserProfileSchema.model_validate(updated_user)

@router.post("/admin/supabase-2fa/enroll", response_model=MFAEnrollResponse)
async def enroll_supabase_2fa(
    current_user: AdminUser = Depends(require_admin)
):
    """
    Admin Only: Generate Supabase 2FA TOTP enrollment details (Secret & QR Code).
    """
    mfa_data = await AuthService.generate_mfa_enrollment(current_user)
    return MFAEnrollResponse(**mfa_data)

@router.post("/admin/supabase-2fa/verify-enrollment", response_model=MFAVerifyResponse)
async def verify_supabase_2fa_enrollment(
    payload: Verify2FARequest,
    request: Request,
    response: Response,
    current_user: AdminUser = Depends(require_admin),
    db: AsyncSession = Depends(get_db)
):
    """
    Admin Only: Verify 6-digit TOTP code to confirm 2FA setup and activate Supabase 2FA.
    """
    success = await AuthService.verify_mfa_code(
        db=db,
        user=current_user,
        totp_code=payload.totp_code,
        secret=payload.secret,
        ip_address=request.client.host if request.client else None
    )

    if not success:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Invalid 6-digit TOTP code. Please check your Authenticator app and try again."
        )

    # Return upgraded token with mfa_verified=True
    token_data = {
        "sub": str(current_user.id),
        "role": current_user.role.value,
        "mfa_verified": True
    }
    upgraded_token = create_access_token(token_data)
    upgraded_refresh = create_refresh_token(token_data)

    secure_cookie = settings.ENVIRONMENT.lower() in ["production", "prod"] or not settings.DEBUG
    response.set_cookie(
        key="refresh_token",
        value=upgraded_refresh,
        httponly=True,
        samesite="lax",
        secure=secure_cookie,
    )

    return MFAVerifyResponse(
        success=True,
        message="Supabase 2FA enabled successfully!",
        access_token=upgraded_token,
        user=UserProfileSchema.model_validate(current_user)
    )

@router.post("/admin/supabase-2fa/verify-login", response_model=MFAVerifyResponse)
async def verify_supabase_2fa_login(
    payload: Verify2FARequest,
    request: Request,
    response: Response,
    current_user: AdminUser = Depends(get_current_user),
    db: AsyncSession = Depends(get_db)
):
    """
    Admin Only: Verify 6-digit TOTP code during login challenge to issue final mfa_verified access token.
    """
    if current_user.role != UserRole.ADMIN:
        raise HTTPException(status_code=status.HTTP_403_FORBIDDEN, detail="2FA login verification is for Admin users")

    success = await AuthService.verify_mfa_code(
        db=db,
        user=current_user,
        totp_code=payload.totp_code,
        secret=payload.secret,
        ip_address=request.client.host if request.client else None
    )

    if not success:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Invalid 6-digit 2FA code."
        )

    token_data = {
        "sub": str(current_user.id),
        "role": current_user.role.value,
        "mfa_verified": True
    }
    upgraded_token = create_access_token(token_data)
    upgraded_refresh = create_refresh_token(token_data)

    secure_cookie = settings.ENVIRONMENT.lower() in ["production", "prod"] or not settings.DEBUG
    response.set_cookie(
        key="refresh_token",
        value=upgraded_refresh,
        httponly=True,
        samesite="lax",
        secure=secure_cookie,
    )

    return MFAVerifyResponse(
        success=True,
        message="2FA authentication verified successfully!",
        access_token=upgraded_token,
        user=UserProfileSchema.model_validate(current_user)
    )

@router.post("/change-password", response_model=UserProfileSchema)
async def change_password(
    payload: ChangePasswordRequest,
    current_user: AdminUser = Depends(get_current_user),
    db: AsyncSession = Depends(get_db)
):
    """Change current user password."""
    curr_pwd = payload.current_password or payload.old_password
    if not curr_pwd or not verify_password(curr_pwd, current_user.password_hash):
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Current password is incorrect"
        )
    
    # Sync new password directly to Azure SQL DMInfo
    dm_identifier = current_user.dmid or current_user.name
    try:
        await GoServiceClient.update_password(dm_identifier, payload.new_password)
    except Exception as e:
        import logging
        logging.error(f"[CHANGE_PASSWORD] Failed to update password in Azure SQL DMInfo: {e}", exc_info=True)
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Failed to update password in database: {str(e)}"
        )

    current_user.password_hash = get_password_hash(payload.new_password)
    current_user.must_change_password = False
    current_user.is_demo_creds = False
    await db.commit()
    await db.refresh(current_user)

    return UserProfileSchema.model_validate(current_user)

