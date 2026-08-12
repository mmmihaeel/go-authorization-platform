package authn

import "testing"

func TestValidateHTTPURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "https", value: "https://identity.example/realms/authz"},
		{name: "local http", value: "http://keycloak:8080/realms/authz"},
		{name: "relative", value: "/realms/authz", wantErr: true},
		{name: "file", value: "file:///tmp/keys.json", wantErr: true},
		{name: "missing host", value: "https:///realms/authz", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := validateHTTPURL(test.value, "test")
			if (err != nil) != test.wantErr {
				t.Fatalf("validateHTTPURL(%q) error=%v, wantErr=%v", test.value, err, test.wantErr)
			}
		})
	}
}
