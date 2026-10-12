from sqlalchemy.ext.asyncio import create_async_engine, async_sessionmaker, AsyncSession
from sqlalchemy.orm import DeclarativeBase
from app.config import settings

if "sqlite" in settings.DATABASE_URL.lower():
    engine = create_async_engine(
        settings.DATABASE_URL,
        echo=False,
    )
else:
    connect_args = {}
    if "supabase" in settings.DATABASE_URL.lower() or settings.ENVIRONMENT.lower() in ["production", "prod"]:
        connect_args["ssl"] = "require"
        connect_args["statement_cache_size"] = 0

    engine = create_async_engine(
        settings.DATABASE_URL,
        echo=False,
        pool_size=3,
        max_overflow=5,
        pool_timeout=30,
        pool_recycle=1800,
        connect_args=connect_args
    )
AsyncSessionLocal = async_sessionmaker(engine, expire_on_commit=False, class_=AsyncSession)

class Base(DeclarativeBase):
    pass

async def get_db():
    async with AsyncSessionLocal() as session:
        try:
            yield session
        finally:
            await session.close()

