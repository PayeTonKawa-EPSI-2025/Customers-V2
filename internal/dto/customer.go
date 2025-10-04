package dto

import "github.com/PayeTonKawa-EPSI-2025/Common/models"

type CustomersOutput struct {
	Body struct {
		Customers []models.Customer `json:"customers"`
	}
}

type CustomerOutput struct {
	Body models.Customer
}

type CustomerCreateInput struct {
	Body struct {
		Username  string `json:"username"`
		FirstName string `json:"firstname"`
		LastName  string `json:"lastname"`

		Address models.Address `json:"address"`
		Company models.Company `json:"company"`
	}
}
