package rabbitmq

import (
	"github.com/PayeTonKawa-EPSI-2025/Customers-V2/internal/rabbitmq/event_handlers"
	"gorm.io/gorm"
)

// SetupEventHandlers configures handlers for different event types
func SetupEventHandlers(dbConn *gorm.DB) *EventRouter {
	router := NewEventRouter()

	// Initialize event handlers
	orderHandlers := event_handlers.NewOrderEventHandlers(dbConn)
	productHandlers := event_handlers.NewProductEventHandlers(dbConn)
	debugHandlers := event_handlers.NewDebugEventHandlers()

	// Register order event handlers
	router.RegisterHandler("order.created", orderHandlers.HandleOrderCreated)
	router.RegisterHandler("order.updated", orderHandlers.HandleOrderUpdated)
	router.RegisterHandler("order.deleted", orderHandlers.HandleOrderDeleted)

	// Register product event handlers
	router.RegisterHandler("product.created", productHandlers.HandleProductCreated)
	router.RegisterHandler("product.updated", productHandlers.HandleProductUpdated)
	router.RegisterHandler("product.deleted", productHandlers.HandleProductDeleted)

	// Register debug catch-all handler
	// Useful during development, can be removed in production
	router.RegisterHandler("#", debugHandlers.HandleAllEvents)

	return router
}
