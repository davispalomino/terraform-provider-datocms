# Webhooks can be imported using the webhook ID
# (shown in the URL when editing the webhook in the DatoCMS UI, or returned by GET /webhooks).
# Note: the API returns http_basic_user and http_basic_password on GET,
# so the import also populates the basic auth credentials.
terraform import datocms_webhook.store_publish 12345

# When the resource uses a project from the provider's api_tokens map,
# prefix the ID with the project key ("project/id").
terraform import datocms_webhook.store_publish store-one/12345
