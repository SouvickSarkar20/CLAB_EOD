package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"go-service/internal/models"
	"go-service/internal/service"
)

type APIHandler struct {
	svc *service.DataService
}

func NewAPIHandler(svc *service.DataService) *APIHandler {
	return &APIHandler{svc: svc}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// Health Check
func (h *APIHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, models.HealthStatus{
		Status:    "healthy",
		Database:  "connected",
		Timestamp: time.Now(),
	})
}

// Verify User Credentials
func (h *APIHandler) VerifyUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, err := h.svc.Authenticate(r.Context(), req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Invalid username or password")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// List Users
func (h *APIHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.svc.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, users)
}

// Filter Options
func (h *APIHandler) GetFilterOptions(w http.ResponseWriter, r *http.Request) {
	dm := r.URL.Query().Get("dm")
	opts, err := h.svc.GetFilterOptions(r.Context(), dm)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, opts)
}

// DM Overview Analytics
func (h *APIHandler) GetDMOverview(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	month := q.Get("month")
	if month == "" {
		month = "202607"
	}
	dm := q.Get("dm")
	district := q.Get("district")
	station := q.Get("station_id")

	overview, err := h.svc.GetDMOverview(r.Context(), month, dm, district, station)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, overview)
}

// Daily Logs Breakdown
func (h *APIHandler) GetDailyLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	month := q.Get("month")
	if month == "" {
		month = "202607"
	}
	dm := q.Get("dm")
	district := q.Get("district")
	station := q.Get("station_id")
	enrollDT := q.Get("date")

	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(q.Get("page_size"))
	if pageSize < 1 {
		pageSize = 25
	}

	logs, err := h.svc.GetDailyLogs(r.Context(), month, dm, district, station, enrollDT, page, pageSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, logs)
}

// Bulk Insert Records
func (h *APIHandler) BulkInsert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req models.BulkInsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON body: "+err.Error())
		return
	}

	resp, err := h.svc.BulkInsertRecords(r.Context(), req.Records)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// AssignStations handles assigning stations to a DM in WorkingKIT
func (h *APIHandler) AssignStations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req models.AssignStationsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON body: "+err.Error())
		return
	}

	resp, err := h.svc.AssignStations(r.Context(), req.DMName, req.StationIDs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// GetDMStations returns currently assigned stations for a DM from WorkingKIT
func (h *APIHandler) GetDMStations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	dmName := r.URL.Query().Get("dm")
	if dmName == "" {
		writeError(w, http.StatusBadRequest, "Query parameter 'dm' is required")
		return
	}

	items, err := h.svc.GetStationsByDM(r.Context(), dmName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"dm":       dmName,
		"stations": items,
		"count":    len(items),
	})
}


