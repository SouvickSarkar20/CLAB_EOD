import uuid
from datetime import date
from typing import Optional
from fastapi import APIRouter, Depends, Query, HTTPException, status
from sqlalchemy.ext.asyncio import AsyncSession

from app.dependencies.db import get_db
from app.dependencies.auth import require_mfa
from app.models import AdminUser
from app.schemas.daily import DailyRow, DailySummaryResponse, DailyBreakdownResponse
from app.utils.pagination import PaginatedResponse, create_paginated_response
from app.services.analytics_service import AnalyticsService
from app.services.go_client import GoServiceClient

router = APIRouter()

@router.get("/", response_model=PaginatedResponse[DailyRow])
async def get_daily_records(
    start_date: Optional[date] = Query(None, description="Start date filter YYYY-MM-DD"),
    end_date: Optional[date] = Query(None, description="End date filter YYYY-MM-DD"),
    month: Optional[int] = Query(None, description="Enrollment month YYYYMM"),
    dm_id: Optional[str] = Query(None, description="Filter by DM User UUID or DMID string"),
    district_id: Optional[int] = Query(None, description="Filter by District ID"),
    station_id: Optional[str] = Query(None, description="Filter by Station ID"),
    operator_code: Optional[str] = Query(None, description="Filter by Operator Code"),
    page: int = Query(1, ge=1, description="Page number"),
    page_size: int = Query(25, ge=1, le=200, description="Items per page"),
    db: AsyncSession = Depends(get_db),
    _: AdminUser = Depends(require_mfa)
):
    """Fetch paginated Daily records with filters from Go microservice (with fallback)."""
    month_str = str(month) if month else ""
    try:
        data = await GoServiceClient.get_daily_logs(
            month=month_str,
            dm=dm_id,
            station_id=station_id,
            page=page,
            page_size=page_size
        )
        items = []
        for idx, it in enumerate(data.get("items", [])):
            items.append(
                DailyRow(
                    record_id=idx + 1 + (page - 1) * page_size,
                    enroll_date=it["enroll_date"],
                    station_id=it["station_id"],
                    station_name=f"Station {it['station_id']}",
                    operator_code=it.get("operator_name") or "N/A",
                    operator_name=it.get("operator_name") or "N/A",
                    district_name=it.get("district") or "N/A",
                    dm_name=it.get("district_manager") or "N/A",
                    total_enrollment=it["total_enrollment"],
                    total_amount=it["total_amount"],
                    bmu_100=it["bmu_100"],
                    bmu_125=it["bmu_125"],
                    dmu_50=it["dmu_50"],
                    dmu_75=it["dmu_75"],
                    mbu_0=it["mbu_0"],
                    mbu_100=it["mbu_100"],
                    mbu_125=it["mbu_125"],
                    new_0=it["new_0"]
                )
            )
        return create_paginated_response(
            items=items,
            total=data.get("total_count", 0),
            page=page,
            page_size=page_size
        )
    except Exception:
        resolved_dm_uuid = await AnalyticsService.resolve_dm_uuid(db, dm_id)
        items, total = await AnalyticsService.get_daily_records(
            db=db,
            start_date=start_date,
            end_date=end_date,
            month=month,
            dm_id=resolved_dm_uuid,
            district_id=district_id,
            station_id=station_id,
            operator_code=operator_code,
            page=page,
            page_size=page_size
        )
        return create_paginated_response(items=items, total=total, page=page, page_size=page_size)

@router.get("/summary", response_model=DailySummaryResponse)
async def get_daily_summary_overview(
    start_date: Optional[date] = Query(None, description="Start date filter YYYY-MM-DD"),
    end_date: Optional[date] = Query(None, description="End date filter YYYY-MM-DD"),
    month: Optional[int] = Query(None, description="Enrollment month YYYYMM"),
    dm_id: Optional[str] = Query(None, description="Filter by DM User UUID or DMID string"),
    district_id: Optional[int] = Query(None, description="Filter by District ID"),
    db: AsyncSession = Depends(get_db),
    _: AdminUser = Depends(require_mfa)
):
    """Fetch Daily Analytics Overview from Go microservice (with fallback)."""
    month_str = str(month) if month else ""
    try:
        data = await GoServiceClient.get_dm_overview(
            month=month_str,
            dm=dm_id
        )
        return DailySummaryResponse.model_validate(data)
    except Exception:
        resolved_dm_uuid = await AnalyticsService.resolve_dm_uuid(db, dm_id)
        return await AnalyticsService.get_daily_summary_overview(
            db=db,
            start_date=start_date,
            end_date=end_date,
            month=month,
            dm_id=resolved_dm_uuid,
            district_id=district_id
        )

@router.get("/breakdown", response_model=DailyBreakdownResponse)
async def get_daily_breakdown(
    month: int = Query(..., description="Enrollment month YYYYMM (e.g. 202601)"),
    station_id: str = Query(..., description="Station ID"),
    operator_code: str = Query(..., description="Operator Code"),
    db: AsyncSession = Depends(get_db),
    _: AdminUser = Depends(require_mfa)
):
    """
    View Daily Breakdown drill-through:
    Returns all daily record rows for a specific station + operator combination in a month.
    """
    return await AnalyticsService.get_daily_breakdown(
        db=db,
        month=month,
        station_id=station_id,
        operator_code=operator_code
    )
