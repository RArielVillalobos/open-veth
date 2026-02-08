package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Docker   DockerConfig
}

// ServerConfig holds HTTP server settings
type ServerConfig struct {
	Address          string
	ShutdownTimeout  time.Duration
	AutoSaveInterval time.Duration
}

// DatabaseConfig holds database connection settings
type DatabaseConfig struct {
	Driver string
	DSN    string
}

// DockerConfig holds Docker-related timeouts
type DockerConfig struct {
	ExecTimeout    time.Duration
	CleanupTimeout time.Duration
}

// Load reads configuration from environment variables with sensible defaults
func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Address:          getEnv("SERVER_ADDRESS", ":8080"),
			ShutdownTimeout:  getDurationEnv("SERVER_SHUTDOWN_TIMEOUT", 10*time.Second),
			AutoSaveInterval: getDurationEnv("AUTO_SAVE_INTERVAL", 30*time.Second),
		},
		Database: DatabaseConfig{
			Driver: getEnv("DB_DRIVER", "sqlite"),
			DSN:    getEnv("DB_DSN", "openveth.db"),
		},
		Docker: DockerConfig{
			ExecTimeout:    getDurationEnv("DOCKER_EXEC_TIMEOUT", 5*time.Second),
			CleanupTimeout: getDurationEnv("DOCKER_CLEANUP_TIMEOUT", 5*time.Second),
		},
	}
}

// getEnv reads an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getDurationEnv reads a duration from environment (in seconds) or returns default
func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if seconds, err := strconv.Atoi(value); err == nil {
			return time.Duration(seconds) * time.Second
		}
	}
	return defaultValue
}
