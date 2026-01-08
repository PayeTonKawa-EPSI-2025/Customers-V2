package operation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
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
	"github.com/PayeTonKawa-EPSI-2025/Common-V2/auth"

)

// ----------------------
// Helper functions
// ----------------------

// Extract and verify token from Authorization header
func extractAndVerifyToken(ctx context.Context, authHeader string) (*auth.Claims, error) {
	if authHeader == "" {
		return nil, huma.NewError(http.StatusUnauthorized, "Missing authorization header")
	}

	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, huma.NewError(http.StatusUnauthorized, "Invalid authorization format")
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")

	idToken, err := auth.Verifier.Verify(ctx, token)
	if err != nil {
		return nil, huma.NewError(http.StatusUnauthorized, "Invalid token")
	}

	var claims auth.Claims
	if err := idToken.Claims(&claims); err != nil {
		return nil, huma.NewError(http.StatusUnauthorized, "Invalid token claims")
	}

	claims.Normalize()
	return &claims, nil
}

// Check if user has a specific role
func hasRole(claims *auth.Claims, role string) bool {
	return slices.Contains(claims.Roles, role)
}


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
func GetCustomer(ctx context.Context, db *gorm.DB, id uint, claims *auth.Claims) (*dto.CustomerOutput, error) {
	resp := &dto.CustomerOutput{}

	// Fetch customer from local DB
	var customer models.Customer
	results := db.First(&customer, id)
	if results.Error != nil {
		if errors.Is(results.Error, gorm.ErrRecordNotFound) {
			return nil, huma.NewError(http.StatusNotFound, "Customer not found")
		}
		return nil, results.Error
	}


	// Check authorization: admin or own customer
	isAdmin := hasRole(claims, "admin")
	if !isAdmin && customer.Username != claims.PreferredUsername {
		return nil, huma.NewError(http.StatusForbidden, "You can only access your own customer data")
	}

	// Assign basic customer data
	resp.Body = customer

	// Fetch orders from Orders service API
	orders_url := os.Getenv("ORDERS_URL")
	url := fmt.Sprintf("%s/orders/%d/customers", orders_url, customer.ID)
	r, err := http.Get(url)
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
func UpdateCustomer(ctx context.Context, db *gorm.DB, ch *amqp.Channel, id uint, input dto.CustomerCreateInput, claims *auth.Claims) (*dto.CustomerOutput, error) {
	resp := &dto.CustomerOutput{}

	var customer models.Customer
	results := db.First(&customer, id)

	if errors.Is(results.Error, gorm.ErrRecordNotFound) {
		return nil, huma.NewError(http.StatusNotFound, "Customer not found")
	}
	if results.Error != nil {
		return nil, results.Error
	}

	// Check authorization: admin or own customer
	isAdmin := hasRole(claims, "admin")
    if !isAdmin && customer.Username != claims.PreferredUsername {
        return nil, huma.NewError(http.StatusForbidden, "You can only update your own customer data")
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

	huma.Register(api, huma.Operation{
		OperationID: "get-customers",
		Summary:     "Get all customers",
		Method:      http.MethodGet,
		Path:        "/customers",
		Tags:        []string{"customers"},
		 Security: []map[string][]string{
        {"bearer": {}},
    },
	}, func(ctx context.Context, input *struct{
		Authorization string `header:"Authorization"`
		}) (*dto.CustomersOutput, error) {

		claims, err := extractAndVerifyToken(ctx, input.Authorization)
		if err != nil {
			return nil, err
		}
		
		if !hasRole(claims, "admin") {
			return nil, huma.NewError(http.StatusForbidden, "Admin access required")
		}
		return GetCustomers(ctx, dbConn)
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-customer",
		Summary:     "Get a customer",
		Method:      http.MethodGet,
		Path:        "/customers/{id}",
		Tags:        []string{"customers"},
		Security: []map[string][]string{
			{"bearer": {}},
		},
	}, func(ctx context.Context, input *struct 
		{
		Authorization string `header:"Authorization"`	
		Id uint `path:"id"`
	}) (*dto.CustomerOutput, error) {
		claims, err := extractAndVerifyToken(ctx, input.Authorization)
		if err != nil {
			return nil, err
		}
		return GetCustomer(ctx, dbConn, input.Id, claims)
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-customer",
		Summary:       "Create a customer",
		Method:        http.MethodPost,
		DefaultStatus: http.StatusCreated,
		Path:          "/customers",
		Tags:          []string{"customers"},
		Security: []map[string][]string{
			{"bearer": {}},
		},
	}, func(ctx context.Context, input *struct {
		Authorization string `header:"Authorization"`
		dto.CustomerCreateInput}) (*dto.CustomerOutput, error) {
		claims, err := extractAndVerifyToken(ctx, input.Authorization)
		if err != nil {
			return nil, err
		}

		// Check admin role
		if !hasRole(claims, "admin") {
			return nil, huma.NewError(http.StatusForbidden, "Admin access required")
		}
		return CreateCustomer(ctx, dbConn, ch, &input.CustomerCreateInput)
	})

	huma.Register(api, huma.Operation{
		OperationID: "put-customer",
		Summary:     "Replace a customer",
		Method:      http.MethodPut,
		Path:        "/customers/{id}",
		Tags:        []string{"customers"},
		Security: []map[string][]string{
			{"bearer": {}},
		},
	}, func(ctx context.Context, input *struct {
		Authorization string `header:"Authorization"`
		Id uint `path:"id"`
		dto.CustomerCreateInput
	}) (*dto.CustomerOutput, error) {
		claims, err := extractAndVerifyToken(ctx, input.Authorization)
		if err != nil {
			return nil, err
		}
		return UpdateCustomer(ctx, dbConn, ch, input.Id, input.CustomerCreateInput, claims)
	})

	huma.Register(api, huma.Operation{
		OperationID:   "delete-customer",
		Summary:       "Delete a customer",
		Method:        http.MethodDelete,
		DefaultStatus: http.StatusNoContent,
		Path:          "/customers/{id}",
		Tags:          []string{"customers"},
		Security: []map[string][]string{
			{"bearer": {}},
		},
	}, func(ctx context.Context, input *struct {
		Authorization string `header:"Authorization"`
		Id uint `path:"id"`
	}) (*struct{}, error) {
		claims, err := extractAndVerifyToken(ctx, input.Authorization)
		if err != nil {
			return &struct{}{}, err
		}

		// Check admin role
		if !hasRole(claims, "admin") {
			return &struct{}{}, huma.NewError(http.StatusForbidden, "Admin access required")
		}

		err = DeleteCustomer(ctx, dbConn, ch, input.Id)
		return &struct{}{}, err
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
}
