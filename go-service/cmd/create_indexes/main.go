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

	// 10 minutes timeout for index creation on 3.8M rows
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	fmt.Println("Starting Index Creation on Azure SQL EODFiles (3.8M rows)...")

	// Index 1: Month index for distinct listing & quick filtering
	idxMonthSQL := `
		IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = 'IX_EODFiles_EnrollMth' AND object_id = OBJECT_ID('EODFiles'))
		BEGIN
			PRINT 'Creating IX_EODFiles_EnrollMth...';
			CREATE NONCLUSTERED INDEX IX_EODFiles_EnrollMth
			ON EODFiles (EnrollMth);
		END
		ELSE
		BEGIN
			PRINT 'IX_EODFiles_EnrollMth already exists.';
		END
	`
	start := time.Now()
	fmt.Println("--> Executing Index 1: IX_EODFiles_EnrollMth ...")
	_, err = pool.DB.ExecContext(ctx, idxMonthSQL)
	if err != nil {
		log.Fatalf("Failed to create IX_EODFiles_EnrollMth: %v", err)
	}
	fmt.Printf("--> Index 1 successfully created/verified in %v\n\n", time.Since(start))

	// Index 2: Covering index for Month + DM analytics aggregations
	idxCoveringSQL := `
		IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = 'IX_EODFiles_EnrollMth_DM' AND object_id = OBJECT_ID('EODFiles'))
		BEGIN
			PRINT 'Creating IX_EODFiles_EnrollMth_DM...';
			CREATE NONCLUSTERED INDEX IX_EODFiles_EnrollMth_DM
			ON EODFiles (EnrollMth, DistrictManger)
			INCLUDE (
				EnrollDT,
				StationID,
				District,
				UpdateType,
				TYPE,
				TOTAL_AMOUNT_CHARGED,
				OperatorName
			);
		END
		ELSE
		BEGIN
			PRINT 'IX_EODFiles_EnrollMth_DM already exists.';
		END
	`
	start = time.Now()
	fmt.Println("--> Executing Index 2: IX_EODFiles_EnrollMth_DM (Covering Index) ...")
	_, err = pool.DB.ExecContext(ctx, idxCoveringSQL)
	if err != nil {
		log.Fatalf("Failed to create IX_EODFiles_EnrollMth_DM: %v", err)
	}
	fmt.Printf("--> Index 2 successfully created/verified in %v\n\n", time.Since(start))

	fmt.Println("=== ALL INDEXES CREATED AND VERIFIED SUCCESSFULLY ===")
}

