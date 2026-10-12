import httpx
from typing import Optional, Dict, Any, List
from app.config import settings

class GoServiceClient:
    """Async client communicating with the Go database microservice."""
    
    @classmethod
    def base_url(cls) -> str:
        return getattr(settings, "GO_SERVICE_URL", "http://localhost:8081").rstrip("/")

    @classmethod
    async def health_check(cls) -> Dict[str, Any]:
        async with httpx.AsyncClient(timeout=10.0) as client:
            resp = await client.get(f"{cls.base_url()}/health")
            resp.raise_for_status()
            return resp.json()

    @classmethod
    async def verify_user(cls, username: str, password: str) -> Optional[Dict[str, Any]]:
        async with httpx.AsyncClient(timeout=10.0) as client:
            try:
                resp = await client.post(
                    f"{cls.base_url()}/api/auth/verify",
                    json={"username": username, "password": password}
                )
                if resp.status_code == 200:
                    return resp.json()
                return None
            except Exception:
                return None

    @classmethod
    async def get_filter_options(cls, dm: Optional[str] = None) -> Dict[str, Any]:
        params = {}
        if dm:
            params["dm"] = dm
        async with httpx.AsyncClient(timeout=90.0) as client:
            resp = await client.get(f"{cls.base_url()}/api/filters/options", params=params)
            resp.raise_for_status()
            return resp.json()

    @classmethod
    async def get_dm_overview(
        cls,
        month: str = "202607",
        dm: Optional[str] = None,
        district: Optional[str] = None,
        station_id: Optional[str] = None
    ) -> Dict[str, Any]:
        params = {"month": month}
        if dm:
            params["dm"] = dm
        if district:
            params["district"] = district
        if station_id:
            params["station_id"] = station_id
        async with httpx.AsyncClient(timeout=90.0) as client:
            resp = await client.get(f"{cls.base_url()}/api/dm/overview", params=params)
            resp.raise_for_status()
            return resp.json()

    @classmethod
    async def get_daily_logs(
        cls,
        month: str = "202607",
        dm: Optional[str] = None,
        district: Optional[str] = None,
        station_id: Optional[str] = None,
        date: Optional[str] = None,
        page: int = 1,
        page_size: int = 25
    ) -> Dict[str, Any]:
        params = {
            "month": month,
            "page": page,
            "page_size": page_size
        }
        if dm:
            params["dm"] = dm
        if district:
            params["district"] = district
        if station_id:
            params["station_id"] = station_id
        if date:
            params["date"] = date
        async with httpx.AsyncClient(timeout=25.0) as client:
            resp = await client.get(f"{cls.base_url()}/api/daily/logs", params=params)
            resp.raise_for_status()
            return resp.json()

    @classmethod
    async def bulk_insert_records(cls, records: List[Dict[str, Any]]) -> Dict[str, Any]:
        async with httpx.AsyncClient(timeout=60.0) as client:
            resp = await client.post(
                f"{cls.base_url()}/api/records/bulk-insert",
                json={"records": records}
            )
            resp.raise_for_status()
            return resp.json()

