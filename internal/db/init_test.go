package db_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		t.Fatal("DATABASE_DSN is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("failed to create pg pool: %v", err)
	}

	// Ensure DB is reachable
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("failed to ping postgres: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
	})

	return pool
}

func TestDBConnect(t *testing.T) {
	pool := Connect(t)

	// Run a simple query to use pool
	var now string
	err := pool.QueryRow(context.Background(), "SELECT NOW()").Scan(&now)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	t.Logf("Successfully connected! Database time: %s", now)
}
