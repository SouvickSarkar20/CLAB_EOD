import httpx
from typing import Optional, Dict, Any, List
from app.config import settings

class GoServiceClient:
    """
    High-performance Async Client for Go Core Data Engine.
    Uses a persistent Keep-Alive connection pool to minimize network latency (<2ms).
    """
    _client: Optional[httpx.AsyncClient] = None

    @classmethod
    def base_url(cls) -> str:
        return getattr(settings, "GO_SERVICE_URL", "http://localhost:8081").rstrip("/")

    @classmethod
    def get_client(cls) -> httpx.AsyncClient:
        """Returns or initializes the singleton persistent connection pool."""
        if cls._client is None or cls._client.is_closed:
            limits = httpx.Limits(
                max_keepalive_connections=20,
                max_connections=50,
                keepalive_expiry=60.0
            )
            timeout = httpx.Timeout(
                connect=5.0,
                read=120.0,
                write=30.0,
                pool=5.0
            )
            headers = {}
            secret = getattr(settings, "INTERNAL_API_SECRET", "")
            if secret:
                headers["X-Internal-Secret"] = secret

            cls._client = httpx.AsyncClient(
                base_url=cls.base_url(),
                headers=headers,
                limits=limits,
                timeout=timeout
            )
        return cls._client

    @classmethod
    async def close(cls):
        """Cleanly close connection pool on FastAPI shutdown."""
        if cls._client is not None and not cls._client.is_closed:
            await cls._client.aclose()
            cls._client = None

    @classmethod
    async def health_check(cls) -> Dict[str, Any]:
        client = cls.get_client()
        resp = await client.get("/health")
        resp.raise_for_status()
        return resp.json()

    @classmethod
    async def verify_user(cls, username: str, password: str) -> Optional[Dict[str, Any]]:
        client = cls.get_client()
        try:
            resp = await client.post(
                "/api/auth/verify",
                json={"username": username, "password": password}
            )
            if resp.status_code == 200:
                return resp.json()
            return None
        except Exception:
            return None

    @classmethod
    async def list_users(cls) -> List[Dict[str, Any]]:
        client = cls.get_client()
        try:
            resp = await client.get("/api/auth/users")
            if resp.status_code == 200:
                return resp.json()
            return []
        except Exception:
            return []

    @classmethod
    async def get_filter_options(cls, dm: Optional[str] = None) -> Dict[str, Any]:
        params = {}
        if dm:
            params["dm"] = dm
        client = cls.get_client()
        resp = await client.get("/api/filters/options", params=params)
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
        client = cls.get_client()
        resp = await client.get("/api/dm/overview", params=params)
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
        client = cls.get_client()
        resp = await client.get("/api/daily/logs", params=params)
        resp.raise_for_status()
        return resp.json()

    @classmethod
    async def get_monthly_records(
        cls,
        month: Optional[str] = None,
        dm: Optional[str] = None,
        district: Optional[str] = None,
        station_id: Optional[str] = None,
        operator_code: Optional[str] = None,
        page: int = 1,
        page_size: int = 25
    ) -> Dict[str, Any]:
        params = {
            "page": page,
            "page_size": page_size
        }
        if month:
            params["month"] = month
        if dm:
            params["dm"] = dm
        if district:
            params["district"] = district
        if station_id:
            params["station_id"] = station_id
        if operator_code:
            params["operator_code"] = operator_code
        client = cls.get_client()
        resp = await client.get("/api/monthly/records", params=params)
        resp.raise_for_status()
        return resp.json()

    @classmethod
    async def bulk_insert_records(cls, records: List[Dict[str, Any]]) -> Dict[str, Any]:
        client = cls.get_client()
        resp = await client.post(
            "/api/records/bulk-insert",
            json={"records": records}
        )
        resp.raise_for_status()
        return resp.json()

    @classmethod
    async def assign_stations(cls, dm_name: str, station_ids: List[str]) -> Dict[str, Any]:
        """Assigns stations to a DM directly in Azure SQL CLAB-EOD (EODMth and EODDt)."""
        client = cls.get_client()
        resp = await client.post(
            "/api/admin/assign-stations",
            json={"dm_name": dm_name, "station_ids": station_ids}
        )
        resp.raise_for_status()
        return resp.json()

    @classmethod
    async def get_dm_stations(cls, dm_name: str) -> Dict[str, Any]:
        """Fetches all stations assigned to a DM from Azure SQL CLAB-EOD (EODMth)."""
        client = cls.get_client()
        resp = await client.get("/api/dm/stations", params={"dm": dm_name})
        resp.raise_for_status()
        return resp.json()

    @classmethod
    async def update_password(cls, user_id: str, new_password: str) -> Dict[str, Any]:
        """Update user password directly in Azure SQL DMInfo via Go microservice."""
        client = cls.get_client()
        resp = await client.post(
            "/api/auth/update-password",
            json={"user_id": user_id, "new_password": new_password}
        )
        resp.raise_for_status()
        return resp.json()

    @classmethod
    async def invalidate_filter_cache(cls) -> Dict[str, Any]:
        """Invalidate the in-memory filter options cache in the Go microservice."""
        client = cls.get_client()
        resp = await client.post("/api/filters/invalidate-cache")
        resp.raise_for_status()
        return resp.json()




