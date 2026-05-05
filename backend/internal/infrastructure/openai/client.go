package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shout/ai-study-tool/backend/internal/domain"
)

const (
	ModelDefault = "gpt-4.1-mini"
	ModelPro     = "gpt-4.1"

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
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("openai: api key is required")
	}

	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: readEnvDurationSeconds("OPENAI_TIMEOUT_SECONDS", defaultTimeoutSeconds),
		},
		maxRetries: readEnvInt("OPENAI_MAX_RETRIES", defaultMaxRetries),
		retryDelay: readEnvDurationMilliseconds("OPENAI_RETRY_BASE_DELAY_MS", defaultRetryBaseDelay),
	}, nil
}

func (c *Client) Close() {
}

func ModelForPlan(plan string) string {
	if plan == "pro" {
		return readEnvString("OPENAI_MODEL_PRO", ModelPro)
	}
	return readEnvString("OPENAI_MODEL_DEFAULT", ModelDefault)
}

func (c *Client) ModelForPlan(plan string) string {
	return ModelForPlan(plan)
}

func (c *Client) GenerateQuestions(ctx context.Context, points []domain.ExtractedPoint, questionType domain.QuestionType, customInstruction string, model string) ([]domain.GeneratedQuestion, error) {
	prompt := BuildBatchGeneratorPrompt(points, questionType, customInstruction)

	resp, err := c.generate(ctx, model, prompt, generatedQuestionsSchema())
	if err != nil {
		return nil, fmt.Errorf("openai: generate questions failed: %w", err)
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
		return nil, fmt.Errorf("openai: failed to parse generate questions response: %w", err)
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

func (c *Client) generate(ctx context.Context, model string, prompt string, schema map[string]any) (string, error) {
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		resp, retryable, err := c.generateOnce(ctx, model, prompt, schema)
		if err == nil {
			return resp, nil
		}

		lastErr = err
		if !retryable || attempt == c.maxRetries {
			break
		}

		if err := waitRetry(ctx, c.retryDelay*time.Duration(attempt+1)); err != nil {
			return "", fmt.Errorf("openai: request canceled while waiting to retry: %w", err)
		}
	}

	return "", lastErr
}

func (c *Client) generateOnce(ctx context.Context, model string, prompt string, schema map[string]any) (string, bool, error) {
	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are a careful study assistant. Return only valid JSON that matches the provided response schema exactly.",
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"response_format": map[string]any{
			"type":        "json_schema",
			"json_schema": schema,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", false, fmt.Errorf("openai: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", false, fmt.Errorf("openai: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", isRetryableTransportError(err), fmt.Errorf("openai: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, fmt.Errorf("openai: read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", isRetryableStatus(resp.StatusCode), fmt.Errorf("openai: status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
				Refusal string `json:"refusal"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		if os.Getenv("DEBUG_OPENAI_RAW") != "" {
			return "", false, fmt.Errorf("openai: decode response: %w: %s", err, string(respBody))
		}
		return "", false, fmt.Errorf("openai: decode response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", false, fmt.Errorf("openai: empty response")
	}
	if strings.TrimSpace(result.Choices[0].Message.Refusal) != "" {
		return "", false, fmt.Errorf("openai: refusal: %s", strings.TrimSpace(result.Choices[0].Message.Refusal))
	}
	if strings.TrimSpace(result.Choices[0].Message.Content) == "" {
		return "", false, fmt.Errorf("openai: empty response content")
	}

	return result.Choices[0].Message.Content, false, nil
}

func generatedQuestionsSchema() map[string]any {
	return map[string]any{
		"name":   "generated_questions_response",
		"strict": true,
		"schema": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"questions"},
			"properties": map[string]any{
				"questions": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type":                 "object",
						"additionalProperties": false,
						"required":             []string{"content", "options", "correct_answer", "explanation"},
						"properties": map[string]any{
							"content": map[string]any{
								"type": "string",
							},
							"options": map[string]any{
								"type":  "array",
								"items": map[string]any{"type": "string"},
							},
							"correct_answer": map[string]any{
								"type": "string",
							},
							"explanation": map[string]any{
								"type": "string",
							},
						},
					},
				},
			},
		},
	}
}

func parseJSON(raw string, v any) error {
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

func readEnvString(key string, fallback string) string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	return raw
}

func isRetryableTransportError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func isRetryableStatus(statusCode int) bool {
	return statusCode == http.StatusRequestTimeout ||
		statusCode == http.StatusConflict ||
		statusCode == http.StatusTooManyRequests ||
		statusCode >= http.StatusInternalServerError
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
