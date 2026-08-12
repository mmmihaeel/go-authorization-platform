//go:build !otel

package observability

import (
	"context"
	"net/http"
)

func SetupTracing(context.Context, string, string) (func(context.Context) error, error) {
	return func(context.Context) error { return nil }, nil
}

func HTTPMiddleware(_ string, next http.Handler) http.Handler { return next }
