package operation_test

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/PayeTonKawa-EPSI-2025/Common-V2/models"
	"github.com/PayeTonKawa-EPSI-2025/Customers-V2/internal/dto"
	"github.com/PayeTonKawa-EPSI-2025/Customers-V2/internal/operation"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	dbMock, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: dbMock,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open gorm DB: %v", err)
	}

	return gormDB, mock
}

func TestGetCustomers(t *testing.T) {
	db, mock := setupMockDB(t)

	rows := sqlmock.NewRows([]string{"id", "username", "first_name", "last_name"}).
		AddRow(1, "jdoe", "John", "DOE").
		AddRow(2, "asmith", "Alice", "SMITH")

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "customers"`)).WillReturnRows(rows)

	resp, err := operation.GetCustomers(context.Background(), db)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resp.Body.Customers) != 2 {
		t.Errorf("expected 2 customers, got %d", len(resp.Body.Customers))
	}

	if resp.Body.Customers[0].Username != "jdoe" {
		t.Errorf("expected first customer 'jdoe', got '%s'", resp.Body.Customers[0].Username)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %v", err)
	}
}

func TestGetCustomersDBError(t *testing.T) {
	db, mock := setupMockDB(t)

	mock.ExpectQuery(`SELECT \* FROM "customers"`).
		WillReturnError(errors.New("db failure"))

	_, err := operation.GetCustomers(context.Background(), db)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetCustomerOK(t *testing.T) {
	db, mock := setupMockDB(t)

	rows := sqlmock.NewRows([]string{"id", "username", "first_name", "last_name"}).
		AddRow(1, "jdoe", "John", "DOE")

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT * FROM "customers" WHERE "customers"."id" = $1 AND "customers"."deleted_at" IS NULL ORDER BY "customers"."id" LIMIT $2`,
	)).
		WithArgs(1, sqlmock.AnyArg()).
		WillReturnRows(rows)

	resp, err := operation.GetCustomer(context.Background(), db, 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.Body.Username != "jdoe" {
		t.Errorf("expected username 'jdoe', got '%s'", resp.Body.Username)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %v", err)
	}
}

func TestGetCustomerNotFound(t *testing.T) {
	db, mock := setupMockDB(t)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "customers" WHERE "customers"."id" = $1`)).
		WithArgs(1, sqlmock.AnyArg()).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := operation.GetCustomer(context.Background(), db, 1)
	if err == nil {
		t.Fatal("expected error for non-existent customer")
	}
}

func TestGetCustomerDBError(t *testing.T) {
	db, mock := setupMockDB(t)

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT * FROM "customers" WHERE "customers"."id" = $1 AND "customers"."deleted_at" IS NULL ORDER BY "customers"."id" LIMIT $2`,
	)).
		WithArgs(1, sqlmock.AnyArg()).
		WillReturnError(errors.New("db failure"))

	_, err := operation.GetCustomer(context.Background(), db, 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %v", err)
	}
}

func TestCreateCustomer(t *testing.T) {
	db, mock := setupMockDB(t)

	input := &dto.CustomerCreateInput{
		Body: dto.CustomerCreateBody{
			Username:  "jdoe",
			FirstName: "john",
			LastName:  "doe",
			Address:   models.Address{}, // test address
			Company:   models.Company{}, // test company
		},
	}

	// Mock insert
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "customers"`)).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	resp, err := operation.CreateCustomer(context.Background(), db, nil, input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.Body.Username != "jdoe" {
		t.Errorf("expected username 'jdoe', got '%s'", resp.Body.Username)
	}

	if resp.Body.FirstName != "John" {
		t.Errorf("expected first name 'John', got '%s'", resp.Body.FirstName)
	}

	if resp.Body.LastName != "DOE" {
		t.Errorf("expected last name 'DOE', got '%s'", resp.Body.LastName)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %v", err)
	}
}

func TestUpdateCustomer(t *testing.T) {
	db, mock := setupMockDB(t)

	// Mock selecting existing customer
	mock.ExpectQuery(`SELECT \* FROM "customers".*`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "first_name", "last_name"}).
			AddRow(1, "jdoe", "John", "DOE"))

	// Mock update
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "customers"`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	input := dto.CustomerCreateInput{
		Body: dto.CustomerCreateBody{
			Username:  "jdoe2",
			FirstName: "johnny",
			LastName:  "doe",
			Address:   models.Address{},
			Company:   models.Company{},
		},
	}

	resp, err := operation.UpdateCustomer(context.Background(), db, nil, 1, input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.Body.Username != "jdoe2" {
		t.Errorf("expected username 'jdoe2', got '%s'", resp.Body.Username)
	}
}

func TestUpdateCustomerNotFound(t *testing.T) {
	db, mock := setupMockDB(t)

	mock.ExpectQuery(`SELECT \* FROM "customers"`).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := operation.UpdateCustomer(context.Background(), db, nil, 1, dto.CustomerCreateInput{})
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestDeleteCustomer(t *testing.T) {
	db, mock := setupMockDB(t)

	// Mock select existing customer
	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT * FROM "customers" WHERE "customers"."id" = $1 AND "customers"."deleted_at" IS NULL ORDER BY "customers"."id" LIMIT $2`,
	)).
		WithArgs(1, sqlmock.AnyArg()). // first arg is id, second is limit
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "first_name", "last_name"}).AddRow(1, "jdoe", "John", "DOE"))

	// Mock soft delete
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(
		`UPDATE "customers" SET "deleted_at"=$1 WHERE "customers"."id" = $2 AND "customers"."deleted_at" IS NULL`)).
		WithArgs(sqlmock.AnyArg(), 1).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := operation.DeleteCustomer(context.Background(), db, nil, 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
