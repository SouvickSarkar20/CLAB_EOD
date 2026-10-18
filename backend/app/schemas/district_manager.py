import uuid
from datetime import datetime
from typing import Optional, List
from pydantic import BaseModel, EmailStr, Field, ConfigDict, field_validator
from app.models.user import UserStatus
from app.utils.security import validate_password_strength

class DMStationAssignmentSchema(BaseModel):
    station_id: str
    station_name: Optional[str] = None
    district_id: int
    district_name: Optional[str] = None
    effective_month: int

    model_config = ConfigDict(from_attributes=True)

class DMListItem(BaseModel):
    id: uuid.UUID
    dmid: str
    name: str
    email: str
    status: UserStatus
    assigned_stations_count: int = 0
    assigned_districts: List[str] = []
    last_login_at: Optional[datetime] = None
    created_at: datetime

    model_config = ConfigDict(from_attributes=True)

class DMDetail(BaseModel):
    id: uuid.UUID
    dmid: str
    name: str
    email: str
    status: UserStatus
    assigned_stations: List[DMStationAssignmentSchema] = []
    assigned_districts: List[str] = []
    last_login_at: Optional[datetime] = None
    created_at: datetime
    updated_at: datetime

    model_config = ConfigDict(from_attributes=True)

class CreateDMRequest(BaseModel):
    dmid: Optional[str] = Field(None, description="Optional DMID (auto-generated if omitted)")
    name: str = Field(..., min_length=2, max_length=100)
    email: EmailStr = Field(..., description="Valid email address")
    district_ids: List[int] = Field(default=[], description="Initial district assignments")

class UpdateDMRequest(BaseModel):
    name: Optional[str] = Field(None, min_length=2, max_length=100)
    email: Optional[EmailStr] = None
    status: Optional[UserStatus] = None
    district_ids: Optional[List[int]] = None
    station_ids: Optional[List[str]] = None
    

class ResetPasswordRequest(BaseModel):
    new_password: Optional[str] = Field(
        None,
        min_length=8,
        max_length=100,
        description="Custom password to set for the DM (8-100 characters, auto-generated if omitted)"
    )

    @field_validator("new_password")
    @classmethod
    def validate_custom_pwd(cls, v: Optional[str]) -> Optional[str]:
        if v is None or v == "":
            return None
        return validate_password_strength(v)

class ResetPasswordResponse(BaseModel):
    dm_id: uuid.UUID
    dmid: str
    temporary_password: str
    message: str
