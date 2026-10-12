package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"go-service/internal/config"

	_ "github.com/microsoft/go-mssqldb"
)

type Pool struct {
	DB *sql.DB
}

func Connect(cfg *config.Config) (*Pool, error) {
	connString := fmt.Sprintf("server=%s;user id=%s;password=%s;port=%s;database=%s;encrypt=true;trustservercertificate=false;",
		cfg.DBServer, cfg.DBUser, cfg.DBPassword, cfg.DBPort, cfg.DBDatabase)

	db, err := sql.Open("sqlserver", connString)
	if err != nil {
		return nil, fmt.Errorf("error opening db connection: %w", err)
	}

	// Azure SQL optimal pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(10 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping failed: %w", err)
	}

	log.Printf("[DB] Successfully connected to Azure SQL [%s on %s]", cfg.DBDatabase, cfg.DBServer)
	return &Pool{DB: db}, nil
}

func (p *Pool) Close() {
	if p.DB != nil {
		_ = p.DB.Close()
	}
}

