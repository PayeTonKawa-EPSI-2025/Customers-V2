package operation

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/PayeTonKawa-EPSI-2025/Common/models"
	"github.com/PayeTonKawa-EPSI-2025/Customers/internal/dto"
	"github.com/PayeTonKawa-EPSI-2025/Customers/internal/rabbitmq"
	"github.com/danielgtaylor/huma/v2"
	amqp "github.com/rabbitmq/amqp091-go"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gorm.io/gorm"
)

func RegisterCustomerRoutes(api huma.API, dbConn *gorm.DB, ch *amqp.Channel) {

	huma.Register(api, huma.Operation{
		OperationID: "get-customers",
		Summary:     "Get all customers",
		Method:      http.MethodGet,
		Path:        "/customers",
		Tags:        []string{"customers"},
	}, func(ctx context.Context, input *struct{}) (*dto.CustomersOutput, error) {
		resp := &dto.CustomersOutput{}

		var customers []models.Customer
		results := dbConn.Find(&customers)

		if results.Error == nil {
			resp.Body.Customers = customers
		}

		return resp, results.Error
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
		resp := &dto.CustomerOutput{}

		var customer models.Customer
		results := dbConn.First(&customer, input.Id)

		if results.Error == nil {
			resp.Body = customer
			return resp, nil
		}

		if errors.Is(results.Error, gorm.ErrRecordNotFound) {
			return nil, huma.NewError(http.StatusNotFound, "Customer not found")
		}

		return nil, results.Error
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-customer",
		Summary:       "Create a customer",
		Method:        http.MethodPost,
		DefaultStatus: http.StatusCreated,
		Path:          "/customers",
		Tags:          []string{"customers"},
	}, func(ctx context.Context, input *dto.CustomerCreateInput) (*dto.CustomerOutput, error) {
		resp := &dto.CustomerOutput{}

		firstname := cases.Title(language.English).String(input.Body.FirstName)
		lastname := strings.ToUpper(input.Body.LastName)

		customer := models.Customer{
			Username:  input.Body.Username,
			FirstName: firstname,
			LastName:  lastname,
			Name:      firstname + " " + lastname,

			Address: input.Body.Address,
			Profile: models.Profile{
				LastName:  lastname,
				FirstName: firstname,
			},
			Company: input.Body.Company,
		}

		results := dbConn.Create(&customer)

		if results.Error == nil {
			resp.Body = customer

			// Publish customer created event
			err := rabbitmq.PublishCustomerEvent(ch, rabbitmq.CustomerCreated, customer)
			if err != nil {
				// Log the error but don't fail the request
				// The customer was already created in the database
				return resp, nil
			}
		}

		return resp, results.Error
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
		resp := &dto.CustomerOutput{}

		var customer models.Customer
		results := dbConn.First(&customer, input.Id)

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

			Address: input.Body.Address,
			Profile: models.Profile{
				LastName:  lastname,
				FirstName: firstname,
			},
			Company: input.Body.Company,
		}

		results = dbConn.Model(&customer).Updates(updates)
		if results.Error != nil {
			return nil, results.Error
		}

		// Get updated customer from DB to ensure all fields are correct
		dbConn.First(&customer, customer.ID)
		resp.Body = customer

		// Publish customer updated event
		err := rabbitmq.PublishCustomerEvent(ch, rabbitmq.CustomerUpdated, customer)
		if err != nil {
			// Log the error but don't fail the request
			// The customer was already updated in the database
		}

		return resp, nil
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
		resp := &struct{}{}

		// First get the customer to have the complete data for the event
		var customer models.Customer
		result := dbConn.First(&customer, input.Id)

		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, huma.NewError(http.StatusNotFound, "Customer not found")
		}

		if result.Error != nil {
			return nil, result.Error
		}

		results := dbConn.Delete(&customer)

		if results.Error == nil {
			// Publish customer deleted event
			err := rabbitmq.PublishCustomerEvent(ch, rabbitmq.CustomerDeleted, customer)
			if err != nil {
				// Log the error but don't fail the request
				// The customer was already deleted from the database
			}

			return resp, nil
		}

		return nil, results.Error
	})
}
