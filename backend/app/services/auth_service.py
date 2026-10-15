from typing import Optional, Dict, Any
from fastapi import HTTPException, status
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy import select
from supabase import create_client, Client

from app.config import settings
from app.models import AdminUser, UserStatus, UserRole, AuditLog
from app.utils.security import verify_password, get_password_hash
from app.utils.supabase import get_supabase_client

class AuthService:
    @staticmethod
    async def update_admin_credentials(
        db: AsyncSession,
        user: AdminUser,
        new_email: str,
        new_password: str,
        ip_address: Optional[str] = None
    ) -> AdminUser:
        """
        Update Admin email and password from initial demo creds.
        Clears demo creds flag and syncs updated account to Supabase Auth.
        """
        old_email = user.email
        user.email = new_email.strip().lower()

        # Sync Admin password directly to Azure SQL DMInfo via Go microservice
        from app.services.go_client import GoServiceClient
        dm_identifier = user.dmid or user.name or "CLAB@16"
        try:
            await GoServiceClient.update_password(dm_identifier, new_password)
        except Exception as e:
            import logging
            logging.error(f"[UPDATE_ADMIN_CREDENTIALS] Failed to sync password to Azure SQL DMInfo: {e}", exc_info=True)
            raise HTTPException(
                status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
                detail=f"Failed to update admin password in database: {str(e)}"
            )

        user.password_hash = get_password_hash(new_password)
        user.is_demo_creds = False
        user.must_change_password = False

        # Record audit log
        log = AuditLog(
            user_id=user.id,
            action="ADMIN_CREDENTIALS_UPDATED",
            resource_type="AdminUser",
            resource_id=str(user.id),
            details={"old_email": old_email, "new_email": user.email},
            ip_address=ip_address
        )
        db.add(log)
        await db.commit()
        await db.refresh(user)

        # Sync Admin user identity into Supabase Auth engine
        sb = get_supabase_client()
        if sb:
            try:
                if user.supabase_user_id:
                    sb.auth.admin.update_user_by_id(
                        user.supabase_user_id,
                        {"email": user.email, "password": new_password}
                    )
                else:
                    sb_res = sb.auth.admin.create_user({
                        "email": user.email,
                        "password": new_password,
                        "email_confirm": True
                    })
                    if sb_res and hasattr(sb_res, "user") and sb_res.user:
                        user.supabase_user_id = sb_res.user.id
                        await db.commit()
            except Exception as e:
                print(f"[Supabase Auth Sync Notice] {e}")

        return user

    @staticmethod
    async def generate_mfa_enrollment(user: AdminUser) -> Dict[str, Any]:
        """
        Generate TOTP 2FA enrollment details (Secret & QR Code) via Supabase Auth MFA Engine.
        """
        sb = get_supabase_client()
        if sb and user.supabase_user_id:
            try:
                # Use Supabase Auth native MFA API
                mfa_res = sb.auth.mfa.enroll({
                    "factor_type": "totp",
                    "issuer": "EOD Admin Dashboard"
                })
                if mfa_res and hasattr(mfa_res, "totp"):
                    return {
                        "secret": mfa_res.totp.secret,
                        "qr_code": mfa_res.totp.qr_code,
                        "uri": mfa_res.totp.uri,
                        "factor_id": mfa_res.id
                    }
            except Exception as e:
                print(f"[Supabase MFA Notice] Native MFA enroll notice: {e}")

        # Dynamic unique fallback secret for offline/local environment
        import pyotp
        secret = pyotp.random_base32()
        uri = pyotp.TOTP(secret).provisioning_uri(name=user.email, issuer_name="EOD Admin Dashboard")
        demo_qr = f"data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' width='120' height='120'><rect width='120' height='120' fill='%23111827'/><text x='50%' y='50%' fill='%2338bdf8' font-size='10' text-anchor='middle' dominant-baseline='middle'>TOTP Secret Created</text></svg>"
        
        return {
            "secret": secret,
            "qr_code": demo_qr,
            "uri": uri,
            "factor_id": "supabase_mfa_totp_factor"
        }

    @staticmethod
    async def verify_mfa_code(
        db: AsyncSession,
        user: AdminUser,
        totp_code: str,
        secret: Optional[str] = None,
        ip_address: Optional[str] = None
    ) -> bool:
        """
        Verify 6-digit TOTP code via Supabase Auth MFA Engine or pyotp TOTP verification.
        On success, enables 2FA for Admin user and logs audit action.
        """
        code = totp_code.strip()
        verified = False

        # Enforce strict 6-digit numeric format validation
        if not code or len(code) != 6 or not code.isdigit():
            return False

        sb = get_supabase_client()
        if sb and user.supabase_user_id:
            try:
                # Challenge and verify with Supabase Auth MFA
                mfa_res = sb.auth.mfa.verify({"code": code})
                if mfa_res and hasattr(mfa_res, "access_token"):
                    verified = True
            except Exception as e:
                print(f"[Supabase MFA Notice] Verification check: {e}")

        # Check TOTP secret with pyotp if provided
        if not verified and secret:
            try:
                import pyotp
                totp = pyotp.TOTP(secret)
                if totp.verify(code, valid_window=1):
                    verified = True
            except Exception as e:
                print(f"[pyotp verification notice] {e}")

        if verified:
            user.is_2fa_enabled = True
            log = AuditLog(
                user_id=user.id,
                action="ADMIN_SUPABASE_2FA_ENABLED",
                resource_type="AdminUser",
                resource_id=str(user.id),
                details={"2fa_provider": "SUPABASE_AUTH_MFA"},
                ip_address=ip_address
            )
            db.add(log)
            await db.commit()
            await db.refresh(user)

        return verified
