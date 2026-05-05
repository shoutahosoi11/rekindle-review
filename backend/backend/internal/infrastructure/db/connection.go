package db

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"

	_ "github.com/lib/pq"
)

// Open validates TLS settings and returns a *sql.DB.
// When APP_ENV=production, sslmode must be require, verify-ca, or verify-full.
func Open(databaseURL string) (*sql.DB, error) {
	if err := validateTLS(databaseURL); err != nil {
		return nil, err
	}
	return sql.Open("postgres", databaseURL)
}

func validateTLS(databaseURL string) error {
	if os.Getenv("APP_ENV") != "production" {
		return nil
	}

	u, err := url.Parse(databaseURL)
	if err != nil {
		return fmt.Errorf("db: invalid DATABASE_URL: %w", err)
	}

	sslmode := strings.ToLower(u.Query().Get("sslmode"))
	switch sslmode {
	case "require", "verify-ca", "verify-full":
		return nil
	case "disable":
		return fmt.Errorf("db: sslmode=disable is not allowed in production")
	default:
		return fmt.Errorf("db: sslmode must be set to 'require' or higher in production (got: %q)", sslmode)
	}
}
