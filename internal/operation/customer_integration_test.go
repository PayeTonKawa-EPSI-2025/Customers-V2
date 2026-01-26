package integration_test

import (
	"context"
	"testing"

	"github.com/PayeTonKawa-EPSI-2025/Common-V2/models"
	"github.com/PayeTonKawa-EPSI-2025/Customers-V2/internal/dto"
	"github.com/PayeTonKawa-EPSI-2025/Customers-V2/internal/operation"
)

func TestIntegration_GetCustomers(t *testing.T) {
	db := ConnectDB(t)
	ResetCustomersTable(t, db)
	SeedDB(t, db)

	resp, err := operation.GetCustomers(context.Background(), db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Body.Customers) != 2 {
		t.Fatalf("expected 2 customers, got %d", len(resp.Body.Customers))
	}
}

func TestIntegration_CreateCustomer(t *testing.T) {
	db := ConnectDB(t)
	ResetCustomersTable(t, db)

	input := dto.CustomerCreateInput{
		Body: models.Customer{
			Username:  "john",
			FirstName: "john",
			LastName:  "doe",
		},
	}

	resp, err := operation.CreateCustomer(context.Background(), db, nil, &input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Body.Username != "john" {
		t.Fatalf("expected username john")
	}
}

func TestIntegration_UpdateCustomer(t *testing.T) {
	db := ConnectDB(t)
	ResetCustomersTable(t, db)
	SeedDB(t, db)

	input := dto.CustomerCreateInput{
		Body: models.Customer{
			Username:  "alice_updated",
			FirstName: "Alice",
			LastName:  "Smith",
			Name:      "Alice Smith",
		},
	}

	resp, err := operation.UpdateCustomer(context.Background(), db, nil, 1, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Body.Username != "alice_updated" {
		t.Fatalf("update failed")
	}
}

func TestIntegration_DeleteCustomer(t *testing.T) {
	db := ConnectDB(t)
	ResetCustomersTable(t, db)
	SeedDB(t, db)

	err := operation.DeleteCustomer(context.Background(), db, nil, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := operation.GetCustomer(context.Background(), db, 1)
	if err == nil {
		t.Fatalf("expected not found after delete")
	}
}
