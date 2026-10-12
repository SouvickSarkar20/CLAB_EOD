from pydantic_settings import BaseSettings, SettingsConfigDict
from pydantic import model_validator

class Settings(BaseSettings):
    DATABASE_URL: str = "postgresql+asyncpg://postgres:postgres@localhost:5432/eod_db"
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

    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8", extra="ignore")

    @model_validator(mode="after")
    def validate_security(self):
        if self.ENVIRONMENT.lower() in ["production", "prod"] and "default_secret_key" in self.SECRET_KEY:
            raise ValueError("SECRET_KEY must be configured securely in production environment!")
        return self

settings = Settings()
