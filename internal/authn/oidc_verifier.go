//go:build oidc

package authn

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
)

type OIDCVerifier struct {
	verifier *oidc.IDTokenVerifier
}

func NewOIDCVerifier(ctx context.Context, issuerURL, audience string) (*OIDCVerifier, error) {
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("discover OIDC provider: %w", err)
	}
	return &OIDCVerifier{verifier: provider.Verifier(&oidc.Config{ClientID: audience})}, nil
}

func (v *OIDCVerifier) Verify(ctx context.Context, rawToken string) (Claims, error) {
	token, err := v.verifier.Verify(ctx, rawToken)
	if err != nil {
		return Claims{}, err
	}
	var raw struct {
		Subject     string         `json:"sub"`
		TenantID    string         `json:"tenant_id"`
		Azp         string         `json:"azp"`
		Act         map[string]any `json:"act"`
		RealmAccess struct {
			Roles []string `json:"roles"`
		} `json:"realm_access"`
	}
	if err := token.Claims(&raw); err != nil {
		return Claims{}, fmt.Errorf("decode OIDC claims: %w", err)
	}
	return Claims{Subject: raw.Subject, TenantID: raw.TenantID, Roles: raw.RealmAccess.Roles, AuthorizedBy: raw.Azp, Actor: raw.Act}, nil
}
