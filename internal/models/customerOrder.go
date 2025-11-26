package models

import "github.com/PayeTonKawa-EPSI-2025/Common-V2/models"

type CustomerOrder struct {
	ID         uint            `json:"id" gorm:"primaryKey"`
	CustomerID uint            `json:"customerId"`
	Customer   models.Customer `gorm:"foreignKey:CustomerID"`
	OrderID    uint            `json:"orderId"`
	Order      Order           `gorm:"foreignKey:OrderID"`
}
