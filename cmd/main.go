package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/PayeTonKawa-EPSI-2025/Customers-V2/internal/db"
	"github.com/PayeTonKawa-EPSI-2025/Customers-V2/internal/operation"
	"github.com/PayeTonKawa-EPSI-2025/Customers-V2/internal/rabbitmq"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/danielgtaylor/huma/v2/humacli"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	amqp "github.com/rabbitmq/amqp091-go"
	"gorm.io/gorm"
)

// Options for CLI
type Options struct {
	Port int `help:"Port to listen on" short:"p" default:"8081"`
}

var dbConn *gorm.DB

func main() {
	_ = godotenv.Load()
	dbConn = db.Init()

	// RabbitMQ setup
	var conn *amqp.Connection
	var ch *amqp.Channel
	disableRabbit := os.Getenv("DISABLE_RABBITMQ") == "true"

	if !disableRabbit {
		conn, ch = rabbitmq.Connect()
		eventRouter := rabbitmq.SetupEventHandlers(dbConn)
		go func() {
			if _, err := rabbitmq.StartListening(ch, eventRouter); err != nil {
				log.Fatalf("Failed to start event listener: %v", err)
			}
		}()
	} else {
		log.Println("DISABLE_RABBITMQ=true, skipping RabbitMQ connection")
	}

	// CLI & API setup
	cli := humacli.New(func(hooks humacli.Hooks, options *Options) {
		router := chi.NewMux()
		router.Use(middleware.Logger)
		router.Use(middleware.Recoverer)
		router.Use(middleware.Compress(5))

		// Prometheus metrics
		httpRequests := prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total HTTP requests",
			},
			[]string{"path", "method", "status"},
		)

		httpRequestDuration := prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"path", "method"},
		)

		prometheus.MustRegister(httpRequests, httpRequestDuration)

		prometheusMiddleware := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				start := time.Now()
				rw := &statusRecorder{ResponseWriter: w, status: 200}
				next.ServeHTTP(rw, r)
				dur := time.Since(start).Seconds()
				httpRequests.WithLabelValues(r.URL.Path, r.Method, fmt.Sprintf("%d", rw.status)).Inc()
				httpRequestDuration.WithLabelValues(r.URL.Path, r.Method).Observe(dur)
			})
		}

		router.Use(prometheusMiddleware)
		router.Handle("/metrics", promhttp.Handler())

		// Huma API
		configs := huma.DefaultConfig("Paye Ton Kawa - Customers", "1.0.0")
		api := humachi.New(router, configs)
		operation.RegisterCustomerRoutes(api, dbConn, ch)

		// Debug endpoint
		router.HandleFunc("/debug/500", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "debug 500", http.StatusInternalServerError)
		})

		// HTTP server
		server := &http.Server{
			Addr:    fmt.Sprintf(":%d", options.Port),
			Handler: router,
		}

		// OnStart: blocking ListenAndServe
		hooks.OnStart(func() {
			log.Printf("Starting server on port %d...", options.Port)
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("HTTP server failed: %v", err)
			}
		})

		// OnStop: graceful shutdown
		hooks.OnStop(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := server.Shutdown(ctx); err != nil {
				log.Printf("Server shutdown error: %v", err)
			}
			if conn != nil {
				_ = conn.Close()
			}
			if ch != nil {
				_ = ch.Close()
			}
		})
	})

	// Run CLI (starts server and blocks)
	cli.Run()
}

// statusRecorder to capture HTTP status codes
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}
