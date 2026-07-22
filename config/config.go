package config

import (
	"os"
)

type Config struct {
	Port         string
	DBPath       string
	MasterKey    string
	LogLevel     string
	FrontendDist string
}

func LoadConfig() *Config {
	port := os.Getenv("GOFLOW_PORT")
	if port == "" {
		port = "8080"
	}

	dbPath := os.Getenv("GOFLOW_DB_PATH")
	if dbPath == "" {
		dbPath = "goflow.db"
	}

	masterKey := os.Getenv("GOFLOW_MASTER_KEY")
	if masterKey == "" {
		// Mặc định 32-byte key ngầm định nếu không truyền qua env
		masterKey = "goflow-master-secret-key-32bytes!"
	}

	logLevel := os.Getenv("GOFLOW_LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	return &Config{
		Port:         port,
		DBPath:       dbPath,
		MasterKey:    masterKey,
		LogLevel:     logLevel,
		FrontendDist: "ui/dist",
	}
}
