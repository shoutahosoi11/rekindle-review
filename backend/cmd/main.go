package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/shout/ai-study-tool/backend/internal/di"
	dbinfra "github.com/shout/ai-study-tool/backend/internal/infrastructure/db"
	"github.com/shout/ai-study-tool/backend/internal/logging"
	"github.com/shout/ai-study-tool/backend/internal/router"
)

const (
	readinessTimeout = 2 * time.Second
	shutdownTimeout  = 10 * time.Second
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	logging.Setup(os.Getenv("APP_ENV"))

	db, err := dbinfra.Open(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	configureDatabasePool(db)

	container, err := di.NewContainer(db)
	if err != nil {
		log.Fatalf("failed to build DI container: %v", err)
	}
	defer container.Close()

	e := echo.New()
	e.Use(echomiddleware.Logger())
	e.Use(echomiddleware.Recover())
	e.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		AllowOrigins: allowedOrigins(),
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Authorization", "Content-Type"},
	}))

	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	e.GET("/ready", readinessHandler(db))

	router.RegisterAPI(e, container)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- e.Start(":" + port)
	}()

	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	case <-signalCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := e.Shutdown(shutdownCtx); err != nil {
			log.Printf("server shutdown error: %v", err)
		}
	}
}

func readinessHandler(db *sql.DB) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx, cancel := context.WithTimeout(c.Request().Context(), readinessTimeout)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			log.Printf("readiness check failed: %v", err)
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		}

		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}
}

func allowedOrigins() []string {
	raw := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if raw == "" {
		return []string{"http://localhost:3000", "http://127.0.0.1:3000"}
	}

	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin != "" {
			origins = append(origins, origin)
		}
	}
	if len(origins) == 0 {
		return []string{"http://localhost:3000", "http://127.0.0.1:3000"}
	}

	return origins
}

func configureDatabasePool(db *sql.DB) {
	db.SetMaxOpenConns(readEnvInt("DB_MAX_OPEN_CONNS", 10))
	db.SetMaxIdleConns(readEnvInt("DB_MAX_IDLE_CONNS", 5))
	db.SetConnMaxLifetime(time.Duration(readEnvInt("DB_CONN_MAX_LIFETIME_SECONDS", 1800)) * time.Second)
	db.SetConnMaxIdleTime(time.Duration(readEnvInt("DB_CONN_MAX_IDLE_SECONDS", 300)) * time.Second)
}

func readEnvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}