package gemini

import (
	"os"
	"strings"

	"github.com/shout/ai-study-tool/backend/internal/domain"
)

type ClientCloser func()

func NewConfiguredClient(apiKey string) (domain.LLMClient, ClientCloser, error) {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("USE_GEMINI_MOCK")), "true") {
		client := NewMockClient()
		return client, client.Close, nil
	}

	client, err := NewClient(apiKey)
	if err != nil {
		return nil, nil, err
	}
	return client, client.Close, nil
}
