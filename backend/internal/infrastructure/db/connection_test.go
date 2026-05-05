package db

import (
	"os"
	"testing"
)

func TestValidateTLS(t *testing.T) {
	tests := []struct {
		name    string
		appEnv  string
		url     string
		wantErr bool
	}{
		{
			name:    "production with sslmode=require passes",
			appEnv:  "production",
			url:     "postgres://user:pass@host/db?sslmode=require",
			wantErr: false,
		},
		{
			name:    "production with sslmode=verify-full passes",
			appEnv:  "production",
			url:     "postgres://user:pass@host/db?sslmode=verify-full",
			wantErr: false,
		},
		{
			name:    "production with sslmode=disable fails",
			appEnv:  "production",
			url:     "postgres://user:pass@host/db?sslmode=disable",
			wantErr: true,
		},
		{
			name:    "production with no sslmode fails",
			appEnv:  "production",
			url:     "postgres://user:pass@host/db",
			wantErr: true,
		},
		{
			name:    "development with sslmode=disable passes",
			appEnv:  "development",
			url:     "postgres://user:pass@host/db?sslmode=disable",
			wantErr: false,
		},
		{
			name:    "no APP_ENV with sslmode=disable passes",
			appEnv:  "",
			url:     "postgres://user:pass@host/db?sslmode=disable",
			wantErr: false,
		},
		{
			name:    "invalid URL returns error in production",
			appEnv:  "production",
			url:     "://invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := os.Getenv("APP_ENV")
			os.Setenv("APP_ENV", tt.appEnv)
			defer os.Setenv("APP_ENV", original)

			err := validateTLS(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTLS() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
