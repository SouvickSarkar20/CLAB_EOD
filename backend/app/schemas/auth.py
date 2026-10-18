import uuid
from typing import Optional, Dict, Any
from pydantic import BaseModel, EmailStr, Field, ConfigDict, field_validator
from app.models.user import UserRole, UserStatus
from app.utils.security import validate_password_strength

class LoginRequest(BaseModel):
    email: str = Field(..., min_length=1, max_length=150, description="Email or DMID")
    password: str = Field(..., min_length=1, max_length=100, description="Password (up to 100 characters)")

    @field_validator("password")
    @classmethod
    def sanitize_login_password(cls, v: str) -> str:
        for ch in v:
            if ord(ch) < 32 or ord(ch) == 127:
                raise ValueError("Password cannot contain control characters or newlines")
        return v

class TokenResponse(BaseModel):
    access_token: str
    token_type: str = "bearer"
    mfa_required: bool = False
    is_demo_creds: bool = False
    user: Optional["UserProfileSchema"] = None

class UserProfileSchema(BaseModel):
    id: uuid.UUID
    dmid: str
    name: str
    email: str
    role: UserRole
    status: UserStatus
    is_2fa_enabled: bool
    is_demo_creds: bool
    must_change_password: bool

    model_config = ConfigDict(from_attributes=True)

class UpdateProfileRequest(BaseModel):
    name: Optional[str] = Field(None, min_length=1, max_length=100)
    email: Optional[EmailStr] = Field(None, description="Updated valid email address")
    dmid: Optional[str] = Field(None, min_length=1, max_length=50)

class UpdateCredentialsRequest(BaseModel):
    new_email: EmailStr
    new_password: str = Field(..., min_length=8, max_length=100, description="New strong password (8-100 characters)")

    @field_validator("new_password")
    @classmethod
    def validate_new_pwd(cls, v: str) -> str:
        return validate_password_strength(v)

class ChangePasswordRequest(BaseModel):
    current_password: Optional[str] = Field(None, max_length=100)
    old_password: Optional[str] = Field(None, max_length=100)
    new_password: str = Field(..., min_length=8, max_length=100, description="New strong password (8-100 characters)")

    @field_validator("new_password")
    @classmethod
    def validate_new_pwd(cls, v: str) -> str:
        return validate_password_strength(v)

class MFAEnrollResponse(BaseModel):
    secret: str
    qr_code: str
    uri: str
    factor_id: str

class Verify2FARequest(BaseModel):
    totp_code: str = Field(..., min_length=6, max_length=6, description="6-digit TOTP code")
    secret: Optional[str] = Field(None, description="Optional TOTP secret if verifying enrollment")

class MFAVerifyResponse(BaseModel):
    success: bool
    message: str
    access_token: Optional[str] = None
    user: Optional[UserProfileSchema] = None
