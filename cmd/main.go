package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/PayeTonKawa-EPSI-2025/Customers-V2/internal/db"
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

// Options for the CLI.
type Options struct {
	Port int `help:"Port to listen on" short:"p" default:"8081"`
}

var (
	dbConn *gorm.DB
)

func main() {
	_ = godotenv.Load()
	dbConn = db.Init()

	disableRabbit := os.Getenv("DISABLE_RABBITMQ") == "true"

	if disableRabbit {
		log.Println("DISABLE_RABBITMQ=true → starting CI server")

		router := chi.NewMux()

		// Health endpoint (used by GitHub Actions)
		router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		})

		// Register API routes WITHOUT RabbitMQ
		configs := huma.DefaultConfig("Customers CI", "1.0.0")
		api := humachi.New(router, configs)
		operation.RegisterCustomerRoutes(api, dbConn, nil)

		server := &http.Server{
			Addr:    ":8080",
			Handler: router,
		}

		go func() {
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("CI server failed: %v", err)
			}
		}()

		// Keep process alive for Postman tests
		select {}
	}

	var conn *amqp.Connection
	var ch *amqp.Channel
	conn, ch = rabbitmq.Connect()

	// Set up event handlers
	eventRouter := rabbitmq.SetupEventHandlers(dbConn)

	// Start listening for events
	_, err := rabbitmq.StartListening(ch, eventRouter)
	if err != nil {
		log.Fatalf("Failed to start event listener: %v", err)
	}

	// Create a CLI app which takes a port option.
	cli := humacli.New(func(hooks humacli.Hooks, options *Options) {
		// Create a new router & API
		router := chi.NewMux()

		router.Use(middleware.Logger)
		router.Use(middleware.Recoverer)
		router.Use(middleware.Compress(5))

		// Prometheus instrumentation (counter + histogram)
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

		// middleware to observe requests
		prometheusMiddleware := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				start := time.Now()
				rw := &statusRecorder{ResponseWriter: w, status: 200}
				next.ServeHTTP(rw, r)
				dur := time.Since(start).Seconds()
				path := r.URL.Path
				httpRequests.WithLabelValues(path, r.Method, fmt.Sprintf("%d", rw.status)).Inc()
				httpRequestDuration.WithLabelValues(path, r.Method).Observe(dur)
			})
		}

		router.Use(prometheusMiddleware)
		router.Handle("/metrics", promhttp.Handler())

		configs := huma.DefaultConfig("Paye Ton Kawa - Customers", "1.0.0")
		api := humachi.New(router, configs)

		operation.RegisterCustomerRoutes(api, dbConn, ch)

		// Debug endpoints for testing/metrics
		router.HandleFunc("/debug/500", func(w http.ResponseWriter, r *http.Request) {
			// return an internal server error to create 5xx metrics
			http.Error(w, "debug 500", http.StatusInternalServerError)
		})

		// Create the HTTP server.
		server := http.Server{
			Addr:    fmt.Sprintf(":%d", options.Port),
			Handler: router,
		}

		// Tell the CLI how to start your router.
		hooks.OnStart(func() {
			server.ListenAndServe()
		})

		// Tell the CLI how to stop your server.
		hooks.OnStop(func() {
			// Give the server 5 seconds to gracefully shut down, then give up.
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			server.Shutdown(ctx)

			// Close the RabbitMQ connection when server shuts down
			conn.Close()
			ch.Close()
		})
	})

	// Run the CLI. When passed no commands, it starts the server.
	cli.Run()
}

// statusRecorder is a small helper to capture HTTP status codes from handlers.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}
