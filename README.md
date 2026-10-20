# Computer LAB EOD Operations & Analytics Portal

A high-performance enterprise analytics and operations supervision platform engineered to monitor, process, and analyze **1,500,000+ End-of-Day (EOD) Aadhaar enrollment records** across districts, stations, and field operators.

---

## System Architecture

The application adopts a **BFF (Backend-for-Frontend) + High-Performance Microservice** architecture:

```
+--------------------------------+        +--------------------------------------------------+
|        Frontend (Vercel)       |        |              Unified Container (Render)          |
|   React 18 + Vite + Tailwind   |        |                                                  |
|   TanStack Query + Recharts    |------->|  FastAPI (Python) Gateway  [Port: 8000]          |
+--------------------------------+  HTTP  |           |                                      |
                                          |           v (Keep-Alive Pool < 2ms)              |
                                          |  Go Core Engine (sqlserver) [Port: 8081]         |
                                          +--------------------+-----------------------------+
                                                               | TLS / NOLOCK
                                                               v
                                                  +--------------------------+
                                                  |    Azure SQL Database    |
                                                  |  EODFiles (1.5M+ rows)   |
                                                  |  WorkingKIT (Stations)   |
                                                  |  EODDB_Users (Auth)      |
                                                  +--------------------------+
```

1. **Frontend (Vercel)**: React 18 SPA with Vite, Tailwind CSS, TanStack React Query, Lucide icons, and Recharts.
2. **Gateway Layer (FastAPI)**: JWT authentication, role enforcement (Admin vs. District Manager), request validation, and lightweight local session caching.
3. **Core Data Engine (Go Microservice)**: Compiled static binary communicating with Azure SQL Server over connection pools with in-memory caching and non-blocking `WITH (NOLOCK)` queries.
4. **Data Layer (Azure SQL Server)**: Houses `EODFiles`, `EOD_202607`, `WorkingKIT`, and `EODDB_Users`.

---

## Live Deployments

- **Frontend Application**: https://clab-eod.vercel.app
- **Backend API & Data Engine**: https://clab-eod.onrender.com
- **API Health Check**: https://clab-eod.onrender.com/api/health
- **Interactive API Documentation**: https://clab-eod.onrender.com/docs

---

## Demo Credentials

| Role | Username / ID | Password | Access Scope |
| :--- | :--- | :--- | :--- |
| **System Administrator** | `ADMIN` *(or `admin@eod.internal`)* | `Admin@123456` | Full statewide overview, station assignment, user auditing, and cross-district analytics |
| **District Manager (Cuttack)** | `DM02` | `utkl02` | Scoped to Utkal Keshari Nayak's assigned stations & EOD uploads |
| **District Manager (Khurda)** | `DM01` | `abhi01` | Scoped to Abhimanyu Das's assigned stations |
| **District Manager (Puri)** | `DM03` | `sand03` | Scoped to Sandeep Jena's assigned stations |

---

## Key Features

### 1. Executive Operations Overview
- **Real-Time KPIs**: Total statewide enrollment volume, revenue collections, active field days, and operator metrics.
- **Visual Analytics**: Interactive daily enrollment trends, fee category mix (`BMU @ 100`, `BMU @ 125`, `DMU @ 75`, `MBU @ 0`, `NEW @ 0`), and top-performing stations.
- **Period-Aware Cascading Filters**: Filter seamlessly across enrollment months (2023–2026), districts, and district managers.

### 2. District Manager Management & Station Reassignment
- Dynamic assignment of field kits (`WorkingKIT`) to District Managers with real-time updates directly in Azure SQL.
- Interactive station selector with pre-selected state tracking and instant search.

### 3. Monthly & Daily Station Rollups
- Paginated table rollups grouping production by Station ID, Date, and Field Operator.
- Fast sub-second response times on 1.5M+ rows using non-blocking dirty read semantics (`WITH (NOLOCK)`) and targeted indexing (`IX_EODFiles_EnrollMth`, `IX_EODFiles_EnrollMth_DM`).

### 4. Daily EOD File Processing & Preview
- Multi-file and drag-and-drop CSV upload for field operators with client-side batching and preview tables.
- Transactional bulk insertion into Azure SQL with filter cache invalidation.

---

## Local Development Setup

### Prerequisites
- Node.js 18+ and npm
- Go 1.22+
- Python 3.10+
- Azure SQL Database credentials

### 1. Clone Repository
```bash
git clone https://github.com/SouvickSarkar20/CLAB_EOD.git
cd CLAB_EOD
```

### 2. Configure Environment Variables

**Backend (`backend/.env`):**
```ini
ENVIRONMENT=development
PORT=8000
GO_SERVICE_URL=http://localhost:8081
SECRET_KEY=your_secure_random_jwt_secret_key_here
FRONTEND_URL=http://localhost:5173
```

**Go Microservice (`go-service/.env`):**
```ini
PORT=8081
AZURE_SQL_SERVER=eod-db.database.windows.net
AZURE_SQL_PORT=1433
AZURE_SQL_DATABASE=EOD_1509250818am
AZURE_SQL_USER=Clab_EOD
AZURE_SQL_PASSWORD=csv@1040
```

**Frontend (`frontend/.env`):**
```ini
VITE_API_URL=http://localhost:8000
```

### 3. Run Locally

**Terminal 1 — Go Microservice:**
```bash
cd go-service
go run cmd/server/main.go
```

**Terminal 2 — FastAPI Backend:**
```bash
cd backend
python -m venv venv
# On Windows:
.\venv\Scripts\activate
# On macOS/Linux:
source venv/bin/activate

pip install -r requirements.txt
uvicorn app.main:app --reload --port 8000
```

**Terminal 3 — Frontend:**
```bash
cd frontend
npm install
npm run dev
```

Open your browser at `http://localhost:5173`.

---

## Production Deployment Guide

### Backend on Render (Unified Docker Container)
The backend uses a multi-stage `Dockerfile`:
- Stage 1 compiles the Go binary statically (`CGO_ENABLED=0`).
- Stage 2 installs Python dependencies and uses `entrypoint.sh` to run the Go engine on `8081` in the background and FastAPI on `$PORT` in the foreground with `--access-log`.

**Render Environment Variables:**
- `FRONTEND_URL`: `https://clab-eod.vercel.app`
- `SECRET_KEY`: `<your-jwt-secret>`
- `AZURE_SQL_SERVER`: `eod-db.database.windows.net`
- `AZURE_SQL_PORT`: `1433`
- `AZURE_SQL_DATABASE`: `EOD_1509250818am`
- `AZURE_SQL_USER`: `Clab_EOD`
- `AZURE_SQL_PASSWORD`: `csv@1040`

### Frontend on Vercel
- Root Directory: `frontend`
- Build Command: `npm run build`
- Output Directory: `dist`
- Environment Variable: `VITE_API_URL=https://clab-eod.onrender.com`

---

## Security & Data Integrity
- Passwords verified against Azure SQL `EODDB_Users`.
- Standard JWT Bearer tokens with role-based access control (`admin` vs. `district_manager`).
- CORS strictly restricted to production domain `https://clab-eod.vercel.app` and local dev hosts.
- Non-blocking read locks (`WITH (NOLOCK)`) prevent deadlocks during high-volume production operations.
