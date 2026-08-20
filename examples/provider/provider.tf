provider "datocms" {
  # api_token can be omitted if the DATOCMS_API_TOKEN environment variable is set
  api_token = var.datocms_api_token

  # Optional, defaults to https://site-api.datocms.com
  # base_url = "https://site-api.datocms.com"
}
