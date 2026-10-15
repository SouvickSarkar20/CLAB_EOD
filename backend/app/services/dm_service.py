import uuid
import secrets
import string
from typing import Optional, List, Tuple
from fastapi import HTTPException, status
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy import select, func, or_, update, distinct

from app.models import (
    AdminUser, UserRole, UserStatus,
    StationAssignment, District, Station, AuditLog
)
from app.utils.security import get_password_hash
from app.schemas.district_manager import (
    CreateDMRequest, UpdateDMRequest, DMListItem, DMDetail, DMStationAssignmentSchema
)
from app.services.go_client import GoServiceClient

def generate_temp_password(length: int = 12) -> str:
    """Generate a secure temporary password."""
    chars = string.ascii_letters + string.digits + "!@#$%^&*"
    return "".join(secrets.choice(chars) for _ in range(length))

class DMService:
    @staticmethod
    async def list_dms(
        db: AsyncSession,
        status_filter: Optional[str] = None,
        district_id: Optional[int] = None,
        search: Optional[str] = None,
        page: int = 1,
        page_size: int = 25
    ) -> Tuple[List[DMListItem], int]:
        """List District Managers with filtering and pagination."""
        query = select(AdminUser).where(AdminUser.role == UserRole.DISTRICT_MANAGER)

        if status_filter:
            try:
                st_enum = UserStatus(status_filter.lower())
                query = query.where(AdminUser.status == st_enum)
            except ValueError:
                pass

        if search:
            s = f"%{search.strip()}%"
            query = query.where(
                or_(
                    AdminUser.name.ilike(s),
                    AdminUser.email.ilike(s),
                    AdminUser.dmid.ilike(s)
                )
            )

        if district_id:
            subq = select(StationAssignment.dm_user_id).where(StationAssignment.district_id == district_id)
            query = query.where(AdminUser.id.in_(subq))

        count_stmt = select(func.count()).select_from(query.subquery())
        total = (await db.execute(count_stmt)).scalar() or 0

        # If local database has no DMs, fetch directly from Azure SQL via Go
        if total == 0:
            try:
                from datetime import datetime, timezone
                azure_users = await GoServiceClient.list_users()
                dm_items = []
                for u in azure_users:
                    if "admin" in u.get("role", "").lower():
                        continue
                    dm_name = u.get("district_manager", "")
                    st_cnt = 0
                    dist_names = []
                    try:
                        st_data = await GoServiceClient.get_dm_stations(dm_name)
                        st_list = st_data.get("stations", [])
                        st_cnt = len(st_list)
                        dist_names = sorted(list(set(s.get("district_name") for s in st_list if s.get("district_name"))))
                    except Exception:
                        pass

                    u_id = uuid.uuid5(uuid.NAMESPACE_DNS, u.get("id", ""))
                    dm_items.append(
                        DMListItem(
                            id=u_id,
                            dmid=u.get("id", ""),
                            name=dm_name,
                            email=f"{u.get('id', '').lower()}@eod.internal",
                            status=UserStatus.ACTIVE,
                            assigned_stations_count=st_cnt,
                            assigned_districts=dist_names,
                            last_login_at=None,
                            created_at=datetime.now(timezone.utc).replace(tzinfo=None)
                        )
                    )
                return dm_items, len(dm_items)
            except Exception:
                pass

        offset = (page - 1) * page_size
        query = query.order_by(AdminUser.name.asc()).offset(offset).limit(page_size)
        res = await db.execute(query)
        dms = res.scalars().all()

        dm_ids = [dm.id for dm in dms]
        asgn_map = {}
        # Fetch station counts directly from Azure SQL WorkingKIT via Go
        for dm in dms:
            try:
                st_data = await GoServiceClient.get_dm_stations(dm.name)
                stations_list = st_data.get("stations", [])
                st_cnt = len(stations_list)
                dist_names = sorted(list(set(s.get("district_name") for s in stations_list if s.get("district_name"))))
                asgn_map[dm.id] = (st_cnt, dist_names)
            except Exception:
                pass

        if not asgn_map and dm_ids:
            batch_query = (
                select(
                    StationAssignment.dm_user_id,
                    func.count(distinct(StationAssignment.station_id)),
                    func.array_agg(distinct(District.district_name))
                )
                .select_from(StationAssignment)
                .join(District, StationAssignment.district_id == District.district_id, isouter=True)
                .where(StationAssignment.dm_user_id.in_(dm_ids))
                .group_by(StationAssignment.dm_user_id)
            )
            asgn_rows = (await db.execute(batch_query)).all()
            for r in asgn_rows:
                dm_uid = r[0]
                st_cnt = r[1] or 0
                dist_names = [d for d in (r[2] or []) if d]
                asgn_map[dm_uid] = (st_cnt, dist_names)

        dm_items = []
        for dm in dms:
            st_count, dist_names = asgn_map.get(dm.id, (0, []))
            dm_items.append(
                DMListItem(
                    id=dm.id,
                    dmid=dm.dmid,
                    name=dm.name,
                    email=dm.email,
                    status=dm.status,
                    assigned_stations_count=st_count,
                    assigned_districts=dist_names,
                    last_login_at=dm.last_login_at,
                    created_at=dm.created_at
                )
            )

        return dm_items, total

    @staticmethod
    async def get_dm_detail(db: AsyncSession, dm_id: uuid.UUID) -> Optional[DMDetail]:
        """Get full DM profile and station assignments."""
        stmt = select(AdminUser).where(AdminUser.id == dm_id, AdminUser.role == UserRole.DISTRICT_MANAGER)
        res = await db.execute(stmt)
        dm = res.scalar_one_or_none()
        if not dm:
            return None

        # 1. Fetch real assigned stations from Azure SQL WorkingKIT via Go
        assignments = []
        districts_set = set()
        try:
            go_res = await GoServiceClient.get_dm_stations(dm.name)
            for item in go_res.get("stations", []):
                sid = item["station_id"]
                dist = item.get("district_name") or "Unknown"
                districts_set.add(dist)
                assignments.append(
                    DMStationAssignmentSchema(
                        station_id=sid,
                        station_name=f"Station {sid}",
                        district_id=1,
                        district_name=dist,
                        effective_month=202609
                    )
                )
        except Exception:
            # Fallback to local DB if Go engine is unreachable
            asgn_stmt = (
                select(
                    StationAssignment.station_id,
                    Station.station_name,
                    StationAssignment.district_id,
                    District.district_name,
                    StationAssignment.effective_month
                )
                .select_from(StationAssignment)
                .join(Station, StationAssignment.station_id == Station.station_id, isouter=True)
                .join(District, StationAssignment.district_id == District.district_id, isouter=True)
                .where(StationAssignment.dm_user_id == dm.id)
                .order_by(StationAssignment.effective_month.desc())
            )
            asgn_rows = (await db.execute(asgn_stmt)).all()
            seen_stations = set()
            for r in asgn_rows:
                if r[0] in seen_stations:
                    continue
                seen_stations.add(r[0])
                dist_name = r[3] or "Unknown"
                districts_set.add(dist_name)
                assignments.append(
                    DMStationAssignmentSchema(
                        station_id=r[0],
                        station_name=r[1] or f"Station {r[0]}",
                        district_id=r[2],
                        district_name=dist_name,
                        effective_month=r[4]
                    )
                )

        return DMDetail(
            id=dm.id,
            dmid=dm.dmid,
            name=dm.name,
            email=dm.email,
            status=dm.status,
            assigned_stations=assignments,
            assigned_districts=sorted(list(districts_set)),
            last_login_at=dm.last_login_at,
            created_at=dm.created_at,
            updated_at=dm.updated_at
        )

    @staticmethod
    async def create_dm(
        db: AsyncSession,
        admin_user: AdminUser,
        payload: CreateDMRequest,
        ip_address: Optional[str] = None
    ) -> Tuple[DMDetail, str]:
        """Create a new District Manager account with initial temp password."""
        email_clean = payload.email.strip().lower()
        
        # Check email uniqueness
        existing_email = (await db.execute(select(AdminUser).where(AdminUser.email == email_clean))).scalar_one_or_none()
        if existing_email:
            raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail=f"User with email '{email_clean}' already exists")

        # Safely generate next collision-free DMID if not supplied
        if not payload.dmid:
            res = await db.execute(
                select(AdminUser.dmid).where(
                    AdminUser.role == UserRole.DISTRICT_MANAGER,
                    AdminUser.dmid.like("ClabDM%")
                )
            )
            existing_dmids = res.scalars().all()
            max_idx = 0
            for d in existing_dmids:
                try:
                    num = int(d.replace("ClabDM", "").strip())
                    if num > max_idx:
                        max_idx = num
                except ValueError:
                    pass
            
            next_idx = max_idx + 1
            candidate = f"ClabDM{next_idx:02d}"
            while True:
                chk = (await db.execute(select(AdminUser).where(AdminUser.dmid == candidate))).scalar_one_or_none()
                if not chk:
                    break
                next_idx += 1
                candidate = f"ClabDM{next_idx:02d}"
            payload.dmid = candidate

        dmid_clean = payload.dmid.strip()
        existing_dmid = (await db.execute(select(AdminUser).where(AdminUser.dmid == dmid_clean))).scalar_one_or_none()
        if existing_dmid:
            payload.dmid = f"ClabDM{uuid.uuid4().hex[:4].upper()}"

        temp_password = generate_temp_password()
        new_dm = AdminUser(
            dmid=payload.dmid.strip(),
            name=payload.name.strip(),
            email=email_clean,
            role=UserRole.DISTRICT_MANAGER,
            status=UserStatus.ACTIVE,
            password_hash=get_password_hash(temp_password),
            is_demo_creds=True,
            must_change_password=True,
        )
        db.add(new_dm)
        await db.flush()

        # Apply initial district assignments if provided
        if payload.district_ids:
            await db.execute(
                update(StationAssignment)
                .where(StationAssignment.district_id.in_(payload.district_ids))
                .values(dm_user_id=new_dm.id)
            )

        log = AuditLog(
            user_id=admin_user.id,
            action="DM_CREATED",
            resource_type="AdminUser",
            resource_id=str(new_dm.id),
            details={"dmid": new_dm.dmid, "name": new_dm.name, "email": new_dm.email, "district_ids": payload.district_ids},
            ip_address=ip_address
        )
        db.add(log)
        await db.commit()

        detail = await DMService.get_dm_detail(db, new_dm.id)
        return detail, temp_password

    @staticmethod
    async def update_dm(
        db: AsyncSession,
        admin_user: AdminUser,
        dm_id: uuid.UUID,
        payload: UpdateDMRequest,
        ip_address: Optional[str] = None
    ) -> Optional[DMDetail]:
        """Update District Manager profile fields, status, and district assignments."""
        stmt = select(AdminUser).where(AdminUser.id == dm_id, AdminUser.role == UserRole.DISTRICT_MANAGER)
        res = await db.execute(stmt)
        dm = res.scalar_one_or_none()
        if not dm:
            return None

        diffs = {}
        if payload.name is not None:
            new_name = payload.name.strip()
            if new_name and new_name != dm.name:
                diffs["name"] = {"before": dm.name, "after": new_name}
                dm.name = new_name

        if payload.email is not None:
            new_e = payload.email.strip().lower()
            if new_e != dm.email.lower():
                existing_email = (await db.execute(select(AdminUser).where(AdminUser.email == new_e))).scalar_one_or_none()
                if existing_email and existing_email.id != dm.id:
                    raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail=f"Email '{new_e}' is already used by another user")
                diffs["email"] = {"before": dm.email, "after": new_e}
                dm.email = new_e

        if payload.status is not None:
            target_status = UserStatus(payload.status) if isinstance(payload.status, str) else payload.status
            if target_status != dm.status:
                before_val = dm.status.value if hasattr(dm.status, 'value') else str(dm.status)
                after_val = target_status.value if hasattr(target_status, 'value') else str(target_status)
                diffs["status"] = {"before": before_val, "after": after_val}
                dm.status = target_status

        if payload.district_ids is not None:
            diffs["district_ids"] = payload.district_ids
            if payload.district_ids:
                await db.execute(
                    update(StationAssignment)
                    .where(StationAssignment.district_id.in_(payload.district_ids))
                    .values(dm_user_id=dm.id)
                )

        if payload.station_ids is not None:
            diffs["station_ids"] = payload.station_ids
            # Assign stations directly in Azure SQL WorkingKIT via Go Data Engine
            try:
                await GoServiceClient.assign_stations(dm.name, payload.station_ids)
            except Exception as e:
                import logging
                logging.error(f"[ASSIGN_STATIONS] Failed to assign stations in Azure SQL WorkingKIT: {e}", exc_info=True)

        if diffs:
            log = AuditLog(
                user_id=admin_user.id,
                action="DM_UPDATED",
                resource_type="AdminUser",
                resource_id=str(dm.id),
                details=diffs,
                ip_address=ip_address
            )
            db.add(log)
            await db.commit()

        return await DMService.get_dm_detail(db, dm.id)

    @staticmethod
    async def deactivate_dm(
        db: AsyncSession,
        admin_user: AdminUser,
        dm_id: uuid.UUID,
        ip_address: Optional[str] = None
    ) -> bool:
        """Deactivate (soft delete) a District Manager."""
        stmt = select(AdminUser).where(AdminUser.id == dm_id, AdminUser.role == UserRole.DISTRICT_MANAGER)
        res = await db.execute(stmt)
        dm = res.scalar_one_or_none()
        if not dm:
            return False

        dm.status = UserStatus.INACTIVE
        log = AuditLog(
            user_id=admin_user.id,
            action="DM_DEACTIVATED",
            resource_type="AdminUser",
            resource_id=str(dm.id),
            details={"dmid": dm.dmid, "name": dm.name},
            ip_address=ip_address
        )
        db.add(log)
        await db.commit()
        return True

    @staticmethod
    async def reset_dm_password(
        db: AsyncSession,
        admin_user: AdminUser,
        dm_id: uuid.UUID,
        custom_password: Optional[str] = None,
        ip_address: Optional[str] = None
    ) -> Optional[Tuple[AdminUser, str]]:
        """Reset a District Manager's password and return fresh temp/custom password."""
        stmt = select(AdminUser).where(AdminUser.id == dm_id, AdminUser.role == UserRole.DISTRICT_MANAGER)
        res = await db.execute(stmt)
        dm = res.scalar_one_or_none()
        if not dm:
            return None

        temp_password = custom_password.strip() if custom_password and custom_password.strip() else generate_temp_password()
        
        # Sync password to Azure SQL DMInfo via Go microservice
        dm_identifier = dm.dmid or dm.name
        try:
            await GoServiceClient.update_password(dm_identifier, temp_password)
        except Exception as e:
            import logging
            logging.error(f"[RESET_DM_PASSWORD] Failed to update password in Azure SQL DMInfo: {e}", exc_info=True)
            raise HTTPException(
                status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
                detail=f"Failed to reset password in database: {str(e)}"
            )

        dm.password_hash = get_password_hash(temp_password)
        dm.must_change_password = True
        dm.is_demo_creds = False

        log = AuditLog(
            user_id=admin_user.id,
            action="DM_PASSWORD_RESET",
            resource_type="AdminUser",
            resource_id=str(dm.id),
            details={"dmid": dm.dmid, "name": dm.name, "is_custom": bool(custom_password)},
            ip_address=ip_address
        )
        db.add(log)
        await db.commit()
        return dm, temp_password
