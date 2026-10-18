package service

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
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

// InvalidateFilterCache clears all cached filter options to force a fresh query against Azure SQL
func (s *DataService) InvalidateFilterCache() {
	globalFilterCache.mu.Lock()
	globalFilterCache.entries = make(map[string]filterCacheEntry)
	globalFilterCache.mu.Unlock()
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
	if pageSize < 1 {
		pageSize = 25
	}
	if pageSize > 50000 {
		pageSize = 50000
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
	if pageSize < 1 {
		pageSize = 25
	}
	if pageSize > 50000 {
		pageSize = 50000
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

// BulkInsertRecords aggregates uploaded raw records and upserts them into EODDt and EODMth
func (s *DataService) BulkInsertRecords(ctx context.Context, records []models.RawEODRecord) (*models.BulkInsertResponse, error) {
	if len(records) == 0 {
		return &models.BulkInsertResponse{
			Success:       true,
			InsertedCount: 0,
			Message:       "No records to insert",
		}, nil
	}

	// 1. Fetch DM map (District_Manager -> DM_ID) from DMInfo for accurate DMID resolution
	dmMap := make(map[string]string)
	dmRows, err := s.pool.DB.QueryContext(ctx, "SELECT District_Manager, DM_ID FROM DMInfo WITH (NOLOCK) WHERE District_Manager IS NOT NULL")
	if err == nil {
		defer dmRows.Close()
		for dmRows.Next() {
			var name, id string
			if err := dmRows.Scan(&name, &id); err == nil {
				dmMap[strings.ToLower(strings.TrimSpace(name))] = id
			}
		}
	}

	type aggItem struct {
		EnrollMth       string
		EnnollDt        string
		District        string
		DistrictManager string
		OperatorID      string
		OperatorName    string
		DMID            string
		TotalEnrollment int
		TotalAmount     float64
		BMU100          int
		BMU125          int
		MBU0            int
		MBU100          int
		MBU125          int
		DMU50           int
		DMU75           int
		New0            int
	}

	dailyMap := make(map[string]*aggItem)
	monthlyMap := make(map[string]*aggItem)

	for _, r := range records {
		enrollDT := strings.ReplaceAll(strings.TrimSpace(r.EnrollDT), "-", "")
		if len(enrollDT) < 8 {
			cand := strings.ReplaceAll(strings.TrimSpace(r.EnrolmentNoDate), "-", "")
			if len(cand) >= 8 {
				enrollDT = cand[:8]
			}
		}

		enrollMth := strings.TrimSpace(r.EnrollMth)
		if enrollMth == "" && len(enrollDT) >= 6 {
			enrollMth = enrollDT[:6]
		}

		opID := strings.TrimSpace(r.OperatorID)
		if opID == "" {
			opID = strings.TrimSpace(r.StationID)
		}
		if opID == "" {
			continue
		}

		opName := strings.TrimSpace(r.OperatorName)
		if opName == "" {
			opName = opID
		}

		dist := strings.TrimSpace(r.District)
		if dist == "" {
			dist = "Unknown"
		}

		dm := strings.TrimSpace(r.DistrictManger)
		dmid := ""
		if dm != "" {
			if id, ok := dmMap[strings.ToLower(dm)]; ok {
				dmid = id
			}
		}

		var amount float64
		if r.TotalAmountCharged != "" {
			if a, err := strconv.ParseFloat(strings.TrimSpace(r.TotalAmountCharged), 64); err == nil {
				amount = a
			}
		}

		uType := strings.ToUpper(strings.TrimSpace(r.UpdateType))
		rType := strings.ToUpper(strings.TrimSpace(r.Type))

		var bmu100, bmu125, dmu50, dmu75, mbu0, mbu100, mbu125, new0 int
		if strings.Contains(uType, "BMU") && strings.Contains(uType, "100") {
			bmu100 = 1
		} else if strings.Contains(uType, "BMU") && strings.Contains(uType, "125") {
			bmu125 = 1
		} else if strings.Contains(uType, "DMU") && strings.Contains(uType, "50") {
			dmu50 = 1
		} else if strings.Contains(uType, "DMU") && strings.Contains(uType, "75") {
			dmu75 = 1
		} else if strings.Contains(uType, "MBU") && strings.Contains(uType, "0") && !strings.Contains(uType, "100") && !strings.Contains(uType, "125") {
			mbu0 = 1
		} else if strings.Contains(uType, "MBU") && strings.Contains(uType, "100") {
			mbu100 = 1
		} else if strings.Contains(uType, "MBU") && strings.Contains(uType, "125") {
			mbu125 = 1
		} else if rType == "N" || strings.Contains(uType, "NEW") {
			new0 = 1
		} else {
			dmu50 = 1
		}

		// Aggregate into Daily
		dailyKey := enrollDT + "|" + opID
		if item, exists := dailyMap[dailyKey]; exists {
			item.TotalEnrollment++
			item.TotalAmount += amount
			item.BMU100 += bmu100
			item.BMU125 += bmu125
			item.DMU50 += dmu50
			item.DMU75 += dmu75
			item.MBU0 += mbu0
			item.MBU100 += mbu100
			item.MBU125 += mbu125
			item.New0 += new0
			if item.District == "Unknown" && dist != "Unknown" {
				item.District = dist
			}
			if item.DistrictManager == "" && dm != "" {
				item.DistrictManager = dm
				item.DMID = dmid
			}
		} else {
			dailyMap[dailyKey] = &aggItem{
				EnrollMth:       enrollMth,
				EnnollDt:        enrollDT,
				District:        dist,
				DistrictManager: dm,
				OperatorID:      opID,
				OperatorName:    opName,
				DMID:            dmid,
				TotalEnrollment: 1,
				TotalAmount:     amount,
				BMU100:          bmu100,
				BMU125:          bmu125,
				MBU0:            mbu0,
				MBU100:          mbu100,
				MBU125:          mbu125,
				DMU50:           dmu50,
				DMU75:           dmu75,
				New0:            new0,
			}
		}

		// Aggregate into Monthly
		monthlyKey := enrollMth + "|" + opID
		if item, exists := monthlyMap[monthlyKey]; exists {
			item.TotalEnrollment++
			item.TotalAmount += amount
			item.BMU100 += bmu100
			item.BMU125 += bmu125
			item.DMU50 += dmu50
			item.DMU75 += dmu75
			item.MBU0 += mbu0
			item.MBU100 += mbu100
			item.MBU125 += mbu125
			item.New0 += new0
			if item.District == "Unknown" && dist != "Unknown" {
				item.District = dist
			}
			if item.DistrictManager == "" && dm != "" {
				item.DistrictManager = dm
				item.DMID = dmid
			}
		} else {
			monthlyMap[monthlyKey] = &aggItem{
				EnrollMth:       enrollMth,
				District:        dist,
				DistrictManager: dm,
				OperatorID:      opID,
				OperatorName:    opName,
				DMID:            dmid,
				TotalEnrollment: 1,
				TotalAmount:     amount,
				BMU100:          bmu100,
				BMU125:          bmu125,
				MBU0:            mbu0,
				MBU100:          mbu100,
				MBU125:          mbu125,
				DMU50:           dmu50,
				DMU75:           dmu75,
				New0:            new0,
			}
		}
	}

	tx, err := s.pool.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 2. Upsert Daily Aggregates into EODDt
	mergeDtSQL := `
		MERGE EODDt WITH (HOLDLOCK) AS target
		USING (SELECT 
			@EnrollMth AS EnrollMth, 
			@EnnollDt AS EnnollDt, 
			@District AS District, 
			@DistrictManger AS DistrictManger, 
			@OPERATOR_ID AS OPERATOR_ID, 
			@OperatorName AS OperatorName, 
			@DMID AS DMID, 
			@Enrollment AS Total_Enrollment, 
			@Amount AS Total_amount, 
			@BMU100 AS BMU_100, 
			@BMU125 AS BMU_125, 
			@MBU0 AS MBU_0, 
			@MBU100 AS MBU_100, 
			@MBU125 AS MBU_125, 
			@DMU50 AS DMU_50, 
			@DMU75 AS DMU_75, 
			@NEW0 AS NEW_0
		) AS src
		ON (target.EnnollDt = src.EnnollDt AND target.OPERATOR_ID = src.OPERATOR_ID)
		WHEN MATCHED THEN
			UPDATE SET 
				target.Total_Enrollment = CAST((COALESCE(TRY_CAST(target.Total_Enrollment AS INT), 0) + src.Total_Enrollment) AS NVARCHAR(12)),
				target.Total_amount = CAST((COALESCE(TRY_CAST(target.Total_amount AS FLOAT), 0) + src.Total_amount) AS NVARCHAR(12)),
				target.BMU_100 = CAST((COALESCE(TRY_CAST(target.BMU_100 AS INT), 0) + src.BMU_100) AS NVARCHAR(10)),
				target.BMU_125 = CAST((COALESCE(TRY_CAST(target.BMU_125 AS INT), 0) + src.BMU_125) AS NVARCHAR(10)),
				target.MBU_0 = CAST((COALESCE(TRY_CAST(target.MBU_0 AS INT), 0) + src.MBU_0) AS NVARCHAR(10)),
				target.MBU_100 = CAST((COALESCE(TRY_CAST(target.MBU_100 AS INT), 0) + src.MBU_100) AS NVARCHAR(10)),
				target.MBU_125 = CAST((COALESCE(TRY_CAST(target.MBU_125 AS INT), 0) + src.MBU_125) AS NVARCHAR(10)),
				target.DMU_50 = CAST((COALESCE(TRY_CAST(target.DMU_50 AS INT), 0) + src.DMU_50) AS NVARCHAR(10)),
				target.DMU_75 = CAST((COALESCE(TRY_CAST(target.DMU_75 AS INT), 0) + src.DMU_75) AS NVARCHAR(10)),
				target.NEW_0 = CAST((COALESCE(TRY_CAST(target.NEW_0 AS INT), 0) + src.NEW_0) AS NVARCHAR(10)),
				target.District = COALESCE(NULLIF(src.District, ''), target.District),
				target.DistrictManger = COALESCE(NULLIF(src.DistrictManger, ''), target.DistrictManger),
				target.OperatorName = COALESCE(NULLIF(src.OperatorName, ''), target.OperatorName),
				target.DMID = COALESCE(NULLIF(src.DMID, ''), target.DMID)
		WHEN NOT MATCHED THEN
			INSERT (EnrollMth, EnnollDt, District, DistrictManger, OPERATOR_ID, OperatorName, Total_Enrollment, Total_amount, BMU_100, BMU_125, MBU_0, MBU_100, MBU_125, DMU_50, DMU_75, NEW_0, DMID)
			VALUES (src.EnrollMth, src.EnnollDt, src.District, src.DistrictManger, src.OPERATOR_ID, src.OperatorName, CAST(src.Total_Enrollment AS NVARCHAR(12)), CAST(src.Total_amount AS NVARCHAR(12)), CAST(src.BMU_100 AS NVARCHAR(10)), CAST(src.BMU_125 AS NVARCHAR(10)), CAST(src.MBU_0 AS NVARCHAR(10)), CAST(src.MBU_100 AS NVARCHAR(10)), CAST(src.MBU_125 AS NVARCHAR(10)), CAST(src.DMU_50 AS NVARCHAR(10)), CAST(src.DMU_75 AS NVARCHAR(10)), CAST(src.NEW_0 AS NVARCHAR(10)), src.DMID);
	`
	dtStmt, err := tx.PrepareContext(ctx, mergeDtSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare EODDt merge statement: %w", err)
	}
	defer dtStmt.Close()

	for _, it := range dailyMap {
		_, err := dtStmt.ExecContext(ctx,
			sql.Named("EnrollMth", it.EnrollMth),
			sql.Named("EnnollDt", it.EnnollDt),
			sql.Named("District", it.District),
			sql.Named("DistrictManger", it.DistrictManager),
			sql.Named("OPERATOR_ID", it.OperatorID),
			sql.Named("OperatorName", it.OperatorName),
			sql.Named("DMID", it.DMID),
			sql.Named("Enrollment", it.TotalEnrollment),
			sql.Named("Amount", it.TotalAmount),
			sql.Named("BMU100", it.BMU100),
			sql.Named("BMU125", it.BMU125),
			sql.Named("MBU0", it.MBU0),
			sql.Named("MBU100", it.MBU100),
			sql.Named("MBU125", it.MBU125),
			sql.Named("DMU50", it.DMU50),
			sql.Named("DMU75", it.DMU75),
			sql.Named("NEW0", it.New0),
		)
		if err != nil {
			return nil, fmt.Errorf("failed merging into EODDt for %s/%s: %w", it.EnnollDt, it.OperatorID, err)
		}
	}

	// 3. Upsert Monthly Aggregates into EODMth
	mergeMthSQL := `
		MERGE EODMth WITH (HOLDLOCK) AS target
		USING (SELECT 
			@EnrollMth AS EnrollMth, 
			@District AS District, 
			@DistrictManger AS DistrictManger, 
			@OPERATOR_ID AS OPERATOR_ID, 
			@OperatorName AS OperatorName, 
			@DMID AS DMID, 
			@Enrollment AS Total_Enrollment, 
			@Amount AS Total_amount, 
			@BMU100 AS BMU_100, 
			@BMU125 AS BMU_125, 
			@MBU0 AS MBU_0, 
			@MBU100 AS MBU_100, 
			@MBU125 AS MBU_125, 
			@DMU50 AS DMU_50, 
			@DMU75 AS DMU_75, 
			@NEW0 AS NEW_0
		) AS src
		ON (target.EnrollMth = src.EnrollMth AND target.OPERATOR_ID = src.OPERATOR_ID)
		WHEN MATCHED THEN
			UPDATE SET 
				target.Total_Enrollment = CAST((COALESCE(TRY_CAST(target.Total_Enrollment AS INT), 0) + src.Total_Enrollment) AS NVARCHAR(10)),
				target.Total_amount = CAST((COALESCE(TRY_CAST(target.Total_amount AS FLOAT), 0) + src.Total_amount) AS NVARCHAR(10)),
				target.BMU_100 = CAST((COALESCE(TRY_CAST(target.BMU_100 AS INT), 0) + src.BMU_100) AS NVARCHAR(10)),
				target.BMU_125 = CAST((COALESCE(TRY_CAST(target.BMU_125 AS INT), 0) + src.BMU_125) AS NVARCHAR(10)),
				target.MBU_0 = CAST((COALESCE(TRY_CAST(target.MBU_0 AS INT), 0) + src.MBU_0) AS NVARCHAR(10)),
				target.MBU_100 = CAST((COALESCE(TRY_CAST(target.MBU_100 AS INT), 0) + src.MBU_100) AS NVARCHAR(10)),
				target.MBU_125 = CAST((COALESCE(TRY_CAST(target.MBU_125 AS INT), 0) + src.MBU_125) AS NVARCHAR(10)),
				target.DMU_50 = CAST((COALESCE(TRY_CAST(target.DMU_50 AS INT), 0) + src.DMU_50) AS NVARCHAR(10)),
				target.DMU_75 = CAST((COALESCE(TRY_CAST(target.DMU_75 AS INT), 0) + src.DMU_75) AS NVARCHAR(10)),
				target.NEW_0 = CAST((COALESCE(TRY_CAST(target.NEW_0 AS INT), 0) + src.NEW_0) AS NVARCHAR(10)),
				target.District = COALESCE(NULLIF(src.District, ''), target.District),
				target.DistrictManger = COALESCE(NULLIF(src.DistrictManger, ''), target.DistrictManger),
				target.OperatorName = COALESCE(NULLIF(src.OperatorName, ''), target.OperatorName),
				target.DMID = COALESCE(NULLIF(src.DMID, ''), target.DMID)
		WHEN NOT MATCHED THEN
			INSERT (EnrollMth, District, DistrictManger, OPERATOR_ID, OperatorName, Total_Enrollment, Total_amount, BMU_100, BMU_125, MBU_0, MBU_100, MBU_125, DMU_50, DMU_75, NEW_0, DMID)
			VALUES (src.EnrollMth, src.District, src.DistrictManger, src.OPERATOR_ID, src.OperatorName, CAST(src.Total_Enrollment AS NVARCHAR(10)), CAST(src.Total_amount AS NVARCHAR(10)), CAST(src.BMU_100 AS NVARCHAR(10)), CAST(src.BMU_125 AS NVARCHAR(10)), CAST(src.MBU_0 AS NVARCHAR(10)), CAST(src.MBU_100 AS NVARCHAR(10)), CAST(src.MBU_125 AS NVARCHAR(10)), CAST(src.DMU_50 AS NVARCHAR(10)), CAST(src.DMU_75 AS NVARCHAR(10)), CAST(src.NEW_0 AS NVARCHAR(10)), src.DMID);
	`
	mthStmt, err := tx.PrepareContext(ctx, mergeMthSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare EODMth merge statement: %w", err)
	}
	defer mthStmt.Close()

	for _, it := range monthlyMap {
		_, err := mthStmt.ExecContext(ctx,
			sql.Named("EnrollMth", it.EnrollMth),
			sql.Named("District", it.District),
			sql.Named("DistrictManger", it.DistrictManager),
			sql.Named("OPERATOR_ID", it.OperatorID),
			sql.Named("OperatorName", it.OperatorName),
			sql.Named("DMID", it.DMID),
			sql.Named("Enrollment", it.TotalEnrollment),
			sql.Named("Amount", it.TotalAmount),
			sql.Named("BMU100", it.BMU100),
			sql.Named("BMU125", it.BMU125),
			sql.Named("MBU0", it.MBU0),
			sql.Named("MBU100", it.MBU100),
			sql.Named("MBU125", it.MBU125),
			sql.Named("DMU50", it.DMU50),
			sql.Named("DMU75", it.DMU75),
			sql.Named("NEW0", it.New0),
		)
		if err != nil {
			return nil, fmt.Errorf("failed merging into EODMth for %s/%s: %w", it.EnrollMth, it.OperatorID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Invalidate filter cache so dropdowns and statistics immediately reflect newly inserted month/data
	s.InvalidateFilterCache()

	return &models.BulkInsertResponse{
		Success:       true,
		InsertedCount: len(records),
		Message:       fmt.Sprintf("Successfully processed %d records into EODDt and rolled up into EODMth", len(records)),
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
	s.InvalidateFilterCache()

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

// UpdatePassword updates the password in DMInfo for a DM or Admin
func (s *DataService) UpdatePassword(ctx context.Context, userID, newPassword string) (*models.UpdatePasswordResponse, error) {
	userID = strings.TrimSpace(userID)
	newPassword = strings.TrimSpace(newPassword)
	if userID == "" || newPassword == "" {
		return nil, fmt.Errorf("user_id and new_password are required")
	}

	// Expand Password column size to NVARCHAR(100) if not already done, ignoring if no DDL permissions
	_, _ = s.pool.DB.ExecContext(ctx, "ALTER TABLE DMInfo ALTER COLUMN Password NVARCHAR(100)")

	query := `UPDATE DMInfo SET Password = @newPassword WHERE DM_ID = @id OR District_Manager = @id`
	res, err := s.pool.DB.ExecContext(ctx, query, sql.Named("newPassword", newPassword), sql.Named("id", userID))
	if err != nil {
		return nil, fmt.Errorf("failed to update password in DMInfo: %w", err)
	}

	rowsAff, _ := res.RowsAffected()
	if rowsAff == 0 {
		return nil, fmt.Errorf("user '%s' not found in DMInfo", userID)
	}

	return &models.UpdatePasswordResponse{
		Success: true,
		Message: fmt.Sprintf("Password successfully updated in DMInfo for %s", userID),
	}, nil
}


