package authn

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
)

type OIDCVerifier struct {
	verifier *oidc.IDTokenVerifier
}

func NewOIDCVerifier(ctx context.Context, issuerURL, audience, jwksURL string) (*OIDCVerifier, error) {
	if issuerURL == "" || audience == "" {
		return nil, fmt.Errorf("OIDC issuer and audience are required")
	}
	if err := validateHTTPURL(issuerURL, "issuer"); err != nil {
		return nil, err
	}
	if jwksURL != "" {
		if err := validateHTTPURL(jwksURL, "JWKS"); err != nil {
			return nil, err
		}
		keys := oidc.NewRemoteKeySet(ctx, jwksURL)
		return &OIDCVerifier{verifier: oidc.NewVerifier(issuerURL, keys, &oidc.Config{ClientID: audience})}, nil
	}
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("discover OIDC provider: %w", err)
	}
	return &OIDCVerifier{verifier: provider.Verifier(&oidc.Config{ClientID: audience})}, nil
}

func validateHTTPURL(raw, name string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("OIDC %s URL must be an absolute HTTP(S) URL", name)
	}
	return nil
}

func (v *OIDCVerifier) Verify(ctx context.Context, rawToken string) (Claims, error) {
	token, err := v.verifier.Verify(ctx, rawToken)
	if err != nil {
		return Claims{}, err
	}
	var raw struct {
		Subject       string         `json:"sub"`
		AuthzSubject  string         `json:"authz_subject"`
		PrincipalType string         `json:"principal_type"`
		TenantID      string         `json:"tenant_id"`
		Azp           string         `json:"azp"`
		Act           map[string]any `json:"act"`
		AMR           []string       `json:"amr"`
		RealmAccess   struct {
			Roles []string `json:"roles"`
		} `json:"realm_access"`
	}
	if err := token.Claims(&raw); err != nil {
		return Claims{}, fmt.Errorf("decode OIDC claims: %w", err)
	}
	if raw.AuthzSubject == "" {
		return Claims{}, fmt.Errorf("token is missing authz_subject")
	}
	principalType := raw.PrincipalType
	if principalType == "" {
		principalType = "user"
	}
	return Claims{
		TokenSubject: raw.Subject, Subject: raw.AuthzSubject, SubjectType: principalType,
		TenantID: raw.TenantID, Roles: raw.RealmAccess.Roles, AuthorizedBy: raw.Azp,
		Actor: raw.Act, MFA: contains(raw.AMR, "mfa") || contains(raw.AMR, "otp"),
	}, nil
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
