package authorization

import (
	"context"
	"testing"
)

func BenchmarkDecisionService(b *testing.B) {
	service, _ := NewService(
		checkerFunc(func(context.Context, RelationshipQuery) (bool, error) { return true, nil }),
		evaluatorFunc(func(context.Context, PolicyInput) (PolicyResult, error) { return PolicyResult{Allow: true}, nil }),
		nil,
		ServiceConfig{},
	)
	request := validRequest()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := service.Check(context.Background(), request); err != nil {
			b.Fatal(err)
		}
	}
}
