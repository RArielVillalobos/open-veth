package api

import (
	"log/slog"
	"testing"

	"open-veth/internal/config"
)

func TestNewServerRequiresWorkingDatabase(t *testing.T) {
	_, err := NewServer(nil, &config.Config{Database: config.DatabaseConfig{Driver: "invalid"}}, slog.Default())
	if err == nil {
		t.Fatal("invalid database must prevent server startup")
	}
}

func TestNewServerAllowsExplicitMemoryStorage(t *testing.T) {
	server, err := NewServer(nil, &config.Config{Database: config.DatabaseConfig{Driver: "memory"}}, slog.Default())
	if err != nil || server == nil {
		t.Fatalf("explicit memory storage failed: %v", err)
	}
}
