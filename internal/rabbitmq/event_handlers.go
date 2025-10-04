package rabbitmq

import (
	"encoding/json"
	"log"
	"time"

	"github.com/PayeTonKawa-EPSI-2025/Common/models"
	"gorm.io/gorm"
)

// GenericEvent is a structure for parsing event data from other services
type GenericEvent struct {
	Type      string          `json:"type"`
	Data      json.RawMessage `json:"data"`
	Timestamp time.Time       `json:"timestamp"`
}

// OrderEvent represents an order event from the Orders service
type OrderEvent struct {
	Type      string       `json:"type"`
	Order     models.Order `json:"order"`
	Timestamp time.Time    `json:"timestamp"`
}

// ProductEvent represents a product event from the Products service
type ProductEvent struct {
	Type      string         `json:"type"`
	Product   models.Product `json:"product"`
	Timestamp time.Time      `json:"timestamp"`
}

// SetupEventHandlers configures handlers for different event types
func SetupEventHandlers(dbConn *gorm.DB) *EventRouter {
	router := NewEventRouter()

	// Handle order events (for example, when a new order is created)
	router.RegisterHandler("order.created", func(body []byte) error {
		var event OrderEvent
		if err := json.Unmarshal(body, &event); err != nil {
			log.Printf("Error unmarshaling order event: %v", err)
			return err
		}

		log.Printf("Received order.created event for order %d by customer %d",
			event.Order.ID, event.Order.CustomerID)

		// You could update customer statistics here, for example:
		// - Number of orders
		// - Last order date
		// - Total spent

		return nil
	})

	// Handle product events
	router.RegisterHandler("product.*", func(body []byte) error {
		var generic GenericEvent
		if err := json.Unmarshal(body, &generic); err != nil {
			log.Printf("Error unmarshaling generic event: %v", err)
			return err
		}

		log.Printf("Received product event of type %s", generic.Type)

		// Handle different product events based on the type
		switch generic.Type {
		case "product.created":
			var productEvent ProductEvent
			if err := json.Unmarshal(body, &productEvent); err != nil {
				log.Printf("Error unmarshaling product event: %v", err)
				return err
			}

			log.Printf("Product created: %s", productEvent.Product.Name)

		case "product.updated":
			var productEvent ProductEvent
			if err := json.Unmarshal(body, &productEvent); err != nil {
				log.Printf("Error unmarshaling product event: %v", err)
				return err
			}

			log.Printf("Product updated: %s", productEvent.Product.Name)

		case "product.deleted":
			var productEvent ProductEvent
			if err := json.Unmarshal(body, &productEvent); err != nil {
				log.Printf("Error unmarshaling product event: %v", err)
				return err
			}

			log.Printf("Product deleted: %s", productEvent.Product.Name)
		}

		return nil
	})

	// Catch-all handler for debugging - will receive all events
	// Useful during development, can be removed in production
	router.RegisterHandler("#", func(body []byte) error {
		var generic GenericEvent
		if err := json.Unmarshal(body, &generic); err != nil {
			log.Printf("Error unmarshaling generic event: %v", err)
			// Don't return error here as it might be a different format
			// Just log and continue
		} else {
			log.Printf("Received event of type %s", generic.Type)
		}

		// Log the raw message for debugging
		log.Printf("Raw event: %s", string(body))
		return nil
	})

	return router
}
