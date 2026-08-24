provider "datocms" {
  # Default token, used by resources that do not set the `project` attribute.
  # Can be omitted if the DATOCMS_API_TOKEN environment variable is set.
  api_token = var.datocms_api_token

  # Optional: manage multiple DatoCMS projects from a single provider
  # configuration. Each key is an arbitrary label that resources reference
  # through their `project` attribute.
  api_tokens = {
    "store-one" = var.datocms_api_token_store_one
    "store-two" = var.datocms_api_token_store_two
  }

  # Optional, defaults to https://site-api.datocms.com
  # base_url = "https://site-api.datocms.com"
}

# Uses the default token (api_token / DATOCMS_API_TOKEN).
resource "datocms_role" "editor" {
  name = "content_editor"
}

# Uses the token of the "store-one" entry of api_tokens.
resource "datocms_role" "store_one_editor" {
  project = "store-one"
  name    = "content_editor"
}
