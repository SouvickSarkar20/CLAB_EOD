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
		log.Fatalf("Failed to connect: %v", err)
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	query := `
		SELECT 
			t.NAME AS table_name,
			p.rows AS row_count
		FROM sys.tables t
		INNER JOIN sys.partitions p ON t.object_id = p.OBJECT_ID
		WHERE p.index_id IN (0, 1) AND t.NAME IN ('EODFiles', 'EOD_202607', 'WorkingKIT', 'EODDB_Users')
		ORDER BY t.NAME;
	`
	rows, err := pool.DB.QueryContext(ctx, query)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var tbl string
		var cnt int64
		if err := rows.Scan(&tbl, &cnt); err == nil {
			fmt.Printf("Table [%s] -> %d rows\n", tbl, cnt)
		}
	}
}

