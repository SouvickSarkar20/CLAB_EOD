from pydantic_settings import BaseSettings, SettingsConfigDict
from pydantic import model_validator

class Settings(BaseSettings):
    DATABASE_URL: str = "sqlite+aiosqlite:///./auth.db"
    SECRET_KEY: str = "default_secret_key_change_in_production_1234567890"
    ALGORITHM: str = "HS256"
    ACCESS_TOKEN_EXPIRE_MINUTES: int = 30
    REFRESH_TOKEN_EXPIRE_DAYS: int = 7
    ENVIRONMENT: str = "development"
    DEBUG: bool = True
    
    # Supabase Credentials for Admin 2FA
    SUPABASE_URL: str = ""
    SUPABASE_SERVICE_ROLE_KEY: str = ""
    
    FRONTEND_URL: str = "http://localhost:5173"
    GO_SERVICE_URL: str = "http://localhost:8081"
    INTERNAL_API_SECRET: str = "clab-internal-secret-change-in-production-2026"

    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8", extra="ignore")

    @model_validator(mode="after")
    def validate_security(self):
        # Fall back to local SQLite for auth session store to eliminate external network dependencies
        if "supabase" in self.DATABASE_URL.lower():
            self.DATABASE_URL = "sqlite+aiosqlite:///./auth.db"
        if self.ENVIRONMENT.lower() in ["production", "prod"] and "default_secret_key" in self.SECRET_KEY:
            raise ValueError("SECRET_KEY must be configured securely in production environment!")
        if self.ENVIRONMENT.lower() in ["production", "prod"] and "change-in-production" in self.INTERNAL_API_SECRET:
            raise ValueError("INTERNAL_API_SECRET must be configured securely in production environment!")
        return self

settings = Settings()
