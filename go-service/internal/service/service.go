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
	query := `
		SELECT Id, DistrictManager, Role 
		FROM EODDB_Users 
		WHERE Id = @id AND Password = @password
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
	query := `
		SELECT Id, DistrictManager, Role 
		FROM EODDB_Users 
		WHERE Id = @id
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
	rows, err := s.pool.DB.QueryContext(ctx, "SELECT Id, DistrictManager, Role FROM EODDB_Users ORDER BY Id")
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

	// ------ 1. Months (EODFiles + static 202607) ------
	wg.Add(1)
	go func() {
		defer wg.Done()
		monthQuery := `SELECT DISTINCT EnrollMth FROM EODFiles WITH (NOLOCK) WHERE EnrollMth IS NOT NULL AND LTRIM(RTRIM(EnrollMth)) <> ''`
		foundMonths := map[string]bool{"202607": true}
		mRows, err := s.pool.DB.QueryContext(ctx, monthQuery)
		if err == nil {
			defer mRows.Close()
			for mRows.Next() {
				var m string
				if err := mRows.Scan(&m); err == nil && m != "" {
					foundMonths[m] = true
				}
			}
		}
		months := make([]string, 0, len(foundMonths))
		for m := range foundMonths {
			months = append(months, m)
		}
		// Sort descending
		for i := 0; i < len(months)-1; i++ {
			for j := i + 1; j < len(months); j++ {
				if months[i] < months[j] {
					months[i], months[j] = months[j], months[i]
				}
			}
		}
		mu.Lock()
		opts.Months = months
		mu.Unlock()
	}()

	// ------ 2. Districts ------
	wg.Add(1)
	go func() {
		defer wg.Done()
		distQuery := "SELECT DISTINCT District FROM WorkingKIT WITH (NOLOCK) WHERE District IS NOT NULL AND LTRIM(RTRIM(District)) <> ''"
		var distArgs []any
		if dmName != "" {
			distQuery += " AND DistrictManger = @dmName"
			distArgs = append(distArgs, sql.Named("dmName", dmName))
		}
		distQuery += " ORDER BY District"
		dRows, err := s.pool.DB.QueryContext(ctx, distQuery, distArgs...)
		if err == nil {
			defer dRows.Close()
			var dists []string
			for dRows.Next() {
				var d string
				if err := dRows.Scan(&d); err == nil {
					dists = append(dists, d)
				}
			}
			mu.Lock()
			opts.Districts = dists
			mu.Unlock()
		}
	}()

	// ------ 3. DMs ------
	wg.Add(1)
	go func() {
		defer wg.Done()
		dmQuery := `
			SELECT DISTINCT
				COALESCE(u.Id, d.DistrictManger) as id,
				d.DistrictManger as name
			FROM (
				SELECT DistrictManger FROM WorkingKIT WITH (NOLOCK) WHERE DistrictManger IS NOT NULL AND LTRIM(RTRIM(DistrictManger)) <> ''
				UNION
				SELECT DistrictManager FROM EODDB_Users WITH (NOLOCK) WHERE DistrictManager IS NOT NULL AND LTRIM(RTRIM(DistrictManager)) <> ''
			) d
			LEFT JOIN EODDB_Users u WITH (NOLOCK) ON d.DistrictManger = u.DistrictManager
			ORDER BY name
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

	// ------ 4. Stations ------
	wg.Add(1)
	go func() {
		defer wg.Done()
		stnQuery := "SELECT DISTINCT StationID FROM WorkingKIT WITH (NOLOCK) WHERE StationID IS NOT NULL AND LTRIM(RTRIM(StationID)) <> ''"
		var stnArgs []any
		if dmName != "" {
			stnQuery += " AND DistrictManger = @dmName"
			stnArgs = append(stnArgs, sql.Named("dmName", dmName))
		}
		stnQuery += " ORDER BY StationID"
		stnRows, err := s.pool.DB.QueryContext(ctx, stnQuery, stnArgs...)
		if err == nil {
			defer stnRows.Close()
			var stns []string
			for stnRows.Next() {
				var sid string
				if err := stnRows.Scan(&sid); err == nil {
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
	table := s.TableForMonth(month)

	var whereClauses []string
	var args []any

	if month != "" && month != "all" {
		whereClauses = append(whereClauses, "EnrollMth = @month")
		args = append(args, sql.Named("month", month))
	}
	if dmName != "" {
		whereClauses = append(whereClauses, "DistrictManger = @dmName")
		args = append(args, sql.Named("dmName", dmName))
	}
	if district != "" {
		whereClauses = append(whereClauses, "District = @district")
		args = append(args, sql.Named("district", district))
	}
	if stationID != "" {
		whereClauses = append(whereClauses, "StationID = @stationID")
		args = append(args, sql.Named("stationID", stationID))
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = " WHERE " + strings.Join(whereClauses, " AND ")
	}

	resp := &models.DMOverviewResponse{
		DailyTrend:  []models.DailyTrendPoint{},
		TopStations: []models.TopStationData{},
		CategoryMix: []models.CategoryMixItem{},
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	// 1. KPI Aggregation
	wg.Add(1)
	go func() {
		defer wg.Done()
		kpiQuery := fmt.Sprintf(`
			SELECT 
				COUNT(*) as total_enrollment,
				COALESCE(SUM(TRY_CAST(TOTAL_AMOUNT_CHARGED AS decimal(10,2))), 0) as total_revenue,
				COUNT(DISTINCT EnrollDT) as active_days
			FROM [%s] WITH (NOLOCK)
			%s
		`, table, whereSQL)

		var totalEnrollment, activeDays int
		var totalRev float64
		err := s.pool.DB.QueryRowContext(ctx, kpiQuery, args...).Scan(
			&totalEnrollment,
			&totalRev,
			&activeDays,
		)
		if err == nil {
			mu.Lock()
			resp.TotalEnrollment = totalEnrollment
			resp.TotalRevenue = totalRev
			resp.ActiveDaysCount = activeDays
			mu.Unlock()
		}
	}()

	// 2. Daily Trend
	wg.Add(1)
	go func() {
		defer wg.Done()
		trendQuery := fmt.Sprintf(`
			SELECT 
				EnrollDT,
				COUNT(*) as daily_enrollment,
				COALESCE(SUM(TRY_CAST(TOTAL_AMOUNT_CHARGED AS decimal(10,2))), 0) as daily_revenue
			FROM [%s] WITH (NOLOCK)
			%s
			GROUP BY EnrollDT
			ORDER BY EnrollDT ASC
		`, table, whereSQL)

		trendRows, err := s.pool.DB.QueryContext(ctx, trendQuery, args...)
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
						label = fmt.Sprintf("%s/%s", dt[6:8], dt[4:6]) // e.g. 03/07
						isoDate = fmt.Sprintf("%s-%s-%s", dt[0:4], dt[4:6], dt[6:8]) // e.g. 2026-07-03
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

	// 3. Category Mix
	wg.Add(1)
	go func() {
		defer wg.Done()
		catQuery := fmt.Sprintf(`
			SELECT 
				COALESCE(UpdateType, 'Other') as cat,
				COUNT(*) as count
			FROM [%s] WITH (NOLOCK)
			%s
			GROUP BY UpdateType
			ORDER BY count DESC
		`, table, whereSQL)

		catRows, err := s.pool.DB.QueryContext(ctx, catQuery, args...)
		if err == nil {
			defer catRows.Close()
			var items []models.CategoryMixItem
			for catRows.Next() {
				var cat string
				var count int
				if err := catRows.Scan(&cat, &count); err == nil {
					items = append(items, models.CategoryMixItem{
						Category: cat,
						Count:    count,
					})
				}
			}
			mu.Lock()
			resp.CategoryMix = items
			mu.Unlock()
		}
	}()

	// 4. Top Performing Stations
	wg.Add(1)
	go func() {
		defer wg.Done()
		topQuery := fmt.Sprintf(`
			SELECT TOP 10
				StationID,
				COALESCE(MAX(District), 'N/A') as district,
				COUNT(DISTINCT EnrollDT) as active_days,
				COUNT(*) as total_enrollment,
				COALESCE(SUM(TRY_CAST(TOTAL_AMOUNT_CHARGED AS decimal(10,2))), 0) as total_revenue
			FROM [%s] WITH (NOLOCK)
			%s
			GROUP BY StationID
			ORDER BY total_enrollment DESC
		`, table, whereSQL)

		topRows, err := s.pool.DB.QueryContext(ctx, topQuery, args...)
		if err == nil {
			defer topRows.Close()
			var topStations []models.TopStationData
			for topRows.Next() {
				var sid, dist string
				var days, enrolls int
				var rev float64
				if err := topRows.Scan(&sid, &dist, &days, &enrolls, &rev); err == nil {
					topStations = append(topStations, models.TopStationData{
						StationID:       sid,
						StationName:     "Station " + sid,
						DistrictName:    dist,
						ActiveDays:      days,
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
	table := s.TableForMonth(month)

	var whereClauses []string
	var args []any

	if month != "" && month != "all" {
		whereClauses = append(whereClauses, "EnrollMth = @month")
		args = append(args, sql.Named("month", month))
	}
	if dmName != "" {
		whereClauses = append(whereClauses, "DistrictManger = @dmName")
		args = append(args, sql.Named("dmName", dmName))
	}
	if district != "" {
		whereClauses = append(whereClauses, "District = @district")
		args = append(args, sql.Named("district", district))
	}
	if stationID != "" {
		whereClauses = append(whereClauses, "StationID = @stationID")
		args = append(args, sql.Named("stationID", stationID))
	}
	if enrollDT != "" {
		whereClauses = append(whereClauses, "EnrollDT = @enrollDT")
		args = append(args, sql.Named("enrollDT", enrollDT))
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = " WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Total groups count
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*) FROM (
			SELECT StationID, EnrollDT, OperatorName
			FROM [%s]
			%s
			GROUP BY StationID, EnrollDT, OperatorName
		) as sub
	`, table, whereSQL)

	var totalCount int
	_ = s.pool.DB.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount)

	// Rollup query
	rollupQuery := fmt.Sprintf(`
		SELECT 
			EnrollDT,
			StationID,
			COALESCE(OperatorName, 'N/A'),
			COALESCE(MAX(District), 'N/A'),
			COALESCE(MAX(DistrictManger), 'N/A'),
			COUNT(*) as total_enrollment,
			COALESCE(SUM(TRY_CAST(TOTAL_AMOUNT_CHARGED AS decimal(10,2))), 0) as total_amount,
			SUM(CASE WHEN UpdateType LIKE 'BMU%%100%%' THEN 1 ELSE 0 END) as bmu_100,
			SUM(CASE WHEN UpdateType LIKE 'BMU%%125%%' THEN 1 ELSE 0 END) as bmu_125,
			SUM(CASE WHEN UpdateType LIKE 'DMU%%50%%' THEN 1 ELSE 0 END) as dmu_50,
			SUM(CASE WHEN UpdateType LIKE 'DMU%%75%%' THEN 1 ELSE 0 END) as dmu_75,
			SUM(CASE WHEN UpdateType LIKE 'MBU%%0%%' THEN 1 ELSE 0 END) as mbu_0,
			SUM(CASE WHEN UpdateType LIKE 'MBU%%100%%' THEN 1 ELSE 0 END) as mbu_100,
			SUM(CASE WHEN UpdateType LIKE 'MBU%%125%%' THEN 1 ELSE 0 END) as mbu_125,
			SUM(CASE WHEN TYPE = 'N' OR UpdateType LIKE 'NEW%%' THEN 1 ELSE 0 END) as new_0
		FROM [%s]
		%s
		GROUP BY EnrollDT, StationID, OperatorName
		ORDER BY EnrollDT DESC, total_enrollment DESC
		OFFSET @offset ROWS FETCH NEXT @pageSize ROWS ONLY
	`, table, whereSQL)

	argsWithPaging := append(args, sql.Named("offset", offset), sql.Named("pageSize", pageSize))
	rows, err := s.pool.DB.QueryContext(ctx, rollupQuery, argsWithPaging...)
	if err != nil {
		return nil, fmt.Errorf("error querying daily logs: %w", err)
	}
	defer rows.Close()

	items := []models.DailyRecordItem{}
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

	return &models.PaginatedDailyResponse{
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

// AssignStations updates the DistrictManger column in WorkingKIT for the specified station IDs
func (s *DataService) AssignStations(ctx context.Context, dmName string, stationIDs []string) (*models.AssignStationsResponse, error) {
	if dmName == "" {
		return nil, fmt.Errorf("dm_name cannot be empty")
	}

	tx, err := s.pool.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Unassign all stations currently assigned to this DM
	unassignSQL := `UPDATE WorkingKIT SET DistrictManger = NULL WHERE DistrictManger = @dmName`
	_, err = tx.ExecContext(ctx, unassignSQL, sql.Named("dmName", dmName))
	if err != nil {
		return nil, fmt.Errorf("failed to clear previous station assignments: %w", err)
	}

	// 2. Assign the new list of stations to this DM
	assignedCount := 0
	if len(stationIDs) > 0 {
		stmt, err := tx.PrepareContext(ctx, `UPDATE WorkingKIT SET DistrictManger = @dmName WHERE StationID = @stnID`)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare assignment statement: %w", err)
		}
		defer stmt.Close()

		for _, sid := range stationIDs {
			sidTrimmed := strings.TrimSpace(sid)
			if sidTrimmed == "" {
				continue
			}
			res, err := stmt.ExecContext(ctx, sql.Named("dmName", dmName), sql.Named("stnID", sidTrimmed))
			if err != nil {
				return nil, fmt.Errorf("failed assigning station %s: %w", sidTrimmed, err)
			}
			if rowsAff, _ := res.RowsAffected(); rowsAff > 0 {
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
		UpdatedCount: assignedCount,
		Message:      fmt.Sprintf("Successfully assigned %d stations to %s", assignedCount, dmName),
	}, nil
}

// GetStationsByDM returns all stations currently assigned to a given DM in WorkingKIT
func (s *DataService) GetStationsByDM(ctx context.Context, dmName string) ([]models.DMStationItem, error) {
	query := `
		SELECT 
			StationID, 
			COALESCE(District, 'N/A') as district,
			COALESCE(StationType, 'Standard') as station_type
		FROM WorkingKIT WITH (NOLOCK)
		WHERE DistrictManger = @dmName
		ORDER BY StationID
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

