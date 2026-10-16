package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go-service/internal/config"
	"go-service/internal/db"
	"go-service/internal/handlers"
	"go-service/internal/service"
)

// Simple CORS and logging middleware
func withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Internal-Secret")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
		log.Printf("[%s] %s -> completed in %v", r.Method, r.URL.Path, time.Since(start))
	})
}

// requireInternalSecret validates that requests to internal API endpoints carry the shared internal secret
func requireInternalSecret(secret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow CORS preflights
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// Health check probe endpoint is always open for container/k8s readiness & liveness probes
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		// Check for X-Internal-Secret header or Bearer token Authorization header
		reqSecret := r.Header.Get("X-Internal-Secret")
		if reqSecret == "" {
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				reqSecret = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		// Constant-time comparison to prevent timing attacks
		if reqSecret == "" || subtle.ConstantTimeCompare([]byte(reqSecret), []byte(secret)) != 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "Unauthorized: invalid or missing internal API secret",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	cfg := config.Load()
	log.Printf("Starting Go DB Microservice on port %s...", cfg.ServerPort)

	// Connect to Azure SQL
	pool, err := db.Connect(cfg)
	if err != nil {
		log.Fatalf("Fatal: could not connect to database: %v", err)
	}
	defer pool.Close()

	// Initialize service & handlers
	svc := service.NewDataService(pool)
	h := handlers.NewAPIHandler(svc)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.HealthCheck)
	mux.HandleFunc("/api/auth/verify", h.VerifyUser)
	mux.HandleFunc("/api/auth/users", h.ListUsers)
	mux.HandleFunc("/api/filters/options", h.GetFilterOptions)
	mux.HandleFunc("/api/dm/overview", h.GetDMOverview)
	mux.HandleFunc("/api/daily/logs", h.GetDailyLogs)
	mux.HandleFunc("/api/monthly/records", h.GetMonthlyRecords)
	mux.HandleFunc("/api/records/bulk-insert", h.BulkInsert)
	mux.HandleFunc("/api/admin/assign-stations", h.AssignStations)
	mux.HandleFunc("/api/dm/stations", h.GetDMStations)
	mux.HandleFunc("/api/auth/update-password", h.UpdatePassword)

	server := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      withMiddleware(requireInternalSecret(cfg.InternalSecret, mux)),
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("Go Microservice HTTP Server listening on http://0.0.0.0:%s", cfg.ServerPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Pre-warm filter options cache in background so incoming requests are instant
	go func() {
		log.Println("[CACHE] Warming up filter options cache...")
		warmCtx, warmCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer warmCancel()
		if _, err := svc.GetFilterOptions(warmCtx, ""); err != nil {
			log.Printf("[CACHE] Warm-up error: %v", err)
		} else {
			log.Println("[CACHE] Filter options cache successfully warmed up!")
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down Go Microservice...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Forced shutdown error: %v", err)
	}
	log.Println("Go Microservice cleanly stopped.")
}

