# Terraform Provider for DatoCMS

A Terraform provider to manage [DatoCMS](https://www.datocms.com/) project configuration through the [Content Management API](https://www.datocms.com/docs/content-management-api), built on the [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework).

> **Note:** this is not an official DatoCMS provider. It is an independent, community-maintained project with no affiliation with DatoCMS.

## Resources

| Resource | Manages | DatoCMS UI location |
|---|---|---|
| `datocms_access_token` | API tokens (CMA resource `access_token`) | Project Settings > API Tokens |
| `datocms_role` | Roles and their permissions | Settings > Roles |
| `datocms_webhook` | Webhooks | Settings > Webhooks |

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.24 (only to build the provider from source)

## Example Usage

```terraform
provider "datocms" {
  # Can be omitted if the DATOCMS_API_TOKEN environment variable is set.
  api_token = var.datocms_api_token
}

resource "datocms_role" "editor" {
  name = "content_editor"

  # Environment access: "On primary environment" on, "On sandbox environments" off.
  environments_access = "primary_only"

  # Content permissions: full access to every model in the primary environment.
  positive_item_type_permissions = [
    {
      action             = "all"
      environment        = "main"
      on_creator         = "anyone"
      localization_scope = "all"
    },
  ]
}

resource "datocms_webhook" "publish_notifications" {
  name = "Publish notifications"
  url  = "https://example.com/hooks/datocms"

  events = [
    {
      entity_type = "item"
      event_types = ["publish", "unpublish"]
    }
  ]
}

# API token that can only read published content via the Content Delivery API.
# The generated secret is the sensitive `token` attribute (stored in the state).
resource "datocms_access_token" "frontend" {
  name           = "Frontend CDA token"
  role_id        = datocms_role.editor.id
  can_access_cda = true
}
```

## Mapping from the DatoCMS UI

Attribute names follow the Content Management API, while the DatoCMS UI groups them into sections of each settings screen. Short version of the mapping for each resource:

### `datocms_role` ("Create a new role" screen)

| DatoCMS UI section | Terraform attributes |
|---|---|
| Project and environment management | `can_edit_site`, `can_manage_environments`, `can_promote_environments`, `can_manage_users`, `can_manage_sso` |
| Automations and integrations | `can_manage_webhooks`, `can_manage_access_tokens`, `can_manage_build_triggers`, `can_manage_search_indexes`, `can_perform_site_search` |
| Activity tracking | `can_access_build_events_log`, `can_access_search_index_events_log`, `can_access_audit_log` |
| Environment access (two toggles) | `environments_access` (`all`, `primary_only`, `sandbox_only`, `none`) |
| Environment permissions | `can_edit_favicon`, `can_manage_menu`, `can_edit_schema`, `can_edit_environment`, `can_manage_workflows`, `can_manage_shared_filters`, `can_manage_upload_collections` |
| Content permissions (records and assets) | `positive_item_type_permissions`, `negative_item_type_permissions`, `positive_upload_permissions`, `negative_upload_permissions` |
| Build permissions | `positive_build_trigger_permissions`, `negative_build_trigger_permissions` |
| Permissions to index with Search Indexes | `positive_search_index_permissions`, `negative_search_index_permissions` |
| Inherits permissions from | `inherits_permissions_from` |

The full toggle-by-toggle mapping, including the `environments_access` combination table, lives in [docs/resources/role.md](docs/resources/role.md).

### `datocms_webhook` ("Create a new webhook" screen)

| DatoCMS UI field | Terraform attribute |
|---|---|
| Name | `name` |
| Enabled? | `enabled` |
| Automatic retries? | `auto_retry` |
| Triggers > Add new trigger | `events` |
| HTTP Settings > URL | `url` |
| HTTP Settings > HTTP basic auth (User / Password) | `http_basic_user` / `http_basic_password` |
| HTTP Settings > Custom headers | `headers` |
| HTTP Body > Payload format | `payload_api_version` |
| HTTP Body > Expand nested blocks in records? | `nested_items_in_payload` |
| HTTP Body > Send a custom payload? | `custom_payload` (set = toggle on, omitted = off) |

Full mapping in [docs/resources/webhook.md](docs/resources/webhook.md).

### `datocms_access_token` ("New API Token" screen)

| DatoCMS UI field | Terraform attribute |
|---|---|
| Name | `name` |
| Permissions > Role associated with this API token | `role_id` |
| Permissions > Access the Content Delivery API | `can_access_cda` |
| Permissions > Access the Content Delivery API in Preview Mode | `can_access_cda_preview` |
| Permissions > Access the Content Management API | `can_access_cma` |

Full mapping in [docs/resources/access_token.md](docs/resources/access_token.md). The generated token secret is stored in the Terraform state; treat the state as sensitive.

## Authentication

The provider needs a DatoCMS full-access (or suitably scoped) API token, created in the DatoCMS UI under Project Settings > API Tokens. It can be supplied in two ways:

- The `DATOCMS_API_TOKEN` environment variable (recommended):

  ```shell
  export DATOCMS_API_TOKEN="xxxxxxxxxxxxxxxx"
  ```

- The `api_token` provider argument, which takes precedence over the environment variable.

## DatoCMS API compatibility

- The provider sends `X-Api-Version: 3` on every request (hardcoded in the API client). Version 3 is the current version of the [Content Management API](https://www.datocms.com/docs/content-management-api), and the header is mandatory on every CMA call per the official [API versioning policy](https://www.datocms.com/docs/content-management-api/api-versioning).
- DatoCMS guarantees that breaking changes (endpoint path changes, request/response payload format changes) are introduced only with a new API version. Additive changes (new optional request attributes, new response fields, required attributes becoming optional) can happen within version 3 without notice and do not break this provider.
- The `datocms_role`, `datocms_webhook` and `datocms_access_token` schemas were validated against the official CMA hyperschema at <https://site-api.datocms.com/docs/site-api-hyperschema.json> on 2026-08-20 (server `Last-Modified: Fri, 14 Aug 2026 06:29:21 GMT`, snapshot sha256 `35726a1e0a6786ac66a3ef83419308506a810e7cc83ef0a1cf6bf5d280cee5ef`).
- If DatoCMS publishes a new API version or changes the schema of these resources, this provider may require a new release. Watch the official [product updates](https://www.datocms.com/product-updates) page for announcements.

## Development

Build the provider from source:

```shell
go build ./...
```

To run a local build against real configurations without publishing it, add a [`dev_overrides`](https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides-for-provider-developers) block to your CLI configuration file (`~/.terraformrc`):

```hcl
provider_installation {
  dev_overrides {
    "registry.terraform.io/davispalomino/datocms" = "/path/to/your/go/bin"
  }
  direct {}
}
```

Then run `go install .` and use `terraform plan`/`terraform apply` as usual (skip `terraform init`, which is not needed with dev overrides).

Other useful targets from the `GNUmakefile`:

```shell
make fmt       # gofmt
make lint      # golangci-lint
make test      # unit tests
make testacc   # acceptance tests (TF_ACC=1, hits a real DatoCMS project)
make generate  # regenerate docs/ with tfplugindocs
```

## Documentation

- [Provider](docs/index.md)
- [datocms_access_token](docs/resources/access_token.md)
- [datocms_role](docs/resources/role.md)
- [datocms_webhook](docs/resources/webhook.md)
