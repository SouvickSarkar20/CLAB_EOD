package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"go-service/internal/db"
	"go-service/internal/models"
)

// filterCache caches the expensive GetFilterOptions result in memory
type filterCacheEntry struct {
	data      *models.FilterOptions
	expiresAt time.Time
}

type filterCache struct {
	mu      sync.RWMutex
	entries map[string]filterCacheEntry
}

var globalFilterCache = &filterCache{
	entries: make(map[string]filterCacheEntry),
}

type DataService struct {
	pool *db.Pool
}

func NewDataService(pool *db.Pool) *DataService {
	return &DataService{pool: pool}
}

// TableForMonth resolves the table name. For 202607 it uses EOD_202607, otherwise EODFiles
func (s *DataService) TableForMonth(month string) string {
	if month == "202607" {
		return "EOD_202607"
	}
	return "EODFiles"
}

// ----------------- AUTHENTICATION -----------------

func (s *DataService) Authenticate(ctx context.Context, id, password string) (*models.UserResponse, error) {
	id = strings.TrimSpace(id)
	password = strings.TrimSpace(password)

	query := `
		SELECT DM_ID, District_Manager, Role 
		FROM DMInfo 
		WHERE (LTRIM(RTRIM(DM_ID)) = @id 
		    OR LTRIM(RTRIM(District_Manager)) = @id 
		    OR LTRIM(RTRIM(Mail_ID)) = @id)
		  AND LTRIM(RTRIM(Password)) = @password
	`
	var user models.UserResponse
	err := s.pool.DB.QueryRowContext(ctx, query, sql.Named("id", id), sql.Named("password", password)).
		Scan(&user.ID, &user.DistrictManager, &user.Role)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("invalid credentials")
		}
		return nil, err
	}
	return &user, nil
}

func (s *DataService) GetUserByID(ctx context.Context, id string) (*models.UserResponse, error) {
	id = strings.TrimSpace(id)
	query := `
		SELECT DM_ID, District_Manager, Role 
		FROM DMInfo 
		WHERE LTRIM(RTRIM(DM_ID)) = @id OR LTRIM(RTRIM(District_Manager)) = @id
	`
	var user models.UserResponse
	err := s.pool.DB.QueryRowContext(ctx, query, sql.Named("id", id)).
		Scan(&user.ID, &user.DistrictManager, &user.Role)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *DataService) ListUsers(ctx context.Context) ([]models.UserResponse, error) {
	query := `
		SELECT DM_ID, District_Manager, Role 
		FROM DMInfo 
		ORDER BY TRY_CAST(Sl_No AS INT), DM_ID
	`
	rows, err := s.pool.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.UserResponse
	for rows.Next() {
		var u models.UserResponse
		if err := rows.Scan(&u.ID, &u.DistrictManager, &u.Role); err == nil {
			users = append(users, u)
		}
	}
	return users, nil
}

// ----------------- FILTERS -----------------

func (s *DataService) GetFilterOptions(ctx context.Context, dmName string) (*models.FilterOptions, error) {
	globalFilterCache.mu.RLock()
	entry, found := globalFilterCache.entries[dmName]
	if found && time.Now().Before(entry.expiresAt) {
		globalFilterCache.mu.RUnlock()
		return entry.data, nil
	}
	globalFilterCache.mu.RUnlock()

	opts := &models.FilterOptions{
		Months:    []string{},
		Districts: []string{},
		DMs:       []models.DMOption{},
		Stations:  []string{},
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	// ------ 1. Months from EODMth ------
	wg.Add(1)
	go func() {
		defer wg.Done()
		monthQuery := `SELECT DISTINCT EnrollMth FROM EODMth WITH (NOLOCK) WHERE EnrollMth IS NOT NULL AND LTRIM(RTRIM(EnrollMth)) <> '' ORDER BY EnrollMth DESC`
		mRows, err := s.pool.DB.QueryContext(ctx, monthQuery)
		if err == nil {
			defer mRows.Close()
			var months []string
			for mRows.Next() {
				var m string
				if err := mRows.Scan(&m); err == nil && m != "" {
					months = append(months, m)
				}
			}
			mu.Lock()
			opts.Months = months
			mu.Unlock()
		}
	}()

	// ------ 2. Districts from EODMth ------
	wg.Add(1)
	go func() {
		defer wg.Done()
		distQuery := "SELECT DISTINCT District FROM EODMth WITH (NOLOCK) WHERE District IS NOT NULL AND LTRIM(RTRIM(District)) <> ''"
		var distArgs []any
		if dmName != "" {
			distQuery += " AND (DistrictManger = @dmName OR DMID = @dmName)"
			distArgs = append(distArgs, sql.Named("dmName", dmName))
		}
		distQuery += " ORDER BY District"
		dRows, err := s.pool.DB.QueryContext(ctx, distQuery, distArgs...)
		if err == nil {
			defer dRows.Close()
			var dists []string
			for dRows.Next() {
				var d string
				if err := dRows.Scan(&d); err == nil && d != "" {
					dists = append(dists, d)
				}
			}
			mu.Lock()
			opts.Districts = dists
			mu.Unlock()
		}
	}()

	// ------ 3. DMs from DMInfo ------
	wg.Add(1)
	go func() {
		defer wg.Done()
		dmQuery := `
			SELECT DM_ID, District_Manager 
			FROM DMInfo WITH (NOLOCK) 
			WHERE Role != 'Admin' AND District_Manager IS NOT NULL AND LTRIM(RTRIM(District_Manager)) <> ''
			ORDER BY TRY_CAST(Sl_No AS INT), District_Manager
		`
		dmRows, err := s.pool.DB.QueryContext(ctx, dmQuery)
		if err == nil {
			defer dmRows.Close()
			var dms []models.DMOption
			for dmRows.Next() {
				var id, name string
				if err := dmRows.Scan(&id, &name); err == nil {
					dms = append(dms, models.DMOption{ID: id, Name: name})
				}
			}
			mu.Lock()
			opts.DMs = dms
			mu.Unlock()
		}
	}()

	// ------ 4. Stations / Operators from EODMth ------
	wg.Add(1)
	go func() {
		defer wg.Done()
		stnQuery := "SELECT DISTINCT OPERATOR_ID FROM EODMth WITH (NOLOCK) WHERE OPERATOR_ID IS NOT NULL AND LTRIM(RTRIM(OPERATOR_ID)) <> ''"
		var stnArgs []any
		if dmName != "" {
			stnQuery += " AND (DistrictManger = @dmName OR DMID = @dmName)"
			stnArgs = append(stnArgs, sql.Named("dmName", dmName))
		}
		stnQuery += " ORDER BY OPERATOR_ID"
		stnRows, err := s.pool.DB.QueryContext(ctx, stnQuery, stnArgs...)
		if err == nil {
			defer stnRows.Close()
			var stns []string
			for stnRows.Next() {
				var sid string
				if err := stnRows.Scan(&sid); err == nil && sid != "" {
					stns = append(stns, sid)
				}
			}
			mu.Lock()
			opts.Stations = stns
			mu.Unlock()
		}
	}()

	wg.Wait()

	globalFilterCache.mu.Lock()
	globalFilterCache.entries[dmName] = filterCacheEntry{
		data:      opts,
		expiresAt: time.Now().Add(10 * time.Minute),
	}
	globalFilterCache.mu.Unlock()

	return opts, nil
}


// ----------------- DM OVERVIEW & ANALYTICS -----------------

func (s *DataService) GetDMOverview(ctx context.Context, month, dmName, district, stationID string) (*models.DMOverviewResponse, error) {
	var mthWhere []string
	var mthArgs []any

	if month != "" && month != "all" {
		mthWhere = append(mthWhere, "EnrollMth = @month")
		mthArgs = append(mthArgs, sql.Named("month", month))
	}
	if dmName != "" {
		mthWhere = append(mthWhere, "(DistrictManger = @dmName OR DMID = @dmName)")
		mthArgs = append(mthArgs, sql.Named("dmName", dmName))
	}
	if district != "" {
		mthWhere = append(mthWhere, "District = @district")
		mthArgs = append(mthArgs, sql.Named("district", district))
	}
	if stationID != "" {
		mthWhere = append(mthWhere, "OPERATOR_ID = @stationID")
		mthArgs = append(mthArgs, sql.Named("stationID", stationID))
	}

	mthWhereSQL := ""
	if len(mthWhere) > 0 {
		mthWhereSQL = " WHERE " + strings.Join(mthWhere, " AND ")
	}

	dtWhereSQL := mthWhereSQL
	dtArgs := mthArgs

	resp := &models.DMOverviewResponse{
		DailyTrend:  []models.DailyTrendPoint{},
		TopStations: []models.TopStationData{},
		CategoryMix: []models.CategoryMixItem{},
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	// 1. KPI Aggregation from EODMth
	wg.Add(1)
	go func() {
		defer wg.Done()
		kpiQuery := fmt.Sprintf(`
			SELECT 
				COALESCE(SUM(TRY_CAST(Total_Enrollment AS INT)), 0) as total_enrollment,
				COALESCE(SUM(TRY_CAST(Total_amount AS FLOAT)), 0) as total_revenue
			FROM EODMth WITH (NOLOCK)
			%s
		`, mthWhereSQL)

		var totalEnrollment int
		var totalRev float64
		err := s.pool.DB.QueryRowContext(ctx, kpiQuery, mthArgs...).Scan(&totalEnrollment, &totalRev)
		if err == nil {
			mu.Lock()
			resp.TotalEnrollment = totalEnrollment
			resp.TotalRevenue = totalRev
			mu.Unlock()
		}
	}()

	// 2. Active Days Count from EODDt
	wg.Add(1)
	go func() {
		defer wg.Done()
		daysQuery := fmt.Sprintf(`
			SELECT COUNT(DISTINCT EnnollDt)
			FROM EODDt WITH (NOLOCK)
			%s
		`, dtWhereSQL)

		var activeDays int
		err := s.pool.DB.QueryRowContext(ctx, daysQuery, dtArgs...).Scan(&activeDays)
		if err == nil {
			mu.Lock()
			resp.ActiveDaysCount = activeDays
			mu.Unlock()
		}
	}()

	// 3. Category Mix from EODMth (Direct pre-aggregated columns)
	wg.Add(1)
	go func() {
		defer wg.Done()
		catQuery := fmt.Sprintf(`
			SELECT 
				COALESCE(SUM(TRY_CAST(BMU_100 AS INT)), 0),
				COALESCE(SUM(TRY_CAST(BMU_125 AS INT)), 0),
				COALESCE(SUM(TRY_CAST(DMU_50 AS INT)), 0),
				COALESCE(SUM(TRY_CAST(DMU_75 AS INT)), 0),
				COALESCE(SUM(TRY_CAST(MBU_0 AS INT)), 0),
				COALESCE(SUM(TRY_CAST(MBU_100 AS INT)), 0),
				COALESCE(SUM(TRY_CAST(MBU_125 AS INT)), 0),
				COALESCE(SUM(TRY_CAST(NEW_0 AS INT)), 0)
			FROM EODMth WITH (NOLOCK)
			%s
		`, mthWhereSQL)

		var bmu100, bmu125, dmu50, dmu75, mbu0, mbu100, mbu125, new0 int
		err := s.pool.DB.QueryRowContext(ctx, catQuery, mthArgs...).Scan(
			&bmu100, &bmu125, &dmu50, &dmu75, &mbu0, &mbu100, &mbu125, &new0,
		)
		if err == nil {
			rawCategories := []models.CategoryMixItem{
				{Category: "DMU @ 75", Count: dmu75},
				{Category: "MBU @ 0", Count: mbu0},
				{Category: "BMU @ 125", Count: bmu125},
				{Category: "BMU @ 100", Count: bmu100},
				{Category: "NEW @ 0", Count: new0},
				{Category: "DMU @ 50", Count: dmu50},
				{Category: "MBU @ 100", Count: mbu100},
				{Category: "MBU @ 125", Count: mbu125},
			}

			// Sort by count descending
			for i := 0; i < len(rawCategories)-1; i++ {
				for j := i + 1; j < len(rawCategories); j++ {
					if rawCategories[i].Count < rawCategories[j].Count {
						rawCategories[i], rawCategories[j] = rawCategories[j], rawCategories[i]
					}
				}
			}

			var filtered []models.CategoryMixItem
			for _, item := range rawCategories {
				if item.Count > 0 {
					filtered = append(filtered, item)
				}
			}

			mu.Lock()
			resp.CategoryMix = filtered
			mu.Unlock()
		}
	}()

	// 4. Daily Trend from EODDt
	wg.Add(1)
	go func() {
		defer wg.Done()
		trendQuery := fmt.Sprintf(`
			SELECT 
				EnnollDt,
				COALESCE(SUM(TRY_CAST(Total_Enrollment AS INT)), 0) as daily_enrollment,
				COALESCE(SUM(TRY_CAST(Total_amount AS FLOAT)), 0) as daily_revenue
			FROM EODDt WITH (NOLOCK)
			%s
			GROUP BY EnnollDt
			ORDER BY EnnollDt ASC
		`, dtWhereSQL)

		trendRows, err := s.pool.DB.QueryContext(ctx, trendQuery, dtArgs...)
		if err == nil {
			defer trendRows.Close()
			var points []models.DailyTrendPoint
			for trendRows.Next() {
				var dt string
				var count int
				var rev float64
				if err := trendRows.Scan(&dt, &count, &rev); err == nil {
					label := dt
					isoDate := dt
					if len(dt) == 8 {
						label = fmt.Sprintf("%s/%s", dt[6:8], dt[4:6])
						isoDate = fmt.Sprintf("%s-%s-%s", dt[0:4], dt[4:6], dt[6:8])
					}
					points = append(points, models.DailyTrendPoint{
						EnrollDate:      isoDate,
						Label:           label,
						TotalEnrollment: count,
						TotalRevenue:    rev,
					})
				}
			}
			mu.Lock()
			resp.DailyTrend = points
			mu.Unlock()
		}
	}()

	// 5. Top Performing Stations / Operators from EODMth
	wg.Add(1)
	go func() {
		defer wg.Done()
		topQuery := fmt.Sprintf(`
			SELECT TOP 10
				OPERATOR_ID,
				COALESCE(MAX(OperatorName), OPERATOR_ID) as op_name,
				COALESCE(MAX(District), 'N/A') as district,
				COALESCE(SUM(TRY_CAST(Total_Enrollment AS INT)), 0) as total_enrollment,
				COALESCE(SUM(TRY_CAST(Total_amount AS FLOAT)), 0) as total_revenue
			FROM EODMth WITH (NOLOCK)
			%s
			GROUP BY OPERATOR_ID
			ORDER BY total_enrollment DESC
		`, mthWhereSQL)

		topRows, err := s.pool.DB.QueryContext(ctx, topQuery, mthArgs...)
		if err == nil {
			defer topRows.Close()
			var topStations []models.TopStationData
			for topRows.Next() {
				var sid, opName, dist string
				var enrolls int
				var rev float64
				if err := topRows.Scan(&sid, &opName, &dist, &enrolls, &rev); err == nil {
					topStations = append(topStations, models.TopStationData{
						StationID:       sid,
						StationName:     opName,
						DistrictName:    dist,
						ActiveDays:      1,
						TotalEnrollment: enrolls,
						TotalRevenue:    rev,
					})
				}
			}
			mu.Lock()
			resp.TopStations = topStations
			mu.Unlock()
		}
	}()

	wg.Wait()

	return resp, nil
}

// ----------------- DAILY LOGS ROLLUP -----------------

func (s *DataService) GetDailyLogs(ctx context.Context, month, dmName, district, stationID, enrollDT string, page, pageSize int) (*models.PaginatedDailyResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 25
	}
	offset := (page - 1) * pageSize

	var whereClauses []string
	var args []any

	if month != "" && month != "all" {
		whereClauses = append(whereClauses, "EnrollMth = @month")
		args = append(args, sql.Named("month", month))
	}
	if dmName != "" {
		whereClauses = append(whereClauses, "(DistrictManger = @dmName OR DMID = @dmName)")
		args = append(args, sql.Named("dmName", dmName))
	}
	if district != "" {
		whereClauses = append(whereClauses, "District = @district")
		args = append(args, sql.Named("district", district))
	}
	if stationID != "" {
		whereClauses = append(whereClauses, "OPERATOR_ID = @stationID")
		args = append(args, sql.Named("stationID", stationID))
	}
	cleanDT := strings.ReplaceAll(strings.TrimSpace(enrollDT), "-", "")
	if cleanDT != "" {
		whereClauses = append(whereClauses, "EnnollDt = @enrollDT")
		args = append(args, sql.Named("enrollDT", cleanDT))
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = " WHERE " + strings.Join(whereClauses, " AND ")
	}

	var wg sync.WaitGroup
	var queryErr error
	var countErr error
	var totalCount int
	items := []models.DailyRecordItem{}

	// 1. Total Count Query
	wg.Add(1)
	go func() {
		defer wg.Done()
		countSQL := fmt.Sprintf("SELECT COUNT(*) FROM EODDt WITH (NOLOCK) %s", whereSQL)
		countErr = s.pool.DB.QueryRowContext(ctx, countSQL, args...).Scan(&totalCount)
	}()

	// 2. Data Query
	wg.Add(1)
	go func() {
		defer wg.Done()
		dataSQL := fmt.Sprintf(`
			SELECT 
				EnnollDt,
				OPERATOR_ID,
				COALESCE(OperatorName, 'N/A'),
				COALESCE(District, 'N/A'),
				COALESCE(DistrictManger, 'N/A'),
				COALESCE(TRY_CAST(Total_Enrollment AS INT), 0),
				COALESCE(TRY_CAST(Total_amount AS FLOAT), 0),
				COALESCE(TRY_CAST(BMU_100 AS INT), 0),
				COALESCE(TRY_CAST(BMU_125 AS INT), 0),
				COALESCE(TRY_CAST(DMU_50 AS INT), 0),
				COALESCE(TRY_CAST(DMU_75 AS INT), 0),
				COALESCE(TRY_CAST(MBU_0 AS INT), 0),
				COALESCE(TRY_CAST(MBU_100 AS INT), 0),
				COALESCE(TRY_CAST(MBU_125 AS INT), 0),
				COALESCE(TRY_CAST(NEW_0 AS INT), 0)
			FROM EODDt WITH (NOLOCK)
			%s
			ORDER BY EnnollDt DESC, OPERATOR_ID ASC
			OFFSET @offset ROWS FETCH NEXT @pageSize ROWS ONLY
		`, whereSQL)

		argsWithPaging := append(append([]any{}, args...), sql.Named("offset", offset), sql.Named("pageSize", pageSize))
		rows, err := s.pool.DB.QueryContext(ctx, dataSQL, argsWithPaging...)
		if err != nil {
			queryErr = err
			return
		}
		defer rows.Close()

		for rows.Next() {
			var item models.DailyRecordItem
			err := rows.Scan(
				&item.EnrollDate,
				&item.StationID,
				&item.OperatorName,
				&item.District,
				&item.DistrictManager,
				&item.TotalEnrollment,
				&item.TotalAmount,
				&item.BMU100,
				&item.BMU125,
				&item.DMU50,
				&item.DMU75,
				&item.MBU0,
				&item.MBU100,
				&item.MBU125,
				&item.New0,
			)
			if err == nil {
				if len(item.EnrollDate) == 8 {
					item.EnrollDate = fmt.Sprintf("%s-%s-%s", item.EnrollDate[0:4], item.EnrollDate[4:6], item.EnrollDate[6:8])
				}
				items = append(items, item)
			}
		}
	}()

	wg.Wait()

	if queryErr != nil {
		return nil, fmt.Errorf("error querying daily logs: %w", queryErr)
	}

	if countErr != nil || totalCount == 0 {
		if len(items) == pageSize {
			totalCount = page*pageSize + 1
		} else {
			totalCount = (page-1)*pageSize + len(items)
		}
	}

	return &models.PaginatedDailyResponse{
		Items:      items,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   pageSize,
	}, nil
}

// ----------------- MONTHLY RECORDS (EODMth) -----------------

// GetMonthlyRecords queries pre-aggregated monthly operator rows directly from EODMth
func (s *DataService) GetMonthlyRecords(ctx context.Context, month, dmName, district, stationID, operatorCode string, page, pageSize int) (*models.PaginatedMonthlyResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 25
	}
	offset := (page - 1) * pageSize

	var whereClauses []string
	var args []any

	if month != "" && month != "all" {
		whereClauses = append(whereClauses, "EnrollMth = @month")
		args = append(args, sql.Named("month", month))
	}
	if dmName != "" {
		whereClauses = append(whereClauses, "(DistrictManger = @dmName OR DMID = @dmName)")
		args = append(args, sql.Named("dmName", dmName))
	}
	if district != "" {
		whereClauses = append(whereClauses, "District = @district")
		args = append(args, sql.Named("district", district))
	}
	if stationID != "" {
		whereClauses = append(whereClauses, "OPERATOR_ID = @stationID")
		args = append(args, sql.Named("stationID", stationID))
	}
	if operatorCode != "" && operatorCode != stationID {
		whereClauses = append(whereClauses, "(OPERATOR_ID = @opCode OR OperatorName LIKE @opNameLike)")
		args = append(args, sql.Named("opCode", operatorCode), sql.Named("opNameLike", "%"+operatorCode+"%"))
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = " WHERE " + strings.Join(whereClauses, " AND ")
	}

	var wg sync.WaitGroup
	var queryErr error
	var countErr error
	var totalCount int
	items := []models.MonthlyRecordItem{}

	// 1. Total Count Query
	wg.Add(1)
	go func() {
		defer wg.Done()
		countSQL := fmt.Sprintf("SELECT COUNT(*) FROM EODMth WITH (NOLOCK) %s", whereSQL)
		countErr = s.pool.DB.QueryRowContext(ctx, countSQL, args...).Scan(&totalCount)
	}()

	// 2. Data Query
	wg.Add(1)
	go func() {
		defer wg.Done()
		dataSQL := fmt.Sprintf(`
			SELECT 
				COALESCE(EnrollMth, ''),
				COALESCE(OPERATOR_ID, ''),
				COALESCE(OperatorName, 'N/A'),
				COALESCE(District, 'N/A'),
				COALESCE(DistrictManger, 'N/A'),
				COALESCE(DMID, ''),
				COALESCE(TRY_CAST(Total_Enrollment AS INT), 0),
				COALESCE(TRY_CAST(Total_amount AS FLOAT), 0),
				COALESCE(TRY_CAST(BMU_100 AS INT), 0),
				COALESCE(TRY_CAST(BMU_125 AS INT), 0),
				COALESCE(TRY_CAST(DMU_50 AS INT), 0),
				COALESCE(TRY_CAST(DMU_75 AS INT), 0),
				COALESCE(TRY_CAST(MBU_0 AS INT), 0),
				COALESCE(TRY_CAST(MBU_100 AS INT), 0),
				COALESCE(TRY_CAST(MBU_125 AS INT), 0),
				COALESCE(TRY_CAST(NEW_0 AS INT), 0)
			FROM EODMth WITH (NOLOCK)
			%s
			ORDER BY EnrollMth DESC, OPERATOR_ID ASC
			OFFSET @offset ROWS FETCH NEXT @pageSize ROWS ONLY
		`, whereSQL)

		argsWithPaging := append(append([]any{}, args...), sql.Named("offset", offset), sql.Named("pageSize", pageSize))
		rows, err := s.pool.DB.QueryContext(ctx, dataSQL, argsWithPaging...)
		if err != nil {
			queryErr = err
			return
		}
		defer rows.Close()

		for rows.Next() {
			var it models.MonthlyRecordItem
			err := rows.Scan(
				&it.EnrollMonth,
				&it.StationID,
				&it.OperatorName,
				&it.District,
				&it.DistrictManager,
				&it.DMID,
				&it.TotalEnrollment,
				&it.TotalAmount,
				&it.BMU100,
				&it.BMU125,
				&it.DMU50,
				&it.DMU75,
				&it.MBU0,
				&it.MBU100,
				&it.MBU125,
				&it.New0,
			)
			if err == nil {
				it.StationName = it.OperatorName
				it.OperatorCode = it.StationID
				items = append(items, it)
			}
		}
	}()

	wg.Wait()

	if queryErr != nil {
		return nil, fmt.Errorf("error querying monthly records: %w", queryErr)
	}

	if countErr != nil || totalCount == 0 {
		if len(items) == pageSize {
			totalCount = page*pageSize + 1
		} else {
			totalCount = (page-1)*pageSize + len(items)
		}
	}

	return &models.PaginatedMonthlyResponse{
		Items:      items,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   pageSize,
	}, nil
}

// BulkInsertRecords inserts an array of RawEODRecord rows into EODFiles
func (s *DataService) BulkInsertRecords(ctx context.Context, records []models.RawEODRecord) (*models.BulkInsertResponse, error) {
	if len(records) == 0 {
		return &models.BulkInsertResponse{
			Success:       true,
			InsertedCount: 0,
			Message:       "No records to insert",
		}, nil
	}

	tx, err := s.pool.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	insertSQL := `
		INSERT INTO EODFiles (
			SLNO, ENROLMENT_NO_DATE, APPOINTMENT_ID, TYPE, MANDATORY_BIO_METRIC_UPDATE_ONLY,
			IS_NRI, TIN_NO, OPERATOR_ID, INTRODUCER, PROOF, RESIDENT_NAME, STATUS,
			GST_AMOUNT, AMOUNT_CHARGED_FOR_NEW_ENROLMENT, AMOUNT_CHARGED_FOR_UPDATE_ENROLMENT,
			TOTAL_AMOUNT_CHARGED, PROCESSING_STATE_DESCRIPTION, REJECT_REASON_DESCRIPTION,
			PACKET_SKIPPED, FOREIGN_RESIDENT, StationID, EnrollDT, EnrollMth, UpdateType,
			DistrictManger, District, OperatorName, StationType
		) VALUES (
			@p1, @p2, @p3, @p4, @p5, @p6, @p7, @p8, @p9, @p10, @p11, @p12,
			@p13, @p14, @p15, @p16, @p17, @p18, @p19, @p20, @p21, @p22, @p23,
			@p24, @p25, @p26, @p27, @p28
		)
	`
	stmt, err := tx.PrepareContext(ctx, insertSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer stmt.Close()

	insertedCount := 0
	for _, r := range records {
		_, err := stmt.ExecContext(ctx,
			r.SLNO,
			r.EnrolmentNoDate,
			r.AppointmentID,
			r.Type,
			r.MandatoryBioMetricUpdateOnly,
			r.IsNRI,
			r.TinNo,
			r.OperatorID,
			r.Introducer,
			r.Proof,
			r.ResidentName,
			r.Status,
			r.GstAmount,
			r.AmountChargedForNewEnrolment,
			r.AmountChargedForUpdateEnrolment,
			r.TotalAmountCharged,
			r.ProcessingStateDescription,
			r.RejectReasonDescription,
			r.PacketSkipped,
			r.ForeignResident,
			r.StationID,
			r.EnrollDT,
			r.EnrollMth,
			r.UpdateType,
			r.DistrictManger,
			r.District,
			r.OperatorName,
			r.StationType,
		)
		if err != nil {
			return nil, fmt.Errorf("failed inserting record %s: %w", r.EnrolmentNoDate, err)
		}
		insertedCount++
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Invalidate filter cache so newly inserted month appears immediately
	globalFilterCache.mu.Lock()
	globalFilterCache.entries = make(map[string]filterCacheEntry)
	globalFilterCache.mu.Unlock()

	return &models.BulkInsertResponse{
		Success:       true,
		InsertedCount: insertedCount,
		Message:       fmt.Sprintf("Successfully inserted %d records into database", insertedCount),
	}, nil
}

// AssignStations updates DistrictManger and DMID in EODMth and EODDt for the specified station IDs
func (s *DataService) AssignStations(ctx context.Context, dmIdentifier string, stationIDs []string) (*models.AssignStationsResponse, error) {
	dmIdentifier = strings.TrimSpace(dmIdentifier)
	if dmIdentifier == "" {
		return nil, fmt.Errorf("dm_name cannot be empty")
	}

	// 1. Resolve canonical DM Name and DMID from DMInfo
	var resolvedName, resolvedID string
	err := s.pool.DB.QueryRowContext(ctx, `
		SELECT TOP 1 DM_ID, District_Manager 
		FROM DMInfo WITH (NOLOCK) 
		WHERE DM_ID = @id OR District_Manager = @name
	`, sql.Named("id", dmIdentifier), sql.Named("name", dmIdentifier)).Scan(&resolvedID, &resolvedName)
	if err != nil {
		resolvedName = dmIdentifier
		resolvedID = dmIdentifier
	}

	tx, err := s.pool.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 2. Unassign previous stations for this DM in EODMth and EODDt
	unassignMthSQL := `UPDATE EODMth SET DistrictManger = NULL, DMID = NULL WHERE DistrictManger = @dmName OR DMID = @dmID`
	_, _ = tx.ExecContext(ctx, unassignMthSQL, sql.Named("dmName", resolvedName), sql.Named("dmID", resolvedID))

	unassignDtSQL := `UPDATE EODDt SET DistrictManger = NULL, DMID = NULL WHERE DistrictManger = @dmName OR DMID = @dmID`
	_, _ = tx.ExecContext(ctx, unassignDtSQL, sql.Named("dmName", resolvedName), sql.Named("dmID", resolvedID))

	// 3. Assign the new list of stations to this DM in EODMth and EODDt
	assignedCount := 0
	if len(stationIDs) > 0 {
		stmtMth, err := tx.PrepareContext(ctx, `UPDATE EODMth SET DistrictManger = @dmName, DMID = @dmID WHERE OPERATOR_ID = @stnID`)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare EODMth assignment statement: %w", err)
		}
		defer stmtMth.Close()

		stmtDt, err := tx.PrepareContext(ctx, `UPDATE EODDt SET DistrictManger = @dmName, DMID = @dmID WHERE OPERATOR_ID = @stnID`)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare EODDt assignment statement: %w", err)
		}
		defer stmtDt.Close()

		for _, sid := range stationIDs {
			sidTrimmed := strings.TrimSpace(sid)
			if sidTrimmed == "" {
				continue
			}
			_, _ = stmtMth.ExecContext(ctx, sql.Named("dmName", resolvedName), sql.Named("dmID", resolvedID), sql.Named("stnID", sidTrimmed))
			resDt, err := stmtDt.ExecContext(ctx, sql.Named("dmName", resolvedName), sql.Named("dmID", resolvedID), sql.Named("stnID", sidTrimmed))
			if err != nil {
				return nil, fmt.Errorf("failed assigning station %s: %w", sidTrimmed, err)
			}
			if rowsAff, _ := resDt.RowsAffected(); rowsAff > 0 {
				assignedCount += int(rowsAff)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Invalidate filter cache so dropdowns and overviews immediately reflect the new assignments
	globalFilterCache.mu.Lock()
	globalFilterCache.entries = make(map[string]filterCacheEntry)
	globalFilterCache.mu.Unlock()

	return &models.AssignStationsResponse{
		Success:      true,
		UpdatedCount: len(stationIDs),
		Message:      fmt.Sprintf("Successfully assigned %d stations to %s (%s)", len(stationIDs), resolvedName, resolvedID),
	}, nil
}

// GetStationsByDM returns all stations currently assigned to a given DM in EODMth
func (s *DataService) GetStationsByDM(ctx context.Context, dmName string) ([]models.DMStationItem, error) {
	dmName = strings.TrimSpace(dmName)
	if dmName == "" {
		return []models.DMStationItem{}, nil
	}

	query := `
		SELECT 
			OPERATOR_ID, 
			COALESCE(MAX(District), 'N/A') as district,
			'Standard' as station_type
		FROM EODMth WITH (NOLOCK)
		WHERE (DistrictManger = @dmName OR DMID = @dmName)
		GROUP BY OPERATOR_ID
		ORDER BY OPERATOR_ID
	`
	rows, err := s.pool.DB.QueryContext(ctx, query, sql.Named("dmName", dmName))
	if err != nil {
		return nil, fmt.Errorf("failed querying stations for DM: %w", err)
	}
	defer rows.Close()

	items := []models.DMStationItem{}
	for rows.Next() {
		var item models.DMStationItem
		if err := rows.Scan(&item.StationID, &item.DistrictName, &item.StationType); err == nil {
			items = append(items, item)
		}
	}
	return items, nil
}

