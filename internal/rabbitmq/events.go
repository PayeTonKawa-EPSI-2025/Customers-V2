package rabbitmq

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/PayeTonKawa-EPSI-2025/Common/events"
	"github.com/PayeTonKawa-EPSI-2025/Common/models"
	amqp "github.com/rabbitmq/amqp091-go"
)

// PublishCustomerEvent publishes a customer event to RabbitMQ
func PublishCustomerEvent(ch *amqp.Channel, eventType events.EventType, customer models.Customer) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	event := events.CustomerEvent{
		Type:      eventType,
		Customer:  customer,
		Timestamp: time.Now(),
	}

	body, err := json.Marshal(event)
	if err != nil {
		log.Printf("Error marshaling event: %v", err)
		return err
	}

	// Use a routing key based on the event type
	routingKey := string(eventType)

	err = ch.PublishWithContext(
		ctx,
		"events", // exchange
		routingKey,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)

	if err != nil {
		log.Printf("Error publishing message: %v", err)
		return err
	}

	log.Printf("Published %s event for customer %d", eventType, customer.ID)
	return nil
}
