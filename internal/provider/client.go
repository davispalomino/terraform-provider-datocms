// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// errNotFound is returned by Get* methods when the API responds with 404.
var errNotFound = errors.New("resource not found")

const maxRequestAttempts = 3

// maxRateLimitWait caps the wait derived from the x-ratelimit-reset header so
// a bogus or huge value cannot stall the provider.
const maxRateLimitWait = 60 * time.Second

// forProject resolves the client to use for the given project key. An empty
// project selects the default token (api_token attribute or DATOCMS_API_TOKEN
// environment variable); a non-empty project selects the matching entry of the
// api_tokens provider attribute. The returned client shares the HTTP client
// and base URL of the receiver. Error messages never include token values.
func (c *DatoCMSClient) forProject(project string) (*DatoCMSClient, error) {
	if project == "" {
		if c.APIToken == "" {
			return nil, errors.New("no default API token is configured: set the api_token provider attribute or the DATOCMS_API_TOKEN environment variable, or set the resource's project attribute to one of the keys of api_tokens")
		}
		return c, nil
	}

	token, ok := c.APITokens[project]
	if !ok {
		keys := make([]string, 0, len(c.APITokens))
		for key := range c.APITokens {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if len(keys) == 0 {
			return nil, fmt.Errorf("project %q is not defined in the provider's api_tokens attribute, which is empty: add an %q entry to api_tokens", project, project)
		}
		return nil, fmt.Errorf("project %q is not defined in the provider's api_tokens attribute: available keys are %s", project, strings.Join(keys, ", "))
	}

	scoped := *c
	scoped.APIToken = token
	return &scoped, nil
}

// parseImportID splits a resource import ID into its optional project key and
// the resource ID. The compound form is "project/id" (for example
// "store-one/334477"); an ID without "/" selects the default token (empty
// project).
func parseImportID(importID string) (project, id string) {
	if before, after, found := strings.Cut(importID, "/"); found {
		return before, after
	}
	return "", importID
}

// doRequest performs an authenticated request against the DatoCMS CMA,
// handling the required headers, JSON:API error bodies and 429 rate-limit
// retries (max 3 attempts, honoring the x-ratelimit-reset header when
// present).
func (c *DatoCMSClient) doRequest(ctx context.Context, method, path string, body any, out any) error {
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
	}

	var lastErr error
	for attempt := 1; attempt <= maxRequestAttempts; attempt++ {
		var reader io.Reader
		if payload != nil {
			reader = bytes.NewReader(payload)
		}

		req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reader)
		if err != nil {
			return fmt.Errorf("building request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+c.APIToken)
		req.Header.Set("X-Api-Version", "3")
		req.Header.Set("Accept", "application/json")
		if payload != nil {
			req.Header.Set("Content-Type", "application/vnd.api+json")
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return fmt.Errorf("%s %s: %w", method, path, err)
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("%s %s: reading response body: %w", method, path, err)
		}

		switch {
		case resp.StatusCode == http.StatusTooManyRequests:
			lastErr = fmt.Errorf("%s %s: HTTP 429 rate limited: %s", method, path, string(respBody))
			if attempt == maxRequestAttempts {
				return lastErr
			}
			wait := time.Duration(attempt) * time.Second
			if reset := resp.Header.Get("x-ratelimit-reset"); reset != "" {
				if seconds, err := strconv.ParseFloat(reset, 64); err == nil && seconds > 0 {
					wait = time.Duration(seconds * float64(time.Second))
					if wait > maxRateLimitWait {
						wait = maxRateLimitWait
					}
				}
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
			continue

		case resp.StatusCode == http.StatusNotFound:
			return fmt.Errorf("%s %s: %w: %s", method, path, errNotFound, string(respBody))

		case resp.StatusCode < 200 || resp.StatusCode >= 300:
			return fmt.Errorf("%s %s: unexpected status %d: %s", method, path, resp.StatusCode, string(respBody))
		}

		if out != nil && len(respBody) > 0 {
			if err := json.Unmarshal(respBody, out); err != nil {
				return fmt.Errorf("%s %s: decoding response body: %w", method, path, err)
			}
		}
		return nil
	}

	return lastErr
}

// --- Role API types (JSON:API) ---

// roleItemTypePermission mirrors one entry of
// positive/negative_item_type_permissions.
type roleItemTypePermission struct {
	Action            string  `json:"action"`
	Environment       *string `json:"environment,omitempty"`
	ItemType          *string `json:"item_type,omitempty"`
	Workflow          *string `json:"workflow,omitempty"`
	OnStage           *string `json:"on_stage,omitempty"`
	ToStage           *string `json:"to_stage,omitempty"`
	OnCreator         *string `json:"on_creator,omitempty"`
	LocalizationScope *string `json:"localization_scope,omitempty"`
	Locale            *string `json:"locale,omitempty"`
}

// roleUploadPermission mirrors one entry of
// positive/negative_upload_permissions.
type roleUploadPermission struct {
	Action                 string  `json:"action"`
	Environment            *string `json:"environment,omitempty"`
	UploadCollection       *string `json:"upload_collection,omitempty"`
	MoveToUploadCollection *string `json:"move_to_upload_collection,omitempty"`
	OnCreator              *string `json:"on_creator,omitempty"`
	LocalizationScope      *string `json:"localization_scope,omitempty"`
	Locale                 *string `json:"locale,omitempty"`
}

// roleBuildTriggerPermission mirrors one entry of
// positive/negative_build_trigger_permissions. A null build_trigger covers
// every build trigger, so the field must serialize explicitly (no omitempty).
type roleBuildTriggerPermission struct {
	BuildTrigger *string `json:"build_trigger"`
}

// roleSearchIndexPermission mirrors one entry of
// positive/negative_search_index_permissions.
type roleSearchIndexPermission struct {
	SearchIndex *string `json:"search_index"`
}

// roleAttributes mirrors the role attributes object of the CMA.
type roleAttributes struct {
	Name                            string                       `json:"name"`
	CanEditSite                     bool                         `json:"can_edit_site"`
	CanEditFavicon                  bool                         `json:"can_edit_favicon"`
	CanEditSchema                   bool                         `json:"can_edit_schema"`
	CanManageMenu                   bool                         `json:"can_manage_menu"`
	CanEditEnvironment              bool                         `json:"can_edit_environment"`
	CanPromoteEnvironments          bool                         `json:"can_promote_environments"`
	CanManageEnvironments           bool                         `json:"can_manage_environments"`
	CanManageUsers                  bool                         `json:"can_manage_users"`
	CanManageSharedFilters          bool                         `json:"can_manage_shared_filters"`
	CanManageSearchIndexes          bool                         `json:"can_manage_search_indexes"`
	CanManageUploadCollections      bool                         `json:"can_manage_upload_collections"`
	CanManageBuildTriggers          bool                         `json:"can_manage_build_triggers"`
	CanManageWebhooks               bool                         `json:"can_manage_webhooks"`
	CanManageSSO                    bool                         `json:"can_manage_sso"`
	CanManageWorkflows              bool                         `json:"can_manage_workflows"`
	CanManageAccessTokens           bool                         `json:"can_manage_access_tokens"`
	CanAccessAuditLog               bool                         `json:"can_access_audit_log"`
	CanPerformSiteSearch            bool                         `json:"can_perform_site_search"`
	CanAccessBuildEventsLog         bool                         `json:"can_access_build_events_log"`
	CanAccessSearchIndexEventsLog   bool                         `json:"can_access_search_index_events_log"`
	EnvironmentsAccess              *string                      `json:"environments_access,omitempty"`
	PositiveItemTypePermissions     []roleItemTypePermission     `json:"positive_item_type_permissions"`
	NegativeItemTypePermissions     []roleItemTypePermission     `json:"negative_item_type_permissions"`
	PositiveUploadPermissions       []roleUploadPermission       `json:"positive_upload_permissions"`
	NegativeUploadPermissions       []roleUploadPermission       `json:"negative_upload_permissions"`
	PositiveBuildTriggerPermissions []roleBuildTriggerPermission `json:"positive_build_trigger_permissions"`
	NegativeBuildTriggerPermissions []roleBuildTriggerPermission `json:"negative_build_trigger_permissions"`
	PositiveSearchIndexPermissions  []roleSearchIndexPermission  `json:"positive_search_index_permissions"`
	NegativeSearchIndexPermissions  []roleSearchIndexPermission  `json:"negative_search_index_permissions"`
}

// roleLinkage is a JSON:API resource linkage object.
type roleLinkage struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// roleRelationships mirrors the role relationships object.
type roleRelationships struct {
	InheritsPermissionsFrom struct {
		Data []roleLinkage `json:"data"`
	} `json:"inherits_permissions_from"`
}

// roleData is the JSON:API resource object for a role.
type roleData struct {
	ID            string             `json:"id,omitempty"`
	Type          string             `json:"type"`
	Attributes    roleAttributes     `json:"attributes"`
	Relationships *roleRelationships `json:"relationships,omitempty"`
	// NOTE: responses also carry meta.final_permissions (the effective
	// permission set after resolving inherited roles); intentionally not
	// decoded/exposed for now.
}

// rolePayload wraps a single role document.
type rolePayload struct {
	Data roleData `json:"data"`
}

func newRoleRelationships(inheritsFrom []string) *roleRelationships {
	rels := &roleRelationships{}
	rels.InheritsPermissionsFrom.Data = make([]roleLinkage, 0, len(inheritsFrom))
	for _, id := range inheritsFrom {
		rels.InheritsPermissionsFrom.Data = append(rels.InheritsPermissionsFrom.Data, roleLinkage{Type: "role", ID: id})
	}
	return rels
}

// CreateRole creates a role via POST /roles.
func (c *DatoCMSClient) CreateRole(ctx context.Context, attrs roleAttributes, inheritsFrom []string) (*roleData, error) {
	body := rolePayload{Data: roleData{
		Type:          "role",
		Attributes:    attrs,
		Relationships: newRoleRelationships(inheritsFrom),
	}}
	var out rolePayload
	if err := c.doRequest(ctx, http.MethodPost, "/roles", body, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// GetRole retrieves a role via GET /roles/{id}. Returns an error wrapping
// errNotFound on 404.
func (c *DatoCMSClient) GetRole(ctx context.Context, id string) (*roleData, error) {
	var out rolePayload
	if err := c.doRequest(ctx, http.MethodGet, "/roles/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// UpdateRole updates a role via PUT /roles/{id}. The full attribute set is
// sent: the API replaces permission arrays wholesale.
func (c *DatoCMSClient) UpdateRole(ctx context.Context, id string, attrs roleAttributes, inheritsFrom []string) (*roleData, error) {
	body := rolePayload{Data: roleData{
		ID:            id,
		Type:          "role",
		Attributes:    attrs,
		Relationships: newRoleRelationships(inheritsFrom),
	}}
	var out rolePayload
	if err := c.doRequest(ctx, http.MethodPut, "/roles/"+url.PathEscape(id), body, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// DeleteRole deletes a role via DELETE /roles/{id}.
func (c *DatoCMSClient) DeleteRole(ctx context.Context, id string) error {
	return c.doRequest(ctx, http.MethodDelete, "/roles/"+url.PathEscape(id), nil, nil)
}

// --- Webhook API types (JSON:API) ---

// webhookEventFilter mirrors one entry of events[].filters (conditional
// triggering). Both fields are required by the API.
type webhookEventFilter struct {
	EntityType string   `json:"entity_type"`
	EntityIDs  []string `json:"entity_ids"`
}

// webhookEvent mirrors one entry of the webhook events array.
type webhookEvent struct {
	EntityType string               `json:"entity_type"`
	EventTypes []string             `json:"event_types"`
	Filters    []webhookEventFilter `json:"filters,omitempty"`
}

// webhookAttributes mirrors the webhook attributes object of the CMA. The
// create endpoint requires headers, events, custom_payload, http_basic_user
// and http_basic_password even when null, so the nullable fields serialize
// explicitly (no omitempty).
type webhookAttributes struct {
	Name                 string            `json:"name"`
	URL                  string            `json:"url"`
	Headers              map[string]string `json:"headers"`
	Events               []webhookEvent    `json:"events"`
	CustomPayload        *string           `json:"custom_payload"`
	HTTPBasicUser        *string           `json:"http_basic_user"`
	HTTPBasicPassword    *string           `json:"http_basic_password"`
	Enabled              bool              `json:"enabled"`
	PayloadAPIVersion    string            `json:"payload_api_version"`
	NestedItemsInPayload bool              `json:"nested_items_in_payload"`
	AutoRetry            bool              `json:"auto_retry"`
}

// webhookData is the JSON:API resource object for a webhook.
type webhookData struct {
	ID         string            `json:"id,omitempty"`
	Type       string            `json:"type"`
	Attributes webhookAttributes `json:"attributes"`
}

// webhookPayload wraps a single webhook document.
type webhookPayload struct {
	Data webhookData `json:"data"`
}

// CreateWebhook creates a webhook via POST /webhooks.
func (c *DatoCMSClient) CreateWebhook(ctx context.Context, attrs webhookAttributes) (*webhookData, error) {
	body := webhookPayload{Data: webhookData{
		Type:       "webhook",
		Attributes: attrs,
	}}
	var out webhookPayload
	if err := c.doRequest(ctx, http.MethodPost, "/webhooks", body, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// GetWebhook retrieves a webhook via GET /webhooks/{id}. Returns an error
// wrapping errNotFound on 404.
func (c *DatoCMSClient) GetWebhook(ctx context.Context, id string) (*webhookData, error) {
	var out webhookPayload
	if err := c.doRequest(ctx, http.MethodGet, "/webhooks/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// UpdateWebhook updates a webhook via PUT /webhooks/{id}. The full attribute
// set is always sent.
func (c *DatoCMSClient) UpdateWebhook(ctx context.Context, id string, attrs webhookAttributes) (*webhookData, error) {
	body := webhookPayload{Data: webhookData{
		ID:         id,
		Type:       "webhook",
		Attributes: attrs,
	}}
	var out webhookPayload
	if err := c.doRequest(ctx, http.MethodPut, "/webhooks/"+url.PathEscape(id), body, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// DeleteWebhook deletes a webhook via DELETE /webhooks/{id}.
func (c *DatoCMSClient) DeleteWebhook(ctx context.Context, id string) error {
	return c.doRequest(ctx, http.MethodDelete, "/webhooks/"+url.PathEscape(id), nil, nil)
}

// --- Access token API types (JSON:API) ---

// accessTokenAttributes mirrors the access_token attributes object of the CMA.
// Create/update accept exactly name plus the three can_access_* flags
// (additionalProperties: false in the hyperschema), so the response-only
// fields carry omitempty and are never set on requests.
type accessTokenAttributes struct {
	Name                string `json:"name"`
	CanAccessCda        bool   `json:"can_access_cda"`
	CanAccessCdaPreview bool   `json:"can_access_cda_preview"`
	CanAccessCma        bool   `json:"can_access_cma"`
	// Token is response-only: the secret value, returned on every endpoint
	// when the caller's role has can_manage_access_tokens, null otherwise.
	Token *string `json:"token,omitempty"`
	// NOTE: responses also carry hardcoded_type (factory-token marker),
	// last_cma_access and last_cda_access (usage enums); intentionally not
	// decoded/exposed for now.
}

// accessTokenRelationships mirrors the access_token relationships object. The
// only relationship is role, whose data is a role linkage ({type,id} object;
// the CMA rejects null), so the field must serialize explicitly (no
// omitempty).
type accessTokenRelationships struct {
	Role struct {
		Data *roleLinkage `json:"data"`
	} `json:"role"`
}

// accessTokenData is the JSON:API resource object for an access token.
type accessTokenData struct {
	ID            string                    `json:"id,omitempty"`
	Type          string                    `json:"type"`
	Attributes    accessTokenAttributes     `json:"attributes"`
	Relationships *accessTokenRelationships `json:"relationships,omitempty"`
}

// accessTokenPayload wraps a single access token document.
type accessTokenPayload struct {
	Data accessTokenData `json:"data"`
}

// newAccessTokenRelationships builds the relationships object: a role linkage
// when roleID is non-nil. NOTE: the CMA rejects a null role linkage on both
// create and update (422 INVALID_FORMAT, verified against the live API and
// the hyperschema: role.data must be a {type,id} object), so callers must
// always pass a non-nil roleID; the resource schema enforces this by making
// role_id required.
func newAccessTokenRelationships(roleID *string) *accessTokenRelationships {
	rels := &accessTokenRelationships{}
	if roleID != nil {
		rels.Role.Data = &roleLinkage{Type: "role", ID: *roleID}
	}
	return rels
}

// CreateAccessToken creates an API token via POST /access_tokens. The
// response carries the token secret in attributes.token.
func (c *DatoCMSClient) CreateAccessToken(ctx context.Context, attrs accessTokenAttributes, roleID *string) (*accessTokenData, error) {
	body := accessTokenPayload{Data: accessTokenData{
		Type:          "access_token",
		Attributes:    attrs,
		Relationships: newAccessTokenRelationships(roleID),
	}}
	var out accessTokenPayload
	if err := c.doRequest(ctx, http.MethodPost, "/access_tokens", body, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// GetAccessToken retrieves an API token via GET /access_tokens/{id}. Returns
// an error wrapping errNotFound on 404. attributes.token is null when the
// caller's role lacks can_manage_access_tokens.
func (c *DatoCMSClient) GetAccessToken(ctx context.Context, id string) (*accessTokenData, error) {
	var out accessTokenPayload
	if err := c.doRequest(ctx, http.MethodGet, "/access_tokens/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// UpdateAccessToken updates an API token via PUT /access_tokens/{id}. The
// relationships object is always sent so the configured role (or its absence)
// is authoritative; the token secret is never affected by updates (rotation
// happens via POST /access_tokens/{id}/regenerate_token, not modeled here).
func (c *DatoCMSClient) UpdateAccessToken(ctx context.Context, id string, attrs accessTokenAttributes, roleID *string) (*accessTokenData, error) {
	body := accessTokenPayload{Data: accessTokenData{
		ID:            id,
		Type:          "access_token",
		Attributes:    attrs,
		Relationships: newAccessTokenRelationships(roleID),
	}}
	var out accessTokenPayload
	if err := c.doRequest(ctx, http.MethodPut, "/access_tokens/"+url.PathEscape(id), body, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

// DeleteAccessToken deletes an API token via DELETE /access_tokens/{id}. If
// the token owns resources (records, uploads, filters, editing sessions), the
// API requires the destination_user_type/destination_user_id query parameters
// to transfer ownership; that transfer is not modeled here, so such deletes
// fail with an explanatory API error. A token also cannot delete itself
// (CANNOT_DESTROY_CURRENT_USER).
func (c *DatoCMSClient) DeleteAccessToken(ctx context.Context, id string) error {
	return c.doRequest(ctx, http.MethodDelete, "/access_tokens/"+url.PathEscape(id), nil, nil)
}
