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

type OpenFGAConfig struct {
	BaseURL              string
	StoreID              string
	AuthorizationModelID string
	APIToken             string
	Timeout              time.Duration
}

type OpenFGAClient struct {
	endpoint             string
	authorizationModelID string
	apiToken             string
	client               *http.Client
}

func NewOpenFGAClient(config OpenFGAConfig) (*OpenFGAClient, error) {
	if strings.TrimSpace(config.BaseURL) == "" {
		return nil, fmt.Errorf("OpenFGA base URL is required")
	}
	if strings.TrimSpace(config.StoreID) == "" {
		return nil, fmt.Errorf("OpenFGA store ID is required")
	}
	parsed, err := url.Parse(strings.TrimRight(config.BaseURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("parse OpenFGA base URL: %w", err)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/stores/" + url.PathEscape(config.StoreID) + "/check"
	if config.Timeout <= 0 {
		config.Timeout = 2 * time.Second
	}
	return &OpenFGAClient{
		endpoint:             parsed.String(),
		authorizationModelID: config.AuthorizationModelID,
		apiToken:             config.APIToken,
		client:               &http.Client{Timeout: config.Timeout},
	}, nil
}

type openFGATupleKey struct {
	User     string `json:"user"`
	Relation string `json:"relation"`
	Object   string `json:"object"`
}

type openFGACheckRequest struct {
	AuthorizationModelID string          `json:"authorization_model_id,omitempty"`
	TupleKey             openFGATupleKey `json:"tuple_key"`
	Context              map[string]any  `json:"context,omitempty"`
}

type openFGACheckResponse struct {
	Allowed bool `json:"allowed"`
}

func (c *OpenFGAClient) Check(ctx context.Context, query RelationshipQuery) (bool, error) {
	body := openFGACheckRequest{
		AuthorizationModelID: c.authorizationModelID,
		TupleKey: openFGATupleKey{
			User:     query.Subject.Type + ":" + query.Subject.ID,
			Relation: query.Relation,
			Object:   query.Resource.Type + ":" + query.Resource.TenantID + "/" + query.Resource.ID,
		},
		Context: map[string]any{
			"tenant_id":      query.Subject.TenantID,
			"device_trusted": query.Context.DeviceTrusted,
			"mfa":            query.Context.MFA,
			"risk_score":     query.Context.RiskScore,
		},
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return false, fmt.Errorf("encode OpenFGA check: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(encoded))
	if err != nil {
		return false, fmt.Errorf("create OpenFGA check request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if c.apiToken != "" {
		request.Header.Set("Authorization", "Bearer "+c.apiToken)
	}

	response, err := c.client.Do(request)
	if err != nil {
		return false, fmt.Errorf("call OpenFGA: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		limited, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return false, fmt.Errorf("OpenFGA returned %s: %s", response.Status, strings.TrimSpace(string(limited)))
	}

	var result openFGACheckResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("decode OpenFGA response: %w", err)
	}
	return result.Allowed, nil
}
