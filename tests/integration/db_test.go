package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/PayeTonKawa-EPSI-2025/Common-V2/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Connect returns a pgxpool.Pool connected to DATABASE_DSN.
// Fails the test if DATABASE_DSN is not set or DB is unreachable.
func ConnectDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		t.Fatal("DATABASE_DSN not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect DB: %v", err)
	}

	sqlDB, _ := db.DB()
	if err := sqlDB.PingContext(ctx); err != nil {
		t.Fatalf("DB unreachable: %v", err)
	}

	return db
}

func ResetCustomersTable(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec("TRUNCATE TABLE customers RESTART IDENTITY CASCADE").Error; err != nil {
		t.Fatalf("failed to reset customers table: %v", err)
	}
}

// SeedDB creates the customers table if missing and inserts sample data.
func SeedDB(t *testing.T, db *gorm.DB) {
	t.Helper()

	// Create table automatically (optional)
	err := db.AutoMigrate(&models.Customer{})
	if err != nil {
		t.Fatalf("failed to auto migrate: %v", err)
	}

	customers := []models.Customer{
		{
			Username:  "Alice",
			FirstName: "Alice",
			LastName:  "Smith",
			Name:      "Alice Smith",
			Address: models.Address{
				PostalCode: "75001",
				City:       "Paris",
			},
			Company: models.Company{
				CompanyName: "ACME Corp",
			},
		},
		{
			Username:  "Bob",
			FirstName: "Bob",
			LastName:  "Johnson",
			Name:      "Bob Johnson",
			Address: models.Address{
				PostalCode: "69000",
				City:       "Lyon",
			},
			Company: models.Company{
				CompanyName: "Globex",
			},
		},
	}

	for _, c := range customers {
		if err := db.Create(&c).Error; err != nil {
			t.Fatalf("failed to seed customer %s: %v", c.Username, err)
		}
	}

	t.Log("Database seeded successfully")
}

// -------------------- TESTS -------------------- //

func TestDBConnect(t *testing.T) {
	pool := ConnectDB(t)

	var now time.Time
	err := pool.QueryRow(context.Background(), "SELECT NOW()").Scan(&now)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	t.Logf("Successfully connected! Database time: %s", now.Format(time.RFC3339))
}

func TestDBConnectAndSeed(t *testing.T) {
	pool := ConnectDB(t)
	SeedDB(t, pool)

	var count int
	err := pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM customers").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count customers: %v", err)
	}

	if count != 2 {
		t.Errorf("expected 2 customers, got %d", count)
	}

	ResetCustomersTable(t, pool)
}
