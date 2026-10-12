import uuid
from typing import Optional
from fastapi import APIRouter, Depends, Query
from sqlalchemy.ext.asyncio import AsyncSession

from app.dependencies.db import get_db
from app.dependencies.auth import get_current_user
from app.models import AdminUser
from app.schemas.filters import FilterOptionsResponse, DistrictOption, DMOption, StationOption, OperatorOption
from app.services.analytics_service import AnalyticsService
from app.services.go_client import GoServiceClient

router = APIRouter()

@router.get("/options", response_model=FilterOptionsResponse)
async def get_filter_options(
    month: Optional[int] = Query(None, description="Selected enrollment month YYYYMM"),
    district_id: Optional[int] = Query(None, description="Selected District ID"),
    dm_id: Optional[str] = Query(None, description="Selected DM User UUID or DMID string"),
    db: AsyncSession = Depends(get_db),
    current_user: AdminUser = Depends(get_current_user)
):
    """
    Fetch period-aware cascading filter choices from Go microservice (with fallback).
    """
    try:
        dm_name = current_user.name if current_user.role.value == "district_manager" else None
        go_data = await GoServiceClient.get_filter_options(dm=dm_name)
        dists = [
            DistrictOption(district_id=idx + 1, district_name=d)
            for idx, d in enumerate(go_data.get("districts", []))
        ]
        dms = [
            DMOption(id=uuid.uuid5(uuid.NAMESPACE_DNS, d["id"]), dmid=d["id"], name=d["name"])
            for d in go_data.get("district_managers", [])
        ]
        stations = [
            StationOption(station_id=s, station_name=f"Station {s}")
            for s in go_data.get("stations", [])
        ]
        return FilterOptionsResponse(
            available_months=[202607],
            selected_month=month or 202607,
            districts=dists,
            district_managers=dms,
            stations=stations,
            operators=[]
        )
    except Exception:
        resolved_dm_uuid = await AnalyticsService.resolve_dm_uuid(db, dm_id)
        return await AnalyticsService.get_cascading_filter_options(
            db=db,
            month=month,
            district_id=district_id,
            dm_id=resolved_dm_uuid
        )
