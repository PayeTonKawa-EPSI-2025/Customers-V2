package db_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/PayeTonKawa-EPSI-2025/Common-V2/models"
	"github.com/PayeTonKawa-EPSI-2025/Customers-V2/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pashagolub/pgxmock"
	"github.com/stretchr/testify/require"
)

// SeedDB inserts initial data for testing
func SeedDB(t *testing.T, pool *pgxpool.Pool) {
	db.Init()

	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Example: Create table if not exists
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Example: Insert sample data
	customers := []models.Customer{
		{Username: "Alice", FirstName: "Alice", LastName: "Smith", Name: "Alice Smith"},
		{Username: "Bob", FirstName: "Bob", LastName: "Johnson", Name: "Bob Johnson"},
	}

	for _, u := range customers {
		_, err := pool.Exec(ctx, `
			INSERT INTO customers (username, first_name, last_name, name) VALUES ($1, $2, $3, $4)
		`, u.Username, u.FirstName, u.LastName, u.Name)
		if err != nil {
			t.Fatalf("failed to insert customer %s: %v", u.Username, err)
		}
	}

	t.Log("Database seeded successfully")
}

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

	// Run a simple query to check connectivity
	var now time.Time
	err := pool.QueryRow(context.Background(), "SELECT NOW()").Scan(&now)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	t.Logf("Successfully connected! Database time: %s", now.Format(time.RFC3339))
}

func TestConnectNoDSN(t *testing.T) {
	os.Unsetenv("DATABASE_DSN")
	t.Run("expect fatal", func(t *testing.T) {
		Connect(t)
	})
}

func TestConnectInvalidDSN(t *testing.T) {
	os.Setenv("DATABASE_DSN", "postgres://invalid:5432/db")
	t.Run("expect fatal", func(t *testing.T) {
		Connect(t)
	})
}

func TestDBConnectAndSeed(t *testing.T) {
	pool := Connect(t)

	// Seed the database
	SeedDB(t, pool)

	// Run a simple query to verify data
	var count int
	err := pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM customers").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count customers: %v", err)
	}

	if count != 2 {
		t.Errorf("expected 2 customers, got %d", count)
	}

	t.Cleanup(func() {
		pool.Exec(context.Background(), "TRUNCATE TABLE IF EXISTS customers")
	})
}

func TestSeedDBCreateTableFail(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)

	// Simulate failure when creating table
	mockPool.ExpectExec("CREATE TABLE IF NOT EXISTS customers").WillReturnError(errors.New("table create failed"))

	// SeedDB should call t.Fatalf on error, so we need to capture t.Fatal
	t.Run("fatal expected", func(t *testing.T) {
		// Optionally, use a helper to capture fatal
		SeedDB(t, mockPool)
	})
}
