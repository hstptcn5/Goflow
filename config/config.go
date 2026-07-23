package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Host                      string
	Port                      string
	DBPath                    string
	MasterKey                 string
	LogLevel                  string
	FrontendDist              string
	APIKey                    string
	MaxConcurrentExecutions   int
	WebhookRateLimitPerMinute int
	ExecutionRetentionDays    int
	MaxExecutionsPerWorkflow  int
}

func LoadConfig() *Config {
	host := os.Getenv("GOFLOW_HOST")
	if host == "" {
		host = "127.0.0.1"
	}

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
		var err error
		masterKey, err = loadOrCreateMasterKey(dbPath)
		if err != nil {
			panic(fmt.Sprintf("failed to load or create master key: %v", err))
		}
	}

	logLevel := os.Getenv("GOFLOW_LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	apiKey := os.Getenv("GOFLOW_API_KEY")

	return &Config{
		Host:                      host,
		Port:                      port,
		DBPath:                    dbPath,
		MasterKey:                 masterKey,
		LogLevel:                  logLevel,
		FrontendDist:              "ui/dist",
		APIKey:                    apiKey,
		MaxConcurrentExecutions:   getEnvInt("GOFLOW_MAX_CONCURRENT_EXECUTIONS", 10),
		WebhookRateLimitPerMinute: getEnvInt("GOFLOW_WEBHOOK_RATE_LIMIT_PER_MINUTE", 60),
		ExecutionRetentionDays:    getEnvInt("GOFLOW_EXECUTION_RETENTION_DAYS", 30),
		MaxExecutionsPerWorkflow:  getEnvInt("GOFLOW_MAX_EXECUTIONS_PER_WORKFLOW", 1000),
	}
}

func (c *Config) IsPublicBind() bool {
	host := strings.TrimSpace(c.Host)
	if host == "" || host == "*" {
		return true
	}
	if strings.EqualFold(host, "localhost") {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return true
	}
	return !ip.IsLoopback()
}

func loadOrCreateMasterKey(dbPath string) (string, error) {
	keyPath := os.Getenv("GOFLOW_MASTER_KEY_FILE")
	if keyPath == "" {
		dir := filepath.Dir(dbPath)
		if dir == "." || dir == "" {
			dir = "."
		}
		keyPath = filepath.Join(dir, "goflow.master.key")
	}

	if data, err := os.ReadFile(keyPath); err == nil {
		key := strings.TrimSpace(string(data))
		if key == "" {
			return "", fmt.Errorf("master key file is empty: %s", keyPath)
		}
		return key, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}

	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", err
	}
	key := base64.RawURLEncoding.EncodeToString(keyBytes)
	if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
		return "", err
	}
	if err := os.WriteFile(keyPath, []byte(key+"\n"), 0600); err != nil {
		return "", err
	}
	return key, nil
}

func getEnvInt(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return fallback
	}
	return value
}
