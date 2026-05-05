package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shout/ai-study-tool/backend/internal/domain"
)

const (
	ModelDefault = "gemini-2.5-flash-lite"

	defaultTimeoutSeconds = 90
	defaultMaxRetries     = 2
	defaultRetryBaseDelay = 1500 * time.Millisecond
)

type Client struct {
	apiKey     string
	httpClient *http.Client
	maxRetries int
	retryDelay time.Duration
}

func NewClient(apiKey string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("gemini: api key is required")
	}
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: readEnvDurationSeconds("GEMINI_TIMEOUT_SECONDS", defaultTimeoutSeconds),
		},
		maxRetries: readEnvInt("GEMINI_MAX_RETRIES", defaultMaxRetries),
		retryDelay: readEnvDurationMilliseconds("GEMINI_RETRY_BASE_DELAY_MS", defaultRetryBaseDelay),
	}, nil
}

func (c *Client) Close() {
}

func ModelForPlan(plan string) string {
	return ModelDefault
}

func (c *Client) ModelForPlan(plan string) string {
	return ModelForPlan(plan)
}

func (c *Client) GenerateQuestions(ctx context.Context, points []domain.ExtractedPoint, questionType domain.QuestionType, customInstruction string, model string) ([]domain.GeneratedQuestion, error) {
	prompt := BuildBatchGeneratorPrompt(points, questionType, customInstruction)
	resp, err := c.generate(ctx, model, prompt)
	if err != nil {
		return nil, fmt.Errorf("gemini: generate questions failed: %w", err)
	}

	var result struct {
		Questions []struct {
			Content       string   `json:"content"`
			Options       []string `json:"options"`
			CorrectAnswer string   `json:"correct_answer"`
			Explanation   string   `json:"explanation"`
		} `json:"questions"`
	}
	if err := parseJSON(resp, &result); err != nil {
		return nil, fmt.Errorf("gemini: failed to parse generate questions response: %w", err)
	}

	questions := make([]domain.GeneratedQuestion, 0, len(result.Questions))
	for _, question := range result.Questions {
		questions = append(questions, domain.GeneratedQuestion{
			Content:       question.Content,
			Options:       question.Options,
			CorrectAnswer: question.CorrectAnswer,
			Explanation:   question.Explanation,
		})
	}

	return questions, nil
}

func (c *Client) generate(ctx context.Context, model string, prompt string) (string, error) {
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		resp, retryable, err := c.generateOnce(ctx, model, prompt)
		if err == nil {
			return resp, nil
		}

		lastErr = err
		if !retryable || attempt == c.maxRetries {
			break
		}

		if err := waitRetry(ctx, c.retryDelay*time.Duration(attempt+1)); err != nil {
			return "", fmt.Errorf("gemini: request canceled while waiting to retry: %w", err)
		}
	}

	return "", lastErr
}

func (c *Client) generateOnce(ctx context.Context, model string, prompt string) (string, bool, error) {
	payload := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature": 0.7,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", false, fmt.Errorf("gemini: marshal request: %w", err)
	}

	endpoint := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		url.PathEscape(model),
		url.QueryEscape(c.apiKey),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", false, fmt.Errorf("gemini: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", isRetryableTransportError(err), fmt.Errorf("gemini: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, fmt.Errorf("gemini: read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", isRetryableStatus(resp.StatusCode), fmt.Errorf("gemini: status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		if os.Getenv("DEBUG_GEMINI_RAW") != "" {
			return "", false, fmt.Errorf("gemini: decode response: %w: %s", err, string(respBody))
		}
		return "", false, fmt.Errorf("gemini: decode response: %w", err)
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", false, fmt.Errorf("gemini: empty response")
	}

	var sb strings.Builder
	for _, part := range result.Candidates[0].Content.Parts {
		sb.WriteString(part.Text)
	}
	return sb.String(), false, nil
}

func parseJSON(raw string, v interface{}) error {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)
	return json.Unmarshal([]byte(raw), v)
}

func readEnvInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return fallback
	}
	return value
}

func readEnvDurationSeconds(key string, fallbackSeconds int) time.Duration {
	seconds := readEnvInt(key, fallbackSeconds)
	if seconds <= 0 {
		seconds = fallbackSeconds
	}
	return time.Duration(seconds) * time.Second
}

func readEnvDurationMilliseconds(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return time.Duration(value) * time.Millisecond
}

func isRetryableTransportError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func isRetryableStatus(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode >= http.StatusInternalServerError
}

func waitRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
