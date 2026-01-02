package operation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/PayeTonKawa-EPSI-2025/Common-V2/events"
	"github.com/PayeTonKawa-EPSI-2025/Common-V2/models"
	"github.com/PayeTonKawa-EPSI-2025/Customers-V2/internal/dto"
	"github.com/PayeTonKawa-EPSI-2025/Customers-V2/internal/rabbitmq"
	"github.com/danielgtaylor/huma/v2"
	amqp "github.com/rabbitmq/amqp091-go"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gorm.io/gorm"
)

var httpGet = http.Get // default http.Get, can be overridden in tests

// ----------------------
// Extracted CRUD Functions
// ----------------------

// Get all customers
func GetCustomers(ctx context.Context, db *gorm.DB) (*dto.CustomersOutput, error) {
	resp := &dto.CustomersOutput{}

	var customers []models.Customer
	results := db.Find(&customers)

	if results.Error == nil {
		resp.Body.Customers = customers
	}

	return resp, results.Error
}

// Get a single customer by ID
func GetCustomer(ctx context.Context, db *gorm.DB, id uint) (*dto.CustomerOutput, error) {
	resp := &dto.CustomerOutput{}

	// 1️⃣ Fetch customer from local DB
	var customer models.Customer
	results := db.First(&customer, id)
	if results.Error != nil {
		if errors.Is(results.Error, gorm.ErrRecordNotFound) {
			return nil, huma.NewError(http.StatusNotFound, "Customer not found")
		}
		return nil, results.Error
	}

	// 2️⃣ Assign basic customer data
	resp.Body = customer

	// 3️⃣ Fetch orders from Orders service API
	orders_url := os.Getenv("ORDERS_URL")
	url := fmt.Sprintf("%s/orders/%d/customers", orders_url, customer.ID)
	r, err := httpGet(url)
	if err != nil {
		fmt.Printf("Failed to fetch orders: %v\n", err)
		return resp, nil // return customer without orders if API fails
	}
	defer r.Body.Close()

	if r.StatusCode == http.StatusOK {
		var ordersResp dto.OrdersOutputBody
		if err := json.NewDecoder(r.Body).Decode(&ordersResp); err != nil {
			fmt.Printf("Failed to decode orders response: %v\n", err)
		} else {
			resp.Body.Orders = ordersResp.Orders
		}
	} else {
		fmt.Printf("Orders API returned status %d\n", r.StatusCode)
	}

	return resp, nil
}

// Create a new customer
func CreateCustomer(ctx context.Context, db *gorm.DB, ch *amqp.Channel, input *dto.CustomerCreateInput) (*dto.CustomerOutput, error) {
	resp := &dto.CustomerOutput{}

	firstname := cases.Title(language.English).String(input.Body.FirstName)
	lastname := strings.ToUpper(input.Body.LastName)

	customer := models.Customer{
		Username:  input.Body.Username,
		FirstName: firstname,
		LastName:  lastname,
		Name:      firstname + " " + lastname,
		Address:   input.Body.Address,
		Profile: models.Profile{
			LastName:  lastname,
			FirstName: firstname,
		},
		Company: input.Body.Company,
	}

	results := db.Create(&customer)

	if results.Error == nil {
		resp.Body = customer
		if ch != nil {
			_ = rabbitmq.PublishCustomerEvent(ch, events.CustomerCreated, customer) // ignore publish error
		}
	}

	return resp, results.Error
}

// Update/replace a customer
func UpdateCustomer(ctx context.Context, db *gorm.DB, ch *amqp.Channel, id uint, input dto.CustomerCreateInput) (*dto.CustomerOutput, error) {
	resp := &dto.CustomerOutput{}

	var customer models.Customer
	results := db.First(&customer, id)

	if errors.Is(results.Error, gorm.ErrRecordNotFound) {
		return nil, huma.NewError(http.StatusNotFound, "Customer not found")
	}
	if results.Error != nil {
		return nil, results.Error
	}

	firstname := cases.Title(language.English).String(input.Body.FirstName)
	lastname := strings.ToUpper(input.Body.LastName)

	updates := models.Customer{
		Username:  input.Body.Username,
		FirstName: firstname,
		LastName:  lastname,
		Name:      firstname + " " + lastname,
		Address:   input.Body.Address,
		Profile: models.Profile{
			LastName:  lastname,
			FirstName: firstname,
		},
		Company: input.Body.Company,
	}

	results = db.Model(&customer).Updates(updates)
	if results.Error != nil {
		return nil, results.Error
	}

	// Reload updated customer
	db.First(&customer, customer.ID)
	resp.Body = customer

	if ch != nil {
		_ = rabbitmq.PublishCustomerEvent(ch, events.CustomerUpdated, customer) // ignore publish error
	}

	return resp, nil
}

// Delete a customer
func DeleteCustomer(ctx context.Context, db *gorm.DB, ch *amqp.Channel, id uint) error {
	var customer models.Customer
	result := db.First(&customer, id)
	if result.Error != nil {
		return result.Error
	}

	results := db.Delete(&customer)
	if results.Error == nil {
		// Only publish if channel is not nil
		if ch != nil {
			_ = rabbitmq.PublishCustomerEvent(ch, events.CustomerDeleted, customer)
		}
		return nil
	}

	return results.Error
}

// ----------------------
// Register routes with Huma
// ----------------------
func RegisterCustomerRoutes(api huma.API, dbConn *gorm.DB, ch *amqp.Channel) {
	// ----------------------
	// Health endpoint
	// ----------------------
	huma.Register(api, huma.Operation{
		OperationID: "health",
		Summary:     "Health check endpoint",
		Method:      http.MethodGet,
		Path:        "/health",
		Tags:        []string{"health"},
	}, func(ctx context.Context, input *struct{}) (*struct {
		Status string `json:"status"`
	}, error) {
		return &struct {
			Status string `json:"status"`
		}{Status: "ok"}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-customers",
		Summary:     "Get all customers",
		Method:      http.MethodGet,
		Path:        "/customers",
		Tags:        []string{"customers"},
	}, func(ctx context.Context, input *struct{}) (*dto.CustomersOutput, error) {
		return GetCustomers(ctx, dbConn)
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-customer",
		Summary:     "Get a customer",
		Method:      http.MethodGet,
		Path:        "/customers/{id}",
		Tags:        []string{"customers"},
	}, func(ctx context.Context, input *struct {
		Id uint `path:"id"`
	}) (*dto.CustomerOutput, error) {
		return GetCustomer(ctx, dbConn, input.Id)
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-customer",
		Summary:       "Create a customer",
		Method:        http.MethodPost,
		DefaultStatus: http.StatusCreated,
		Path:          "/customers",
		Tags:          []string{"customers"},
	}, func(ctx context.Context, input *dto.CustomerCreateInput) (*dto.CustomerOutput, error) {
		return CreateCustomer(ctx, dbConn, ch, input)
	})

	huma.Register(api, huma.Operation{
		OperationID: "put-customer",
		Summary:     "Replace a customer",
		Method:      http.MethodPut,
		Path:        "/customers/{id}",
		Tags:        []string{"customers"},
	}, func(ctx context.Context, input *struct {
		Id uint `path:"id"`
		dto.CustomerCreateInput
	}) (*dto.CustomerOutput, error) {
		return UpdateCustomer(ctx, dbConn, ch, input.Id, input.CustomerCreateInput)
	})

	huma.Register(api, huma.Operation{
		OperationID:   "delete-customer",
		Summary:       "Delete a customer",
		Method:        http.MethodDelete,
		DefaultStatus: http.StatusNoContent,
		Path:          "/customers/{id}",
		Tags:          []string{"customers"},
	}, func(ctx context.Context, input *struct {
		Id uint `path:"id"`
	}) (*struct{}, error) {
		err := DeleteCustomer(ctx, dbConn, ch, input.Id)
		return &struct{}{}, err
	})
}
