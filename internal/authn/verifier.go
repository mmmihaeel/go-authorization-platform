package authn

import (
	"context"
	"net/http"
	"strings"
)

type Claims struct {
	TokenSubject string         `json:"sub"`
	Subject      string         `json:"authz_subject"`
	SubjectType  string         `json:"principal_type"`
	TenantID     string         `json:"tenant_id"`
	Roles        []string       `json:"roles,omitempty"`
	AuthorizedBy string         `json:"azp,omitempty"`
	Actor        map[string]any `json:"act,omitempty"`
	MFA          bool           `json:"-"`
}

type Verifier interface {
	Verify(context.Context, string) (Claims, error)
}

type contextKey struct{}

func FromContext(ctx context.Context) (Claims, bool) {
	claims, ok := ctx.Value(contextKey{}).(Claims)
	return claims, ok
}

func RequireBearer(verifier Verifier, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if verifier == nil {
			http.Error(w, "authentication verifier is not configured", http.StatusServiceUnavailable)
			return
		}
		header := r.Header.Get("Authorization")
		parts := strings.Fields(header)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			http.Error(w, "bearer token is required", http.StatusUnauthorized)
			return
		}
		token := parts[1]
		if token == "" {
			http.Error(w, "bearer token is required", http.StatusUnauthorized)
			return
		}
		claims, err := verifier.Verify(r.Context(), token)
		if err != nil {
			http.Error(w, "invalid bearer token", http.StatusUnauthorized)
			return
		}
		if claims.Subject == "" || claims.TenantID == "" {
			http.Error(w, "token is missing required authorization claims", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, claims)))
	})
}
