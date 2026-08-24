// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"strings"
	"testing"
)

func TestForProject(t *testing.T) {
	t.Parallel()

	base := &DatoCMSClient{
		APIToken: "default-token",
		APITokens: map[string]string{
			"store-one": "token-one",
			"store-two": "token-two",
		},
		BaseURL: defaultBaseURL,
	}

	t.Run("empty project resolves the default token", func(t *testing.T) {
		t.Parallel()
		client, err := base.forProject("")
		if err != nil {
			t.Fatalf("forProject(%q) = %v, want nil", "", err)
		}
		if client.APIToken != "default-token" {
			t.Fatalf("resolved token = %q, want %q", client.APIToken, "default-token")
		}
	})

	t.Run("known project resolves its token", func(t *testing.T) {
		t.Parallel()
		client, err := base.forProject("store-two")
		if err != nil {
			t.Fatalf("forProject(%q) = %v, want nil", "store-two", err)
		}
		if client.APIToken != "token-two" {
			t.Fatalf("resolved token = %q, want %q", client.APIToken, "token-two")
		}
		if client.BaseURL != base.BaseURL {
			t.Fatalf("resolved base URL = %q, want %q", client.BaseURL, base.BaseURL)
		}
		if base.APIToken != "default-token" {
			t.Fatalf("receiver token mutated to %q", base.APIToken)
		}
	})

	t.Run("unknown project errors with the project and available keys", func(t *testing.T) {
		t.Parallel()
		_, err := base.forProject("store-three")
		if err == nil {
			t.Fatalf("forProject(%q) = nil, want error", "store-three")
		}
		msg := err.Error()
		for _, want := range []string{`"store-three"`, "store-one", "store-two"} {
			if !strings.Contains(msg, want) {
				t.Fatalf("error %q does not mention %q", msg, want)
			}
		}
		for _, leaked := range []string{"default-token", "token-one", "token-two"} {
			if strings.Contains(msg, leaked) {
				t.Fatalf("error %q leaks token %q", msg, leaked)
			}
		}
	})

	t.Run("unknown project with empty api_tokens errors", func(t *testing.T) {
		t.Parallel()
		noMap := &DatoCMSClient{APIToken: "default-token"}
		_, err := noMap.forProject("store-one")
		if err == nil {
			t.Fatalf("forProject(%q) = nil, want error", "store-one")
		}
		if !strings.Contains(err.Error(), `"store-one"`) {
			t.Fatalf("error %q does not mention the project", err.Error())
		}
	})

	t.Run("empty project without a default token errors", func(t *testing.T) {
		t.Parallel()
		noDefault := &DatoCMSClient{APITokens: map[string]string{"store-one": "token-one"}}
		_, err := noDefault.forProject("")
		if err == nil {
			t.Fatalf("forProject(%q) = nil, want error", "")
		}
		if !strings.Contains(err.Error(), "no default API token") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestParseImportID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		importID    string
		wantProject string
		wantID      string
	}{
		{name: "plain ID", importID: "334477", wantProject: "", wantID: "334477"},
		{name: "compound ID", importID: "store-one/334477", wantProject: "store-one", wantID: "334477"},
		{name: "ID with extra slash", importID: "store-one/a/b", wantProject: "store-one", wantID: "a/b"},
		{name: "missing ID", importID: "store-one/", wantProject: "store-one", wantID: ""},
		{name: "empty", importID: "", wantProject: "", wantID: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			project, id := parseImportID(tt.importID)
			if project != tt.wantProject || id != tt.wantID {
				t.Fatalf("parseImportID(%q) = (%q, %q), want (%q, %q)", tt.importID, project, id, tt.wantProject, tt.wantID)
			}
		})
	}
}
