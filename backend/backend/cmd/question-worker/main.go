package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	dbinfra "github.com/shout/ai-study-tool/backend/internal/infrastructure/db"
	"github.com/shout/ai-study-tool/backend/internal/infrastructure/gemini"
	"github.com/shout/ai-study-tool/backend/internal/infrastructure/persistence"
	"github.com/shout/ai-study-tool/backend/internal/logging"
	"github.com/shout/ai-study-tool/backend/internal/usecase"
)

const defaultPollInterval = 10 * time.Minute

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

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}
	configureDatabasePool(db)

	llmClient, closeLLMClient, err := gemini.NewConfiguredClient(os.Getenv("GEMINI_API_KEY"))
	if err != nil {
		log.Fatalf("failed to create gemini client: %v", err)
	}
	defer closeLLMClient()

	highlightRepo := persistence.NewHighlightRepository(db)
	questionRepo := persistence.NewQuestionRepository(db)
	worker := usecase.NewQuestionWorkerUsecase(highlightRepo, questionRepo, llmClient)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if os.Getenv("WORKER_RUN_ONCE") == "1" {
		if err := worker.RunOnce(ctx); err != nil {
			log.Fatalf("question worker run failed: %v", err)
		}
		return
	}

	pollInterval := readPollInterval()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	log.Printf("question worker started with poll interval %s", pollInterval)
	for {
		if err := worker.RunOnce(ctx); err != nil {
			log.Printf("question worker run error: %v", err)
		}
		select {
		case <-ctx.Done():
			log.Println("question worker shutting down")
			return
		case <-ticker.C:
		}
	}
}

func readPollInterval() time.Duration {
	raw := os.Getenv("QUESTION_WORKER_POLL_INTERVAL_SECONDS")
	if raw == "" {
		return defaultPollInterval
	}

	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		return defaultPollInterval
	}

	return time.Duration(seconds) * time.Second
}

func configureDatabasePool(db *sql.DB) {
	db.SetMaxOpenConns(readEnvInt("DB_MAX_OPEN_CONNS", 10))
	db.SetMaxIdleConns(readEnvInt("DB_MAX_IDLE_CONNS", 5))
	db.SetConnMaxLifetime(time.Duration(readEnvInt("DB_CONN_MAX_LIFETIME_SECONDS", 1800)) * time.Second)
	db.SetConnMaxIdleTime(time.Duration(readEnvInt("DB_CONN_MAX_IDLE_SECONDS", 300)) * time.Second)
}

func readEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}
