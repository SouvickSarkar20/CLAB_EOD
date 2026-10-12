package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"go-service/internal/config"
	"go-service/internal/db"
)

func main() {
	cfg := config.Load()
	pool, err := db.Connect(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to Azure SQL: %v", err)
	}
	defer pool.Close()

	// 20 minutes context deadline
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	fmt.Println("Resuming creation of Index 2: IX_EODFiles_EnrollMth_DM with 20-minute timeout...")

	idxCoveringSQL := `
		IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = 'IX_EODFiles_EnrollMth_DM' AND object_id = OBJECT_ID('EODFiles'))
		BEGIN
			CREATE NONCLUSTERED INDEX IX_EODFiles_EnrollMth_DM
			ON EODFiles (EnrollMth, DistrictManger);
		END
	`
	start := time.Now()
	_, err = pool.DB.ExecContext(ctx, idxCoveringSQL)
	if err != nil {
		log.Fatalf("Failed to create IX_EODFiles_EnrollMth_DM: %v", err)
	}
	fmt.Printf("--> Index 2 (EnrollMth, DistrictManger) successfully created in %v!\n", time.Since(start))
}

