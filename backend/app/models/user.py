import uuid
import enum
from datetime import datetime, timezone
from typing import Optional
from sqlalchemy import String, DateTime, Enum as SQLEnum, Boolean, Uuid
from sqlalchemy.orm import Mapped, mapped_column
from app.database import Base

class UserRole(str, enum.Enum):
    ADMIN = "admin"
    DISTRICT_MANAGER = "district_manager"

class UserStatus(str, enum.Enum):
    ACTIVE = "active"
    INACTIVE = "inactive"

def utc_now():
    return datetime.utcnow()

class AdminUser(Base):
    __tablename__ = "admin_users"

    id: Mapped[uuid.UUID] = mapped_column(Uuid(as_uuid=True), primary_key=True, default=uuid.uuid4)
    dmid: Mapped[str] = mapped_column(String(50), unique=True, index=True)
    name: Mapped[str] = mapped_column(String(100))
    email: Mapped[str] = mapped_column(String(150), unique=True, index=True)
    role: Mapped[UserRole] = mapped_column(SQLEnum(UserRole, native_enum=False), default=UserRole.DISTRICT_MANAGER)
    status: Mapped[UserStatus] = mapped_column(SQLEnum(UserStatus, native_enum=False), default=UserStatus.ACTIVE)
    password_hash: Mapped[str] = mapped_column(String(255))
    supabase_user_id: Mapped[Optional[str]] = mapped_column(String(255), nullable=True)  # Linked Supabase Auth ID for Admin 2FA
    is_2fa_enabled: Mapped[bool] = mapped_column(Boolean, default=False)  # Admin 2FA toggle status via Supabase
    is_demo_creds: Mapped[bool] = mapped_column(Boolean, default=True)    # Flag indicating if Admin is using initial demo creds
    must_change_password: Mapped[bool] = mapped_column(Boolean, default=False)
    created_at: Mapped[datetime] = mapped_column(DateTime, default=utc_now)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=utc_now, onupdate=utc_now)
    last_login_at: Mapped[Optional[datetime]] = mapped_column(DateTime, nullable=True)
