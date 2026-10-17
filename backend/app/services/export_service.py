import io
import uuid
import pandas as pd
from datetime import date
from typing import Optional, List, Dict, Any
from fastapi import Response, HTTPException, status
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy import select, func, desc, and_

from app.models import (
    MonthlySummary,
    DailyRecord,
    StationAssignment,
    Station,
    District,
    AdminUser,
)
from app.services.analytics_service import AnalyticsService
from app.services.anomaly_service import AnomalyService
from app.services.go_client import GoServiceClient

class ExportService:

    @staticmethod
    def _generate_excel(df: pd.DataFrame, sheet_name: str = "Report") -> bytes:
        """Helper to create a formatted Excel file with xlsxwriter."""
        output = io.BytesIO()
        with pd.ExcelWriter(output, engine="xlsxwriter") as writer:
            df.to_excel(writer, sheet_name=sheet_name, index=False)
            workbook = writer.book
            worksheet = writer.sheets[sheet_name]

            # Header format
            header_format = workbook.add_format({
                "bold": True,
                "text_wrap": True,
                "valign": "center",
                "fg_color": "#1E293B",  # Dark slate background
                "font_color": "#FFFFFF", # White text
                "border": 1
            })

            # Write formatted headers
            for col_num, value in enumerate(df.columns.values):
                worksheet.write(0, col_num, value, header_format)

            # Auto-adjust column widths
            for i, col in enumerate(df.columns):
                max_len = max(
                    df[col].astype(str).map(len).max() if len(df) > 0 else 0,
                    len(str(col))
                ) + 4
                worksheet.set_column(i, i, min(max_len, 50))

        return output.getvalue()

    @staticmethod
    def _generate_csv(df: pd.DataFrame) -> bytes:
        """Helper to create UTF-8 encoded CSV bytes."""
        return df.to_csv(index=False).encode("utf-8")

    @classmethod
    def create_export_response(
        cls,
        df: pd.DataFrame,
        filename_prefix: str,
        export_format: str = "xlsx"
    ) -> Response:
        """Format DataFrame into XLSX or CSV Response with Content-Disposition headers."""
        fmt = export_format.lower()
        if fmt == "csv":
            content = cls._generate_csv(df)
            media_type = "text/csv"
            filename = f"{filename_prefix}.csv"
        else:
            content = cls._generate_excel(df, sheet_name=filename_prefix[:30])
            media_type = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
            filename = f"{filename_prefix}.xlsx"

        return Response(
            content=content,
            media_type=media_type,
            headers={
                "Content-Disposition": f'attachment; filename="{filename}"',
                "X-Export-Filename": filename
            }
        )

    @classmethod
    async def export_monthly(
        cls,
        db: AsyncSession,
        month: Optional[int] = None,
        district_id: Optional[int] = None,
        dm_id: Optional[str] = None,
        station_id: Optional[str] = None,
        operator_code: Optional[str] = None,
        export_format: str = "xlsx"
    ) -> Response:
        """Export Monthly summary records to XLSX/CSV querying Azure SQL CLAB-EOD with fallback."""
        rows: List[Dict[str, Any]] = []

        # 1. Primary: Query Azure SQL CLAB-EOD via Go microservice
        month_str = str(month) if month else ""
        dm_str = str(dm_id) if dm_id else None
        try:
            go_data = await GoServiceClient.get_monthly_records(
                month=month_str,
                dm=dm_str,
                station_id=station_id,
                operator_code=operator_code,
                page=1,
                page_size=50000
            )
            for m in go_data.get("items", []):
                rows.append({
                    "Month (YYYYMM)": m.get("enroll_month") or month_str or "All",
                    "District Name": m.get("district") or "N/A",
                    "DM Name": m.get("district_manager") or "Unassigned",
                    "Station ID": m.get("station_id") or "N/A",
                    "Station Name": m.get("station_name") or f"Station {m.get('station_id')}",
                    "Operator Code": m.get("operator_code") or m.get("operator_name") or "N/A",
                    "Total Enrollment": m.get("total_enrollment", 0),
                    "Total Amount (INR)": float(m.get("total_amount", 0.0)),
                    "BMU 100": m.get("bmu_100", 0),
                    "DMU 50": m.get("dmu_50", 0),
                    "MBU 0": m.get("mbu_0", 0),
                    "MBU 100": m.get("mbu_100", 0),
                    "NEW 0": m.get("new_0", 0),
                    "BMU 125": m.get("bmu_125", 0),
                    "DMU 75": m.get("dmu_75", 0),
                    "MBU 125": m.get("mbu_125", 0),
                })
        except Exception as e:
            import logging
            logging.warning(f"[EXPORT_MONTHLY] Go service query failed, falling back to local DB: {e}")

        # 2. Fallback to local SQLite if Go service returned no rows
        if not rows:
            items, _ = await AnalyticsService.get_monthly_records(
                db=db,
                month=month,
                district_id=district_id,
                dm_id=None,
                station_id=station_id,
                operator_code=operator_code,
                page=1,
                page_size=10000
            )
            for m in items:
                rows.append({
                    "Month (YYYYMM)": m.enroll_month,
                    "District Name": m.district_name or "N/A",
                    "DM Name": m.dm_name or "Unassigned",
                    "Station ID": m.station_id,
                    "Station Name": m.station_name or m.station_id,
                    "Operator Code": m.operator_code,
                    "Total Enrollment": m.total_enrollment,
                    "Total Amount (INR)": float(m.total_amount),
                    "BMU 100": m.bmu_100,
                    "DMU 50": m.dmu_50,
                    "MBU 0": m.mbu_0,
                    "MBU 100": m.mbu_100,
                    "NEW 0": m.new_0,
                    "BMU 125": m.bmu_125,
                    "DMU 75": m.dmu_75,
                    "MBU 125": m.mbu_125,
                })

        df = pd.DataFrame(rows)
        filename = f"monthly_summary_{month if month else 'all'}"
        return cls.create_export_response(df, filename_prefix=filename, export_format=export_format)

    @classmethod
    async def export_daily(
        cls,
        db: AsyncSession,
        month: Optional[int] = None,
        start_date: Optional[date] = None,
        end_date: Optional[date] = None,
        district_id: Optional[int] = None,
        dm_id: Optional[str] = None,
        station_id: Optional[str] = None,
        operator_code: Optional[str] = None,
        export_format: str = "xlsx"
    ) -> Response:
        """Export Daily operational records to XLSX/CSV querying Azure SQL CLAB-EOD with fallback."""
        rows: List[Dict[str, Any]] = []

        # 1. Primary: Query Azure SQL CLAB-EOD via Go microservice
        month_str = str(month) if month else ""
        dm_str = str(dm_id) if dm_id else None
        date_str = str(start_date) if start_date else None
        try:
            go_data = await GoServiceClient.get_daily_logs(
                month=month_str,
                dm=dm_str,
                station_id=station_id,
                date=date_str,
                page=1,
                page_size=50000
            )
            for d in go_data.get("items", []):
                rows.append({
                    "Enrollment Date": str(d.get("enroll_date") or ""),
                    "District Name": d.get("district") or "N/A",
                    "DM Name": d.get("district_manager") or "Unassigned",
                    "Station ID": d.get("station_id") or "N/A",
                    "Station Name": f"Station {d.get('station_id')}",
                    "Operator Code": d.get("operator_name") or "N/A",
                    "Total Enrollment": d.get("total_enrollment", 0),
                    "Total Amount (INR)": float(d.get("total_amount", 0.0)),
                    "BMU 100": d.get("bmu_100", 0),
                    "DMU 50": d.get("dmu_50", 0),
                    "MBU 0": d.get("mbu_0", 0),
                    "MBU 100": d.get("mbu_100", 0),
                    "NEW 0": d.get("new_0", 0),
                    "BMU 125": d.get("bmu_125", 0),
                    "DMU 75": d.get("dmu_75", 0),
                    "MBU 125": d.get("mbu_125", 0),
                })
        except Exception as e:
            import logging
            logging.warning(f"[EXPORT_DAILY] Go service query failed, falling back to local DB: {e}")

        # 2. Fallback to local SQLite if Go service returned no rows
        if not rows:
            items, _ = await AnalyticsService.get_daily_records(
                db=db,
                month=month,
                start_date=start_date,
                end_date=end_date,
                district_id=district_id,
                dm_id=None,
                station_id=station_id,
                operator_code=operator_code,
                page=1,
                page_size=50000
            )
            for d in items:
                rows.append({
                    "Enrollment Date": str(d.enroll_date),
                    "District Name": d.district_name or "N/A",
                    "DM Name": d.dm_name or "Unassigned",
                    "Station ID": d.station_id,
                    "Station Name": d.station_name or d.station_id,
                    "Operator Code": d.operator_code,
                    "Total Enrollment": d.total_enrollment,
                    "Total Amount (INR)": float(d.total_amount),
                    "BMU 100": d.bmu_100,
                    "DMU 50": d.dmu_50,
                    "MBU 0": d.mbu_0,
                    "MBU 100": d.mbu_100,
                    "NEW 0": d.new_0,
                    "BMU 125": d.bmu_125,
                    "DMU 75": d.dmu_75,
                    "MBU 125": d.mbu_125,
                })

        df = pd.DataFrame(rows)
        filename = f"daily_records_{month if month else 'period'}"
        return cls.create_export_response(df, filename_prefix=filename, export_format=export_format)

    @classmethod
    async def export_anomalies(
        cls,
        db: AsyncSession,
        month: Optional[int] = None,
        district_id: Optional[int] = None,
        dm_id: Optional[uuid.UUID] = None,
        anomaly_type: Optional[str] = None,
        severity: Optional[str] = None,
        export_format: str = "xlsx"
    ) -> Response:
        """Export Anomaly report to XLSX/CSV."""
        report = await AnomalyService.get_anomalies(
            db=db,
            month=month,
            district_id=district_id,
            dm_id=dm_id,
            anomaly_type=anomaly_type,
            severity=severity
        )

        rows: List[Dict[str, Any]] = []
        for a in report.items:
            rows.append({
                "Anomaly ID": a.id,
                "Type": a.type,
                "Severity": a.severity,
                "Headline": a.title,
                "Description": a.description,
                "District Name": a.district_name or "N/A",
                "DM Name": a.dm_name or "N/A",
                "Station Code": a.station_code or "N/A",
                "Station Name": a.station_name or "N/A",
                "Metrics": str(a.metrics),
                "Detected At": a.detected_at,
            })

        df = pd.DataFrame(rows)
        filename = f"operational_anomalies_{month if month else 'latest'}"
        return cls.create_export_response(df, filename_prefix=filename, export_format=export_format)
