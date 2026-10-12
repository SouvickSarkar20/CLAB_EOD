package models

import "time"

// User represents a user from EODDB_Users
type DBUser struct {
	ID              string `json:"id"`
	DistrictManager string `json:"district_manager"`
	Role            string `json:"role"`
	Password        string `json:"password,omitempty"`
}

// LoginRequest / Response
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type UserResponse struct {
	ID              string `json:"id"`
	DistrictManager string `json:"district_manager"`
	Role            string `json:"role"`
}

// WorkingKitAssignment represents station-to-DM assignment
type WorkingKitAssignment struct {
	UserID         string `json:"user_id"`
	StationID      string `json:"station_id"`
	DistrictManger string `json:"district_manager"`
	District       string `json:"district"`
	OperatorName   string `json:"operator_name"`
	StationType    string `json:"station_type"`
	EnrollMonth    string `json:"enroll_month"`
}

// FilterOptions
type FilterOptions struct {
	Months    []string   `json:"months"`
	Districts []string   `json:"districts"`
	DMs       []DMOption `json:"district_managers"`
	Stations  []string   `json:"stations"`
}

type DMOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// DMOverviewResponse matches the exact frontend schema
type DMOverviewResponse struct {
	TotalEnrollment int               `json:"total_enrollment"`
	TotalRevenue    float64           `json:"total_revenue"`
	ActiveDaysCount int               `json:"active_days_count"`
	DailyTrend      []DailyTrendPoint `json:"daily_trend"`
	TopStations     []TopStationData  `json:"top_stations"`
	CategoryMix     []CategoryMixItem `json:"category_mix"`
}

type DailyTrendPoint struct {
	EnrollDate      string  `json:"enroll_date"`
	Label           string  `json:"label"`
	TotalEnrollment int     `json:"total_enrollment"`
	TotalRevenue    float64 `json:"total_revenue"`
}

type TopStationData struct {
	StationID       string  `json:"station_id"`
	StationName     string  `json:"station_name"`
	DistrictName    string  `json:"district_name"`
	ActiveDays      int     `json:"active_days"`
	TotalEnrollment int     `json:"total_enrollment"`
	TotalRevenue    float64 `json:"total_revenue"`
}

type CategoryMixItem struct {
	Category string `json:"category"`
	Count    int    `json:"count"`
}

// DailyRecordItem represents an aggregated day-station-operator row or raw row
type DailyRecordItem struct {
	EnrollDate      string  `json:"enroll_date"`
	StationID       string  `json:"station_id"`
	OperatorName    string  `json:"operator_name"`
	District        string  `json:"district"`
	DistrictManager string  `json:"district_manager"`
	TotalEnrollment int     `json:"total_enrollment"`
	TotalAmount     float64 `json:"total_amount"`
	BMU100          int     `json:"bmu_100"`
	BMU125          int     `json:"bmu_125"`
	DMU50           int     `json:"dmu_50"`
	DMU75           int     `json:"dmu_75"`
	MBU0            int     `json:"mbu_0"`
	MBU100          int     `json:"mbu_100"`
	MBU125          int     `json:"mbu_125"`
	New0            int     `json:"new_0"`
}

type PaginatedDailyResponse struct {
	Items      []DailyRecordItem `json:"items"`
	TotalCount int               `json:"total_count"`
	Page       int               `json:"page"`
	PageSize   int               `json:"page_size"`
}

// HealthStatus
type HealthStatus struct {
	Status    string    `json:"status"`
	Database  string    `json:"database"`
	Timestamp time.Time `json:"timestamp"`
}

