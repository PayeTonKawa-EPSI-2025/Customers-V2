package event_handlers

import (
	"encoding/json"
	"log"

	"github.com/PayeTonKawa-EPSI-2025/Common/events"
	localModels "github.com/PayeTonKawa-EPSI-2025/Customers/internal/models"
	"gorm.io/gorm"
)

// OrderEventHandlers provides handlers for order-related events
type OrderEventHandlers struct {
	db *gorm.DB
}

// NewOrderEventHandlers creates a new order event handlers instance
func NewOrderEventHandlers(db *gorm.DB) *OrderEventHandlers {
	return &OrderEventHandlers{db: db}
}

// HandleOrderCreated handles the order.created event
func (h *OrderEventHandlers) HandleOrderCreated(body []byte) error {
	var event events.OrderEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.Printf("Error unmarshaling order.created event: %v", err)
		return err
	}

	log.Printf("Received order.created event for order %d", event.Order.OrderID)

	// Create the order in the local database
	order := localModels.Order{}
	order.ID = event.Order.OrderID

	if err := h.db.Create(&order).Error; err != nil {
		log.Printf("Error creating order in DB: %v", err)
		return err
	}

	log.Printf("Successfully created order %d in local database", order.ID)

	customerOrder := localModels.CustomerOrder{}
	customerOrder.CustomerID = event.Order.CustomerID
	customerOrder.OrderID = event.Order.OrderID

	if err := h.db.Create(&customerOrder).Error; err != nil {
		log.Printf("Error creating customer order in DB: %v", err)
		return err
	}

	log.Printf("Successfully created customer order for customer %d and order %d in local database", event.Order.CustomerID, order.ID)

	return nil
}

// HandleOrderUpdated handles the order.updated event
func (h *OrderEventHandlers) HandleOrderUpdated(body []byte) error {
	var event events.OrderEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.Printf("Error unmarshaling order.updated event: %v", err)
		return err
	}

	log.Printf("Received order.updated event for order %d", event.Order.OrderID)

	// Update the order in the local database
	order := localModels.Order{}
	order.ID = event.Order.OrderID

	if err := h.db.Save(&order).Error; err != nil {
		log.Printf("Error updating order in DB: %v", err)
		return err
	}

	log.Printf("Successfully updated order %d in local database", order.ID)
	return nil
}

// HandleOrderDeleted handles the order.deleted event
func (h *OrderEventHandlers) HandleOrderDeleted(body []byte) error {
	var event events.OrderEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.Printf("Error unmarshaling order.deleted event: %v", err)
		return err
	}

	log.Printf("Received order.deleted event for order %d", event.Order.OrderID)

	// Delete the order from the local database
	if err := h.db.Delete(&localModels.Order{}, event.Order.OrderID).Error; err != nil {
		log.Printf("Error deleting order from DB: %v", err)
		return err
	}

	log.Printf("Successfully deleted order %d from local database", event.Order.OrderID)
	return nil
}
