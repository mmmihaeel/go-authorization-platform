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
	StoreName            string
	AuthorizationModelID string
	APIToken             string
	Timeout              time.Duration
}

type openFGAStore struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type openFGAStoresResponse struct {
	Stores []openFGAStore `json:"stores"`
}

type openFGAModel struct {
	ID string `json:"id"`
}

type openFGAModelsResponse struct {
	AuthorizationModels []openFGAModel `json:"authorization_models"`
}

// DiscoverOpenFGAConfig resolves the store and latest immutable model created by
// the local bootstrap job. Explicit IDs still take precedence for deployments.
func DiscoverOpenFGAConfig(ctx context.Context, config OpenFGAConfig) (OpenFGAConfig, error) {
	if config.StoreID != "" && config.AuthorizationModelID != "" {
		return config, nil
	}
	baseURL, err := parseHTTPBaseURL(config.BaseURL, "OpenFGA")
	if err != nil {
		return OpenFGAConfig{}, err
	}
	if config.Timeout <= 0 {
		config.Timeout = 2 * time.Second
	}
	client := &http.Client{Timeout: config.Timeout}
	if config.StoreID == "" {
		if config.StoreName == "" {
			return OpenFGAConfig{}, fmt.Errorf("OpenFGA store ID or store name is required")
		}
		var stores openFGAStoresResponse
		if err := getOpenFGAJSON(ctx, client, baseURL+"/stores?page_size=100", config.APIToken, &stores); err != nil {
			return OpenFGAConfig{}, fmt.Errorf("list OpenFGA stores: %w", err)
		}
		for _, store := range stores.Stores {
			if store.Name == config.StoreName && (config.StoreID == "" || store.CreatedAt.After(findStoreCreatedAt(stores.Stores, config.StoreID))) {
				config.StoreID = store.ID
			}
		}
		if config.StoreID == "" {
			return OpenFGAConfig{}, fmt.Errorf("OpenFGA store %q was not found", config.StoreName)
		}
	}
	if config.AuthorizationModelID == "" {
		var models openFGAModelsResponse
		endpoint := baseURL + "/stores/" + url.PathEscape(config.StoreID) + "/authorization-models?page_size=1"
		if err := getOpenFGAJSON(ctx, client, endpoint, config.APIToken, &models); err != nil {
			return OpenFGAConfig{}, fmt.Errorf("list OpenFGA models: %w", err)
		}
		if len(models.AuthorizationModels) == 0 || models.AuthorizationModels[0].ID == "" {
			return OpenFGAConfig{}, fmt.Errorf("OpenFGA store %s has no authorization model", config.StoreID)
		}
		config.AuthorizationModelID = models.AuthorizationModels[0].ID
	}
	return config, nil
}

func findStoreCreatedAt(stores []openFGAStore, id string) time.Time {
	for _, store := range stores {
		if store.ID == id {
			return store.CreatedAt
		}
	}
	return time.Time{}
}

func getOpenFGAJSON(ctx context.Context, client *http.Client, endpoint, token string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		limited, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("returned %s: %s", response.Status, strings.TrimSpace(string(limited)))
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func parseHTTPBaseURL(raw, component string) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(raw, "/"))
	if err != nil {
		return "", fmt.Errorf("parse %s base URL: %w", component, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("%s base URL must use http or https", component)
	}
	if parsed.Host == "" || parsed.User != nil {
		return "", fmt.Errorf("%s base URL must contain a host and no credentials", component)
	}
	return parsed.String(), nil
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
	baseURL, err := parseHTTPBaseURL(config.BaseURL, "OpenFGA")
	if err != nil {
		return nil, err
	}
	parsed, _ := url.Parse(baseURL)
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
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result); err != nil {
		return false, fmt.Errorf("decode OpenFGA response: %w", err)
	}
	return result.Allowed, nil
}
