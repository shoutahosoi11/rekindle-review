package cloudrun

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	runv2 "google.golang.org/api/run/v2"
)

// HighlightImportJobTrigger は Cloud Run Jobs API 経由でジョブを起動する。
// 環境変数 HIGHLIGHT_IMPORT_JOB_NAME が未設定の場合は no-op として動作する。
type HighlightImportJobTrigger struct {
	jobName string
}

func NewHighlightImportJobTrigger() *HighlightImportJobTrigger {
	return &HighlightImportJobTrigger{
		jobName: os.Getenv("HIGHLIGHT_IMPORT_JOB_NAME"),
	}
}

// TriggerHighlightImportJob は Cloud Run Job の実行をキックする。
// jobName 未設定（ローカル開発）の場合はスキップして nil を返す。
func (t *HighlightImportJobTrigger) TriggerHighlightImportJob(ctx context.Context) error {
	if t.jobName == "" {
		log.Println("highlight import job trigger: HIGHLIGHT_IMPORT_JOB_NAME not set, skipping")
		return nil
	}

	client, err := newRunClient(ctx)
	if err != nil {
		return fmt.Errorf("highlight import job trigger: create run client: %w", err)
	}

	_, err = client.Projects.Locations.Jobs.Run(
		t.jobName,
		&runv2.GoogleCloudRunV2RunJobRequest{},
	).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("highlight import job trigger: run job %s: %w", t.jobName, err)
	}

	log.Printf("highlight import job trigger: triggered %s", t.jobName)
	return nil
}

func newRunClient(ctx context.Context) (*runv2.Service, error) {
	creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return nil, fmt.Errorf("find default credentials: %w", err)
	}

	httpClient := oauth2Transport(creds)
	return runv2.NewService(ctx, option.WithHTTPClient(httpClient))
}

func oauth2Transport(creds *google.Credentials) *http.Client {
	return &http.Client{
		Transport: &oauth2RoundTripper{creds: creds},
	}
}

type oauth2RoundTripper struct {
	creds *google.Credentials
}

func (t *oauth2RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	token, err := t.creds.TokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	return http.DefaultTransport.RoundTrip(req)
}
