package db_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/PayeTonKawa-EPSI-2025/Common-V2/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect returns a pgxpool.Pool connected to DATABASE_DSN.
// Fails the test if DATABASE_DSN is not set or DB is unreachable.
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

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("failed to ping postgres: %v", err)
	}

	return pool
}

func StopSeededTest(t *testing.T, pool *pgxpool.Pool) {
	// Safe cleanup: only truncate if table exists
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DO $$
			BEGIN
				IF EXISTS (SELECT FROM pg_tables WHERE schemaname = 'public' AND tablename = 'customers') THEN
					TRUNCATE TABLE customers RESTART IDENTITY CASCADE;
				END IF;
			END
			$$;`)
	})
}

// SeedDB creates the customers table if missing and inserts sample data.
func SeedDB(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS customers (
			id SERIAL PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			first_name TEXT NOT NULL,
			last_name TEXT NOT NULL,
			name TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		t.Fatalf("failed to create customers table: %v", err)
	}

	customers := []models.Customer{
		{Username: "Alice", FirstName: "Alice", LastName: "Smith", Name: "Alice Smith"},
		{Username: "Bob", FirstName: "Bob", LastName: "Johnson", Name: "Bob Johnson"},
	}

	for _, c := range customers {
		_, err := pool.Exec(ctx, `
			INSERT INTO customers (username, first_name, last_name, name) VALUES ($1, $2, $3, $4)
		`, c.Username, c.FirstName, c.LastName, c.Name)
		if err != nil {
			t.Fatalf("failed to insert customer %s: %v", c.Username, err)
		}
	}

	t.Log("Database seeded successfully")
}

// -------------------- TESTS -------------------- //

func TestDBConnect(t *testing.T) {
	pool := Connect(t)

	var now time.Time
	err := pool.QueryRow(context.Background(), "SELECT NOW()").Scan(&now)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	t.Logf("Successfully connected! Database time: %s", now.Format(time.RFC3339))
}

func TestDBConnectAndSeed(t *testing.T) {
	pool := Connect(t)
	SeedDB(t, pool)

	var count int
	err := pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM customers").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count customers: %v", err)
	}

	if count != 2 {
		t.Errorf("expected 2 customers, got %d", count)
	}

	StopSeededTest(t, pool)
}
