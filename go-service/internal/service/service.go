package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"go-service/internal/db"
	"go-service/internal/models"
)

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
	opts := &models.FilterOptions{
		Months:    []string{"202607"},
		Districts: []string{},
		DMs:       []models.DMOption{},
		Stations:  []string{},
	}

	// Distinct Districts
	distQuery := "SELECT DISTINCT District FROM WorkingKIT WHERE District IS NOT NULL AND LTRIM(RTRIM(District)) <> ''"
	var distArgs []any
	if dmName != "" {
		distQuery += " AND DistrictManger = @dmName"
		distArgs = append(distArgs, sql.Named("dmName", dmName))
	}
	distQuery += " ORDER BY District"

	dRows, err := s.pool.DB.QueryContext(ctx, distQuery, distArgs...)
	if err == nil {
		defer dRows.Close()
		for dRows.Next() {
			var d string
			if err := dRows.Scan(&d); err == nil {
				opts.Districts = append(opts.Districts, d)
			}
		}
	}

	// Distinct DMs from WorkingKIT and EODDB_Users
	dmQuery := `
		SELECT DISTINCT
			COALESCE(u.Id, d.DistrictManger) as id,
			d.DistrictManger as name
		FROM (
			SELECT DistrictManger FROM WorkingKIT WHERE DistrictManger IS NOT NULL AND LTRIM(RTRIM(DistrictManger)) <> ''
			UNION
			SELECT DistrictManager FROM EODDB_Users WHERE DistrictManager IS NOT NULL AND LTRIM(RTRIM(DistrictManager)) <> ''
		) d
		LEFT JOIN EODDB_Users u ON d.DistrictManger = u.DistrictManager
		ORDER BY name
	`
	dmRows, err := s.pool.DB.QueryContext(ctx, dmQuery)
	if err == nil {
		defer dmRows.Close()
		for dmRows.Next() {
			var id, name string
			if err := dmRows.Scan(&id, &name); err == nil {
				opts.DMs = append(opts.DMs, models.DMOption{ID: id, Name: name})
			}
		}
	}

	// Distinct Stations
	stnQuery := "SELECT DISTINCT StationID FROM WorkingKIT WHERE StationID IS NOT NULL AND LTRIM(RTRIM(StationID)) <> ''"
	var stnArgs []any
	if dmName != "" {
		stnQuery += " AND DistrictManger = @dmName"
		stnArgs = append(stnArgs, sql.Named("dmName", dmName))
	}
	stnQuery += " ORDER BY StationID"

	stnRows, err := s.pool.DB.QueryContext(ctx, stnQuery, stnArgs...)
	if err == nil {
		defer stnRows.Close()
		for stnRows.Next() {
			var sid string
			if err := stnRows.Scan(&sid); err == nil {
				opts.Stations = append(opts.Stations, sid)
			}
		}
	}

	return opts, nil
}

// ----------------- DM OVERVIEW & ANALYTICS -----------------

func (s *DataService) GetDMOverview(ctx context.Context, month, dmName, district, stationID string) (*models.DMOverviewResponse, error) {
	table := s.TableForMonth(month)

	var whereClauses []string
	var args []any

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

	// 1. KPI Aggregation
	kpiQuery := fmt.Sprintf(`
		SELECT 
			COUNT(*) as total_enrollment,
			COALESCE(SUM(TRY_CAST(TOTAL_AMOUNT_CHARGED AS decimal(10,2))), 0) as total_revenue,
			COUNT(DISTINCT EnrollDT) as active_days
		FROM [%s]
		%s
	`, table, whereSQL)

	err := s.pool.DB.QueryRowContext(ctx, kpiQuery, args...).Scan(
		&resp.TotalEnrollment,
		&resp.TotalRevenue,
		&resp.ActiveDaysCount,
	)
	if err != nil {
		return nil, fmt.Errorf("error querying KPIs: %w", err)
	}

	// 2. Daily Trend
	trendQuery := fmt.Sprintf(`
		SELECT 
			EnrollDT,
			COUNT(*) as daily_enrollment,
			COALESCE(SUM(TRY_CAST(TOTAL_AMOUNT_CHARGED AS decimal(10,2))), 0) as daily_revenue
		FROM [%s]
		%s
		GROUP BY EnrollDT
		ORDER BY EnrollDT ASC
	`, table, whereSQL)

	trendRows, err := s.pool.DB.QueryContext(ctx, trendQuery, args...)
	if err == nil {
		defer trendRows.Close()
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
				resp.DailyTrend = append(resp.DailyTrend, models.DailyTrendPoint{
					EnrollDate:      isoDate,
					Label:           label,
					TotalEnrollment: count,
					TotalRevenue:    rev,
				})
			}
		}
	}

	// 3. Category Mix
	catQuery := fmt.Sprintf(`
		SELECT 
			COALESCE(UpdateType, 'Other') as cat,
			COUNT(*) as count
		FROM [%s]
		%s
		GROUP BY UpdateType
		ORDER BY count DESC
	`, table, whereSQL)

	catRows, err := s.pool.DB.QueryContext(ctx, catQuery, args...)
	if err == nil {
		defer catRows.Close()
		for catRows.Next() {
			var cat string
			var count int
			if err := catRows.Scan(&cat, &count); err == nil {
				resp.CategoryMix = append(resp.CategoryMix, models.CategoryMixItem{
					Category: cat,
					Count:    count,
				})
			}
		}
	}

	// 4. Top Performing Stations
	topQuery := fmt.Sprintf(`
		SELECT TOP 10
			StationID,
			COALESCE(MAX(District), 'N/A') as district,
			COUNT(DISTINCT EnrollDT) as active_days,
			COUNT(*) as total_enrollment,
			COALESCE(SUM(TRY_CAST(TOTAL_AMOUNT_CHARGED AS decimal(10,2))), 0) as total_revenue
		FROM [%s]
		%s
		GROUP BY StationID
		ORDER BY total_enrollment DESC
	`, table, whereSQL)

	topRows, err := s.pool.DB.QueryContext(ctx, topQuery, args...)
	if err == nil {
		defer topRows.Close()
		for topRows.Next() {
			var sid, dist string
			var days, enrolls int
			var rev float64
			if err := topRows.Scan(&sid, &dist, &days, &enrolls, &rev); err == nil {
				resp.TopStations = append(resp.TopStations, models.TopStationData{
					StationID:       sid,
					StationName:     "Station " + sid,
					DistrictName:    dist,
					ActiveDays:      days,
					TotalEnrollment: enrolls,
					TotalRevenue:    rev,
				})
			}
		}
	}

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
