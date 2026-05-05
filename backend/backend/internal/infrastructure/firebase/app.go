package firebase

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"google.golang.org/api/option"
)

func NewApp(ctx context.Context, credPath string) (*firebase.App, error) {
	if credPath != "" {
		app, err := firebase.NewApp(ctx, nil, option.WithCredentialsFile(credPath))
		if err != nil {
			return nil, fmt.Errorf("firebase: init app with credentials file: %w", err)
		}
		return app, nil
	}

	app, err := firebase.NewApp(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("firebase: init app with application default credentials: %w", err)
	}
	return app, nil
}
