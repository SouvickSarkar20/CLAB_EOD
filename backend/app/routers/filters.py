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
        raw_months = go_data.get("months", [])
        avail_months = []
        for m in raw_months:
            try:
                avail_months.append(int(m))
            except (ValueError, TypeError):
                continue
        if not avail_months:
            avail_months = [202607]

        selected_m = month if month in avail_months else (avail_months[0] if avail_months else 202607)

        return FilterOptionsResponse(
            available_months=avail_months,
            selected_month=selected_m,
            districts=dists,
            district_managers=dms,
            stations=stations,
            operators=[]
        )
    except Exception as e:
        import logging
        logging.error(f"[FILTERS] Error contacting Go Core Engine: {e}", exc_info=True)
        from fastapi import HTTPException
        raise HTTPException(
            status_code=503,
            detail="The core database engine is temporarily unavailable. Please try again shortly."
        )
