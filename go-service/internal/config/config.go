package config

import (
	"bufio"
	"log"
	"os"
	"strings"
)

type Config struct {
	ServerPort     string
	DBServer       string
	DBPort         string
	DBUser         string
	DBPassword     string
	DBDatabase     string
	InternalSecret string
}

// loadDotEnv reads key=value pairs from .env or ../.env if present and sets them in os.Environ
func loadDotEnv() {
	paths := []string{".env", "../.env"}
	for _, p := range paths {
		file, err := os.Open(p)
		if err != nil {
			continue
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				k := strings.TrimSpace(parts[0])
				v := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				if os.Getenv(k) == "" {
					_ = os.Setenv(k, v)
				}
			}
		}
		return
	}
}

func Load() *Config {
	loadDotEnv()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	dbServer := os.Getenv("AZURE_SQL_SERVER")
	dbPort := os.Getenv("AZURE_SQL_PORT")
	if dbPort == "" {
		dbPort = "1433"
	}

	dbUser := os.Getenv("AZURE_SQL_USER")
	dbPassword := os.Getenv("AZURE_SQL_PASSWORD")
	dbDatabase := os.Getenv("AZURE_SQL_DATABASE")
	if dbDatabase == "" {
		dbDatabase = "CLAB-EOD"
	}

	if dbServer == "" || dbUser == "" || dbPassword == "" || dbDatabase == "" {
		log.Println("[CONFIG WARNING] One or more database credentials (AZURE_SQL_SERVER, AZURE_SQL_USER, AZURE_SQL_PASSWORD, AZURE_SQL_DATABASE) are not set!")
	}

	internalSecret := os.Getenv("INTERNAL_API_SECRET")
	if internalSecret == "" {
		internalSecret = os.Getenv("GO_SERVICE_SECRET")
	}
	if internalSecret == "" {
		log.Println("[SECURITY WARNING] INTERNAL_API_SECRET is not set in environment or .env file!")
	}

	return &Config{
		ServerPort:     port,
		DBServer:       dbServer,
		DBPort:         dbPort,
		DBUser:         dbUser,
		DBPassword:     dbPassword,
		DBDatabase:     dbDatabase,
		InternalSecret: internalSecret,
	}
}
