package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type customUpdateDispatcher interface {
	Enabled() bool
	Dispatch(context.Context) error
}

type githubWorkflowDispatcher struct {
	token      string
	repository string
	workflow   string
	ref        string
	client     *http.Client
}

func newGitHubWorkflowDispatcherFromEnv() customUpdateDispatcher {
	return &githubWorkflowDispatcher{
		token:      strings.TrimSpace(os.Getenv("CUSTOM_UPDATE_GITHUB_TOKEN")),
		repository: customUpdateEnv("CUSTOM_UPDATE_GITHUB_REPOSITORY", "ywq19950815/sub2api"),
		workflow:   customUpdateEnv("CUSTOM_UPDATE_GITHUB_WORKFLOW", "custom-sync-upstream.yml"),
		ref:        customUpdateEnv("CUSTOM_UPDATE_GITHUB_REF", "custom/batch-liveness"),
		client:     &http.Client{Timeout: 30 * time.Second},
	}
}

func customUpdateEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func (d *githubWorkflowDispatcher) Enabled() bool {
	return d != nil && d.token != ""
}

func (d *githubWorkflowDispatcher) Dispatch(ctx context.Context) error {
	if !d.Enabled() {
		return fmt.Errorf("custom update workflow is not configured")
	}

	payload, err := json.Marshal(map[string]string{"ref": d.ref})
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf(
		"https://api.github.com/repos/%s/actions/workflows/%s/dispatches",
		d.repository,
		url.PathEscape(d.workflow),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+d.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("dispatch custom update workflow: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("dispatch custom update workflow: GitHub returned %s", resp.Status)
	}
	return nil
}
