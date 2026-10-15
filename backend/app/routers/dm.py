import uuid
import csv
import io
import collections
from datetime import date
from typing import Optional, Dict, Any, List
from fastapi import APIRouter, Depends, Query, UploadFile, File, HTTPException, status, Body
from sqlalchemy.ext.asyncio import AsyncSession

from app.dependencies.db import get_db
from app.dependencies.auth import require_dm
from app.models import AdminUser
from app.schemas.daily import DailyRow, DailySummaryResponse, DailyBreakdownResponse
from app.schemas.monthly import MonthlyRow
from app.utils.pagination import PaginatedResponse, create_paginated_response
from app.services.analytics_service import AnalyticsService
from app.services.go_client import GoServiceClient

router = APIRouter()

@router.get("/overview", response_model=DailySummaryResponse)
async def get_dm_overview(
    start_date: Optional[date] = Query(None, description="Start date filter YYYY-MM-DD"),
    end_date: Optional[date] = Query(None, description="End date filter YYYY-MM-DD"),
    month: Optional[int] = Query(None, description="Enrollment month YYYYMM"),
    db: AsyncSession = Depends(get_db),
    current_dm: AdminUser = Depends(require_dm)
):
    """
    Fetch DM Dashboard Overview from Go database microservice (with fallback).
    Automatically scoped to the authenticated DM's district.
    """
    month_str = str(month) if month else ""
    dm_scope = current_dm.name if current_dm.role.value == "district_manager" else None
    try:
        data = await GoServiceClient.get_dm_overview(
            month=month_str,
            dm=dm_scope
        )
        return DailySummaryResponse.model_validate(data)
    except Exception as e:
        import logging
        logging.error(f"[DM_OVERVIEW] Error contacting Go Core Engine: {e}", exc_info=True)
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="The analytics engine is currently processing. Please refresh in a moment."
        )

@router.get("/daily", response_model=PaginatedResponse[DailyRow])
async def get_dm_daily_records(
    start_date: Optional[date] = Query(None, description="Start date filter YYYY-MM-DD"),
    end_date: Optional[date] = Query(None, description="End date filter YYYY-MM-DD"),
    month: Optional[int] = Query(None, description="Enrollment month YYYYMM"),
    station_id: Optional[str] = Query(None, description="Filter by Station ID"),
    operator_code: Optional[str] = Query(None, description="Filter by Operator Code"),
    page: int = Query(1, ge=1),
    page_size: int = Query(25, ge=1, le=200),
    db: AsyncSession = Depends(get_db),
    current_dm: AdminUser = Depends(require_dm)
):
    """
    Fetch Daily records from Go database microservice (with fallback).
    """
    month_str = str(month) if month else ""
    dm_scope = current_dm.name if current_dm.role.value == "district_manager" else None
    try:
        data = await GoServiceClient.get_daily_logs(
            month=month_str,
            dm=dm_scope,
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
    except Exception as e:
        import logging
        logging.error(f"[DM_DAILY] Error contacting Go Core Engine: {e}", exc_info=True)
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="The records engine is currently processing. Please refresh in a moment."
        )

@router.get("/daily/breakdown", response_model=DailyBreakdownResponse)
async def get_dm_daily_breakdown(
    month: int = Query(..., description="Enrollment month YYYYMM"),
    station_id: str = Query(..., description="Station ID"),
    operator_code: str = Query(..., description="Operator Code"),
    db: AsyncSession = Depends(get_db),
    current_dm: AdminUser = Depends(require_dm)
):
    """
    Fetch Daily records breakdown for a specific DM station & operator
    """
    return await AnalyticsService.get_daily_breakdown(
        db=db,
        month=month,
        dm_id=current_dm.id,
        district_id=None,
        station_id=station_id,
        operator_code=operator_code
    )

@router.get("/monthly", response_model=PaginatedResponse[MonthlyRow])
async def get_dm_monthly_records(
    month: Optional[int] = Query(None, description="Enrollment month YYYYMM"),
    station_id: Optional[str] = Query(None, description="Filter by Station ID"),
    operator_code: Optional[str] = Query(None, description="Filter by Operator Code"),
    page: int = Query(1, ge=1),
    page_size: int = Query(25, ge=1, le=200),
    db: AsyncSession = Depends(get_db),
    current_dm: AdminUser = Depends(require_dm)
):
    """
    Fetch Monthly records specifically for this DM's district.
    """
    items, total = await AnalyticsService.get_monthly_records(
        db=db,
        month=month,
        dm_id=current_dm.id,
        district_id=None,
        station_id=station_id,
        operator_code=operator_code,
        page=page,
        page_size=page_size
    )
    if total > 0:
        return create_paginated_response(items=items, total=total, page=page, page_size=page_size)

    # Delegate to Go microservice backed by Azure SQL
    try:
        from decimal import Decimal
        month_str = str(month) if month else ""
        dm_scope = current_dm.name if current_dm.role.value == "district_manager" else None
        go_data = await GoServiceClient.get_monthly_records(
            month=month_str,
            dm=dm_scope,
            station_id=station_id,
            operator_code=operator_code,
            page=page,
            page_size=page_size
        )
        logs = go_data.get("items", [])
        tot_count = go_data.get("total_count", len(logs))
        res_items = []
        for idx, it in enumerate(logs):
            mth_val = 0
            if it.get("enroll_month"):
                try:
                    mth_val = int(it.get("enroll_month"))
                except ValueError:
                    mth_val = 0
            res_items.append(
                MonthlyRow(
                    summary_id=idx + 1 + (page - 1) * page_size,
                    enroll_month=mth_val,
                    station_id=it.get("station_id", ""),
                    station_name=it.get("station_name", f"Station {it.get('station_id', '')}"),
                    operator_code=it.get("operator_code", it.get("station_id", "")),
                    operator_name=it.get("operator_name", ""),
                    district_id=1,
                    district_name=it.get("district", ""),
                    dm_id=it.get("dm_id", it.get("district_manager", "")),
                    dm_name=it.get("district_manager", ""),
                    total_enrollment=it.get("total_enrollment", 0),
                    total_amount=Decimal(str(it.get("total_amount", 0.0))),
                    bmu_100=it.get("bmu_100", 0),
                    dmu_50=it.get("dmu_50", 0),
                    mbu_0=it.get("mbu_0", 0),
                    mbu_100=it.get("mbu_100", 0),
                    new_0=it.get("new_0", 0),
                    bmu_125=it.get("bmu_125", 0),
                    dmu_75=it.get("dmu_75", 0),
                    mbu_125=it.get("mbu_125", 0)
                )
            )
        return create_paginated_response(items=res_items, total=tot_count, page=page, page_size=page_size)
    except Exception:
        pass

    return create_paginated_response(items=items, total=total, page=page, page_size=page_size)

REQUIRED_EOD_COLUMNS = [
    "SLNO", "ENROLMENT_NO_DATE", "APPOINTMENT_ID", "TYPE", "MANDATORY_BIO_METRIC_UPDATE_ONLY",
    "IS_NRI", "TIN_NO", "OPERATOR_ID", "INTRODUCER", "PROOF", "RESIDENT_NAME", "STATUS",
    "GST_AMOUNT", "AMOUNT_CHARGED_FOR_NEW_ENROLMENT", "AMOUNT_CHARGED_FOR_UPDATE_ENROLMENT",
    "TOTAL_AMOUNT_CHARGED", "PROCESSING_STATE_DESCRIPTION", "REJECT_REASON_DESCRIPTION",
    "PACKET_SKIPPED", "FOREIGN_RESIDENT", "StationID", "EnrollDT", "EnrollMth", "UpdateType",
    "DistrictManger", "District", "OperatorName", "StationType"
]

@router.post("/upload/preview")
async def preview_eod_file(
    file: UploadFile = File(...),
    current_dm: AdminUser = Depends(require_dm)
):
    """
    Parse uploaded .txt / .csv EOD file, validate headers, compute summary stats, and return a preview.
    """
    if not file.filename.lower().endswith(('.txt', '.csv')):
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Invalid file format. Please upload a .txt or .csv file."
        )

    content_bytes = await file.read()
    try:
        content_text = content_bytes.decode('utf-8-sig') # handles potential UTF-8 BOM
    except UnicodeDecodeError:
        try:
            content_text = content_bytes.decode('latin-1')
        except Exception:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail="Unable to decode file. Please ensure the file is plain text UTF-8 or ASCII."
            )

    reader = csv.reader(io.StringIO(content_text))
    rows = list(reader)

    if not rows or len(rows) < 2:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="File is empty or contains no data rows."
        )

    # Validate header
    raw_headers = [h.strip() for h in rows[0]]
    header_map = {h: idx for idx, h in enumerate(raw_headers)}
    
    missing_cols = [col for col in REQUIRED_EOD_COLUMNS if col not in header_map]
    if missing_cols:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=f"Missing required columns in file: {', '.join(missing_cols[:5])}"
        )

    records = []
    total_rev = 0.0
    dates = []
    stations = set()
    category_counts = collections.defaultdict(int)
    file_dms = set()

    for row_idx, row in enumerate(rows[1:], start=2):
        if not row or all(c.strip() == "" for c in row):
            continue

        def get_val(col_name: str) -> str:
            idx = header_map.get(col_name)
            if idx is not None and idx < len(row):
                return row[idx].strip()
            return ""

        enrolment_no = get_val("ENROLMENT_NO_DATE")
        if not enrolment_no:
            continue

        tot_amount_str = get_val("TOTAL_AMOUNT_CHARGED")
        try:
            tot_amount = float(tot_amount_str)
        except ValueError:
            tot_amount = 0.0
        total_rev += tot_amount

        dt = get_val("EnrollDT")
        if dt:
            dates.append(dt)

        sid = get_val("StationID")
        if sid:
            stations.add(sid)

        utype = get_val("UpdateType") or "Other"
        category_counts[utype] += 1

        dm_val = get_val("DistrictManger")
        if dm_val:
            file_dms.add(dm_val)

        rec = {
            "slno": get_val("SLNO"),
            "enrolment_no_date": enrolment_no,
            "appointment_id": get_val("APPOINTMENT_ID"),
            "type": get_val("TYPE"),
            "mandatory_bio_metric_update_only": get_val("MANDATORY_BIO_METRIC_UPDATE_ONLY"),
            "is_nri": get_val("IS_NRI"),
            "tin_no": get_val("TIN_NO"),
            "operator_id": get_val("OPERATOR_ID"),
            "introducer": get_val("INTRODUCER"),
            "proof": get_val("PROOF"),
            "resident_name": get_val("RESIDENT_NAME"),
            "status": get_val("STATUS"),
            "gst_amount": get_val("GST_AMOUNT"),
            "amount_charged_for_new_enrolment": get_val("AMOUNT_CHARGED_FOR_NEW_ENROLMENT"),
            "amount_charged_for_update_enrolment": get_val("AMOUNT_CHARGED_FOR_UPDATE_ENROLMENT"),
            "total_amount_charged": tot_amount_str,
            "processing_state_description": get_val("PROCESSING_STATE_DESCRIPTION"),
            "reject_reason_description": get_val("REJECT_REASON_DESCRIPTION"),
            "packet_skipped": get_val("PACKET_SKIPPED"),
            "foreign_resident": get_val("FOREIGN_RESIDENT"),
            "station_id": sid,
            "enroll_dt": dt,
            "enroll_mth": get_val("EnrollMth"),
            "update_type": utype,
            "district_manager": dm_val,
            "district": get_val("District"),
            "operator_name": get_val("OperatorName"),
            "station_type": get_val("StationType"),
        }
        records.append(rec)

    if not records:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="No valid data records found in file."
        )

    # Format Date Range
    dates_sorted = sorted(dates)
    min_date = dates_sorted[0] if dates_sorted else "N/A"
    max_date = dates_sorted[-1] if dates_sorted else "N/A"
    if len(min_date) == 8:
        min_date = f"{min_date[0:4]}-{min_date[4:6]}-{min_date[6:8]}"
    if len(max_date) == 8:
        max_date = f"{max_date[0:4]}-{max_date[4:6]}-{max_date[6:8]}"

    dm_in_file = list(file_dms)[0] if file_dms else "Unknown"
    dm_matches = (dm_in_file.lower() == current_dm.name.lower()) or (current_dm.role.value == "admin")

    category_mix = [
        {"category": k, "count": v}
        for k, v in sorted(category_counts.items(), key=lambda x: x[1], reverse=True)
    ]

    return {
        "is_valid": True,
        "total_records": len(records),
        "total_revenue": round(total_rev, 2),
        "target_month": records[0]["enroll_mth"] if records else "N/A",
        "stations": sorted(list(stations)),
        "date_range": {"start": min_date, "end": max_date},
        "dm_name_in_file": dm_in_file,
        "dm_name_logged_in": current_dm.name,
        "dm_matches": dm_matches,
        "category_mix": category_mix,
        "sample_rows": records[:25],
        "records": records
    }

@router.post("/upload/commit")
async def commit_eod_file(
    payload: Dict[str, Any] = Body(...),
    current_dm: AdminUser = Depends(require_dm)
):
    """
    Commit pre-validated records to Azure SQL via Go microservice.
    """
    records = payload.get("records", [])
    if not records:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="No records provided to commit."
        )

    try:
        res = await GoServiceClient.bulk_insert_records(records)
        return {
            "success": True,
            "inserted_count": res.get("inserted_count", len(records)),
            "message": f"Successfully committed {len(records)} records to database."
        }
    except Exception as e:
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Database insertion failed: {str(e)}"
        )

