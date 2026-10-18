from datetime import datetime, timedelta, timezone
from typing import Optional, Dict, Any
import bcrypt
from jose import jwt, JWTError
from app.config import settings

def verify_password(plain_password: str, hashed_password: str) -> bool:
    """Verify a plain text password against a stored bcrypt hash."""
    if not plain_password or not hashed_password:
        return False
    try:
        return bcrypt.checkpw(plain_password.encode('utf-8'), hashed_password.encode('utf-8'))
    except Exception:
        return False

def get_password_hash(password: str) -> str:
    """Generate a bcrypt hash from a plain text password."""
    pwd_bytes = password.encode('utf-8')
    salt = bcrypt.gensalt()
    return bcrypt.hashpw(pwd_bytes, salt).decode('utf-8')

def create_access_token(data: Dict[str, Any], expires_delta: Optional[timedelta] = None) -> str:
    """Create a signed JWT access token."""
    to_encode = data.copy()
    if expires_delta:
        expire = datetime.now(timezone.utc) + expires_delta
    else:
        expire = datetime.now(timezone.utc) + timedelta(minutes=settings.ACCESS_TOKEN_EXPIRE_MINUTES)
    to_encode.update({"exp": expire, "token_type": "access"})
    encoded_jwt = jwt.encode(to_encode, settings.SECRET_KEY, algorithm=settings.ALGORITHM)
    return encoded_jwt

def create_refresh_token(data: Dict[str, Any], expires_delta: Optional[timedelta] = None) -> str:
    """Create a signed JWT refresh token."""
    to_encode = data.copy()
    if expires_delta:
        expire = datetime.now(timezone.utc) + expires_delta
    else:
        expire = datetime.now(timezone.utc) + timedelta(days=settings.REFRESH_TOKEN_EXPIRE_DAYS)
    to_encode.update({"exp": expire, "token_type": "refresh"})
    encoded_jwt = jwt.encode(to_encode, settings.SECRET_KEY, algorithm=settings.ALGORITHM)
    return encoded_jwt

def decode_token(token: str) -> Optional[Dict[str, Any]]:
    """Decode and validate a JWT token."""
    try:
        payload = jwt.decode(token, settings.SECRET_KEY, algorithms=[settings.ALGORITHM])
        return payload
    except JWTError:
        return None

SENSITIVE_AUDIT_KEYS = {
    "password", "old_password", "new_password", "current_password",
    "secret", "totp_secret", "access_token", "refresh_token", "token", "password_hash"
}

def sanitize_audit_details(details: Optional[Dict[str, Any]]) -> Dict[str, Any]:
    """Sanitize audit details dict to prevent leaking passwords, tokens, or TOTP secrets."""
    if not details or not isinstance(details, dict):
        return details if isinstance(details, dict) else {}
    sanitized = {}
    for k, v in details.items():
        if k.lower() in SENSITIVE_AUDIT_KEYS:
            sanitized[k] = "[REDACTED]"
        elif isinstance(v, dict):
            sanitized[k] = sanitize_audit_details(v)
        else:
            sanitized[k] = v
    return sanitized

def validate_month_param(month: Optional[int]) -> Optional[int]:
    """Validate integer month parameter in YYYYMM format (e.g. 202601)."""
    if month is None:
        return None
    year = month // 100
    m = month % 100
    if year < 2000 or year > 2100 or m < 1 or m > 12:
        from fastapi import HTTPException, status
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=f"Invalid month '{month}'. Month must be in YYYYMM format with valid month 01-12 (e.g., 202601)."
        )
    return month

def validate_password_strength(password: str) -> str:
    """
    Validate password length, characters, and formatting:
    - Must be between 8 and 100 characters.
    - No leading or trailing whitespace.
    - No control characters (\x00-\x1f, \x7f) or newlines.
    """
    if not password:
        raise ValueError("Password cannot be empty")
    if len(password) < 8:
        raise ValueError("Password must be at least 8 characters long")
    if len(password) > 100:
        raise ValueError("Password cannot exceed 100 characters")
    if password != password.strip():
        raise ValueError("Password cannot begin or end with whitespace")
    for ch in password:
        if ord(ch) < 32 or ord(ch) == 127:
            raise ValueError("Password cannot contain control characters or newlines")
    return password
