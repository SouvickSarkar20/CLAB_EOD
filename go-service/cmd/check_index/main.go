package main

import (
	"context"
	"database/sql"
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

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	fmt.Println("=== 1. CHECKING EXISTING INDEXES ON EODFiles ===")
	indexQuery := `
		SELECT 
			i.name AS index_name,
			c.name AS column_name,
			i.type_desc,
			i.is_primary_key
		FROM sys.indexes i
		INNER JOIN sys.index_columns ic ON i.object_id = ic.object_id AND i.index_id = ic.index_id
		INNER JOIN sys.columns c ON ic.object_id = c.object_id AND ic.column_id = c.column_id
		WHERE i.object_id = OBJECT_ID('EODFiles')
		ORDER BY i.name, ic.key_ordinal;
	`
	rows, err := pool.DB.QueryContext(ctx, indexQuery)
	if err != nil {
		fmt.Printf("Error querying sys.indexes: %v\n", err)
	} else {
		defer rows.Close()
		found := false
		for rows.Next() {
			var idxName, colName, typeDesc string
			var isPK bool
			if err := rows.Scan(&idxName, &colName, &typeDesc, &isPK); err == nil {
				found = true
				fmt.Printf("Found index: %s on column: %s (Type: %s, PrimaryKey: %v)\n", idxName, colName, typeDesc, isPK)
			}
		}
		if !found {
			fmt.Println("No indexes found on EODFiles table.")
		}
	}

	fmt.Println("\n=== 2. CHECKING DDL / CREATE INDEX PERMISSIONS ===")
	permQuery := `
		SELECT 
			HAS_PERMS_BY_NAME(DB_NAME(), 'DATABASE', 'CREATE TABLE') AS can_create_table,
			HAS_PERMS_BY_NAME('EODFiles', 'OBJECT', 'ALTER') AS can_alter_table,
			USER_NAME() AS current_db_user;
	`
	var canCreateTable, canAlterTable sql.NullInt64
	var currentUser string
	err = pool.DB.QueryRowContext(ctx, permQuery).Scan(&canCreateTable, &canAlterTable, &currentUser)
	if err != nil {
		fmt.Printf("Error checking permissions via HAS_PERMS_BY_NAME: %v\n", err)
	} else {
		fmt.Printf("Connected DB User: %s\n", currentUser)
		fmt.Printf("Has ALTER permission on EODFiles (required to CREATE INDEX): %v (1 = Yes, 0 = No)\n", canAlterTable.Int64)
		fmt.Printf("Has CREATE TABLE permission on database: %v (1 = Yes, 0 = No)\n", canCreateTable.Int64)
	}
}

