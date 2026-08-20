// SPDX-License-Identifier: MPL-2.0

package provider

import "testing"

func TestValidateBaseURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		baseURL string
		wantErr bool
	}{
		{name: "https default", baseURL: defaultBaseURL, wantErr: false},
		{name: "https custom host", baseURL: "https://example.com", wantErr: false},
		{name: "http localhost", baseURL: "http://localhost:8080", wantErr: false},
		{name: "http loopback", baseURL: "http://127.0.0.1:8080", wantErr: false},
		{name: "http remote host", baseURL: "http://example.com", wantErr: true},
		{name: "no scheme", baseURL: "example.com", wantErr: true},
		{name: "unsupported scheme", baseURL: "ftp://example.com", wantErr: true},
		{name: "unparsable", baseURL: "https://exa mple.com/%zz", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateBaseURL(tt.baseURL)
			if tt.wantErr && err == nil {
				t.Fatalf("validateBaseURL(%q) = nil, want error", tt.baseURL)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("validateBaseURL(%q) = %v, want nil", tt.baseURL, err)
			}
		})
	}
}
