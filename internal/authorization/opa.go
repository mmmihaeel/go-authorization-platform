package authorization

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type OPAConfig struct {
	BaseURL      string
	DecisionPath string
	Timeout      time.Duration
}

type OPAClient struct {
	endpoint string
	client   *http.Client
}

func NewOPAClient(config OPAConfig) (*OPAClient, error) {
	if strings.TrimSpace(config.BaseURL) == "" {
		return nil, fmt.Errorf("OPA base URL is required")
	}
	decisionPath := strings.Trim(config.DecisionPath, "/")
	if decisionPath == "" {
		decisionPath = "authz/decision"
	}
	parsed, err := url.Parse(strings.TrimRight(config.BaseURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("parse OPA base URL: %w", err)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/v1/data/" + decisionPath
	if config.Timeout <= 0 {
		config.Timeout = 500 * time.Millisecond
	}
	return &OPAClient{endpoint: parsed.String(), client: &http.Client{Timeout: config.Timeout}}, nil
}

type opaRequest struct {
	Input PolicyInput `json:"input"`
}

type opaResponse struct {
	Result *PolicyResult `json:"result"`
}

func (c *OPAClient) Evaluate(ctx context.Context, input PolicyInput) (PolicyResult, error) {
	encoded, err := json.Marshal(opaRequest{Input: input})
	if err != nil {
		return PolicyResult{}, fmt.Errorf("encode OPA input: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(encoded))
	if err != nil {
		return PolicyResult{}, fmt.Errorf("create OPA request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.client.Do(request)
	if err != nil {
		return PolicyResult{}, fmt.Errorf("call OPA: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		limited, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return PolicyResult{}, fmt.Errorf("OPA returned %s: %s", response.Status, strings.TrimSpace(string(limited)))
	}

	var result opaResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return PolicyResult{}, fmt.Errorf("decode OPA response: %w", err)
	}
	if result.Result == nil {
		return PolicyResult{}, fmt.Errorf("OPA decision was undefined")
	}
	return *result.Result, nil
}
