package postgres

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestApplyDefaultConnectTimeout_setsWhenZero(t *testing.T) {
	cfg, err := pgxpool.ParseConfig("postgres://u:p@127.0.0.1:5432/db?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConnConfig.ConnectTimeout != 0 {
		t.Fatalf("fixture ConnectTimeout = %v, want 0 before apply", cfg.ConnConfig.ConnectTimeout)
	}
	ApplyDefaultConnectTimeout(cfg, 30*time.Second)
	if cfg.ConnConfig.ConnectTimeout != 30*time.Second {
		t.Fatalf("ConnectTimeout = %v, want 30s", cfg.ConnConfig.ConnectTimeout)
	}
}

func TestApplyDefaultConnectTimeout_preservesExisting(t *testing.T) {
	cfg, err := pgxpool.ParseConfig("postgres://u:p@127.0.0.1:5432/db?sslmode=disable&connect_timeout=5")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConnConfig.ConnectTimeout != 5*time.Second {
		t.Fatalf("fixture ConnectTimeout = %v, want 5s", cfg.ConnConfig.ConnectTimeout)
	}
	ApplyDefaultConnectTimeout(cfg, 30*time.Second)
	if cfg.ConnConfig.ConnectTimeout != 5*time.Second {
		t.Fatalf("ConnectTimeout = %v, want preserved 5s", cfg.ConnConfig.ConnectTimeout)
	}
}

func TestApplyDefaultConnectTimeout_nilSafe(t *testing.T) {
	ApplyDefaultConnectTimeout(nil, 30*time.Second) // must not panic
}
