from contextlib import asynccontextmanager
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from app.config import settings
from app.services.go_client import GoServiceClient
from app.routers import (
    auth,
    district_managers,
    monthly,
    daily,
    filters,
    exports,
    audit_log,
    dm,
)

@asynccontextmanager
async def lifespan(app: FastAPI):
    # Initialize GoServiceClient persistent connection pool on startup
    GoServiceClient.get_client()

    # If running with SQLite, ensure tables and initial admin user exist
    if "sqlite" in settings.DATABASE_URL.lower():
        from app.database import engine, Base, AsyncSessionLocal
        from app.models import AdminUser, UserRole, UserStatus
        from app.utils.security import get_password_hash
        from sqlalchemy import select
        async with engine.begin() as conn:
            await conn.run_sync(Base.metadata.create_all)

        async with AsyncSessionLocal() as session:
            stmt = select(AdminUser).where(AdminUser.email == "admin@eod.internal")
            res = await session.execute(stmt)
            if not res.scalar_one_or_none():
                admin = AdminUser(
                    dmid="ADMIN",
                    name="System Administrator",
                    email="admin@eod.internal",
                    password_hash=get_password_hash("Admin@123456"),
                    role=UserRole.ADMIN,
                    status=UserStatus.ACTIVE,
                    is_demo_creds=True
                )
                session.add(admin)
                await session.commit()

    yield
    # Gracefully close connection pool on shutdown
    await GoServiceClient.close()

app = FastAPI(
    title="EOD Management & Analytics Dashboard API",
    description="Backend API providing operation supervision, analytics, and Supabase 2FA authentication for EOD operation.",
    version="1.0.0",
    lifespan=lifespan,
)

# CORS configuration
origins = [
    settings.FRONTEND_URL,
    "https://clab-eod.vercel.app",
    "http://localhost:5173",
    "http://127.0.0.1:5173",
    "http://localhost:5174",
    "http://127.0.0.1:5174",
    "http://localhost:5175",
    "http://127.0.0.1:5175",
]

app.add_middleware(
    CORSMiddleware,
    allow_origins=origins,
    allow_origin_regex=r"https://.*\.vercel\.app|http://(localhost|127\.0\.0\.1)(:\d+)?",
    allow_credentials=True,
    allow_methods=["GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"],
    allow_headers=["*"],
)

# Register API Routers
app.include_router(auth.router, prefix="/api/auth", tags=["Authentication"])
app.include_router(district_managers.router, prefix="/api/district-managers", tags=["District Managers"])
app.include_router(monthly.router, prefix="/api/monthly", tags=["Monthly Analytics"])
app.include_router(daily.router, prefix="/api/daily", tags=["Daily Analytics"])
app.include_router(filters.router, prefix="/api/filters", tags=["Filter Options"])
app.include_router(exports.router, prefix="/api/exports", tags=["Data Exports"])
app.include_router(audit_log.router, prefix="/api/audit-log", tags=["Audit Log"])
app.include_router(dm.router, prefix="/api/dm", tags=["District Manager Dashboard"])

@app.api_route("/api/health", methods=["GET", "HEAD"], tags=["Health"])
async def health_check():
    return {"status": "healthy", "service": "EOD Dashboard API"}

@app.api_route("/", methods=["GET", "HEAD"], tags=["Root"])
async def root():
    return {
        "message": "Welcome to EOD Dashboard API",
        "docs_url": "/docs",
        "health_check": "/api/health"
    }
