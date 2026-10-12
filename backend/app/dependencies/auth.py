import uuid
from typing import Optional, Dict, Any
from fastapi import Depends, HTTPException, status
from fastapi.security import HTTPBearer, HTTPAuthorizationCredentials
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy import select

from app.dependencies.db import get_db
from app.models import AdminUser, UserRole, UserStatus
from app.utils.security import decode_token

security_scheme = HTTPBearer(auto_error=False)

async def get_current_user(
    credentials: Optional[HTTPAuthorizationCredentials] = Depends(security_scheme),
    db: AsyncSession = Depends(get_db)
) -> AdminUser:
    """Extract and validate JWT bearer token, returning the current AdminUser."""
    if not credentials or not credentials.credentials:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Authentication credentials were not provided",
            headers={"WWW-Authenticate": "Bearer"},
        )
    
    payload = decode_token(credentials.credentials)
    if not payload or payload.get("token_type") != "access":
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid or expired access token",
            headers={"WWW-Authenticate": "Bearer"},
        )
    
    user_id_str = payload.get("sub")
    if not user_id_str:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Token missing user identity",
            headers={"WWW-Authenticate": "Bearer"},
        )
    
    try:
        user_uuid = uuid.UUID(user_id_str)
    except ValueError:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid user ID format in token",
        )
    
    stmt = select(AdminUser).where(AdminUser.id == user_uuid)
    result = await db.execute(stmt)
    user = result.scalar_one_or_none()

    if not user:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="User account no longer exists",
        )
    
    if user.status != UserStatus.ACTIVE:
        raise HTTPException(
            status_code=status.HTTP_403_FORBIDDEN,
            detail="User account has been deactivated",
        )
    
    return user

async def require_admin(
    current_user: AdminUser = Depends(get_current_user)
) -> AdminUser:
    """Ensure the current authenticated user has ADMIN role."""
    if current_user.role != UserRole.ADMIN:
        raise HTTPException(
            status_code=status.HTTP_403_FORBIDDEN,
            detail="Administrator access required for this operation",
        )
    return current_user

async def require_mfa(
    credentials: Optional[HTTPAuthorizationCredentials] = Depends(security_scheme),
    current_user: AdminUser = Depends(require_admin)
) -> AdminUser:
    """
    For Admin users with Supabase 2FA enabled, verify that the token
    contains the mfa_verified=True claim.
    """
    if current_user.is_2fa_enabled:
        if not credentials or not credentials.credentials:
            raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="Missing authorization header")
        
        payload = decode_token(credentials.credentials)
        if not payload or not payload.get("mfa_verified", False):
            raise HTTPException(
                status_code=status.HTTP_403_FORBIDDEN,
                detail="Supabase 2FA verification required before accessing this resource",
                headers={"X-MFA-Required": "true"}
            )
    return current_user

async def require_dm(
    current_user: AdminUser = Depends(get_current_user)
) -> AdminUser:
    """Ensure the current authenticated user has DM or Admin role."""
    if current_user.role not in [UserRole.DISTRICT_MANAGER, UserRole.ADMIN]:
        raise HTTPException(
            status_code=status.HTTP_403_FORBIDDEN,
            detail="District Manager or Administrator access required for this operation",
        )
    return current_user
