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

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	fmt.Println("==================================================")
	fmt.Println(" STEP 1: COUNTING ROWS FOR AUGUST & SEPTEMBER")
	fmt.Println("==================================================")

	var augCountBefore, sepCountBefore int
	err = pool.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM EODFiles WHERE EnrollMth = '202608'").Scan(&augCountBefore)
	if err != nil {
		log.Fatalf("Error querying August row count: %v", err)
	}
	err = pool.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM EODFiles WHERE EnrollMth = '202609'").Scan(&sepCountBefore)
	if err != nil {
		log.Fatalf("Error querying September row count: %v", err)
	}

	fmt.Printf("Rows before deletion:\n")
	fmt.Printf(" - August 2026 (202608):    %d rows\n", augCountBefore)
	fmt.Printf(" - September 2026 (202609): %d rows\n\n", sepCountBefore)

	if augCountBefore == 0 && sepCountBefore == 0 {
		fmt.Println("No rows found for August or September. Database is already clean.")
		return
	}

	fmt.Println("==================================================")
	fmt.Println(" STEP 2: DELETING AUGUST & SEPTEMBER ROWS")
	fmt.Println("==================================================")

	deleteQuery := `
		DELETE FROM EODFiles 
		WHERE EnrollMth IN ('202608', '202609');
	`
	start := time.Now()
	res, err := pool.DB.ExecContext(ctx, deleteQuery)
	if err != nil {
		log.Fatalf("Failed to delete rows: %v", err)
	}
	rowsAffected, _ := res.RowsAffected()
	fmt.Printf("Deletion executed successfully in %v. Total rows deleted: %d\n\n", time.Since(start), rowsAffected)

	fmt.Println("==================================================")
	fmt.Println(" STEP 3: VERIFYING DELETION")
	fmt.Println("==================================================")

	var augCountAfter, sepCountAfter int
	err = pool.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM EODFiles WHERE EnrollMth = '202608'").Scan(&augCountAfter)
	if err != nil {
		log.Fatalf("Error re-checking August row count: %v", err)
	}
	err = pool.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM EODFiles WHERE EnrollMth = '202609'").Scan(&sepCountAfter)
	if err != nil {
		log.Fatalf("Error re-checking September row count: %v", err)
	}

	fmt.Printf("Rows remaining after deletion:\n")
	fmt.Printf(" - August 2026 (202608):    %d rows\n", augCountAfter)
	fmt.Printf(" - September 2026 (202609): %d rows\n", sepCountAfter)

	if augCountAfter == 0 && sepCountAfter == 0 {
		fmt.Println("\n>>> VERIFICATION SUCCESSFUL: Both August and September data are completely deleted! <<<")
	} else {
		log.Fatalf("\n>>> VERIFICATION FAILED: Some rows were not deleted! <<<")
	}
}

