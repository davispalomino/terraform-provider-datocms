# Webhooks can be imported using the webhook ID
# (shown in the URL when editing the webhook in the DatoCMS UI, or returned by GET /webhooks).
# Note: http_basic_password is not returned by the API and must be re-set in the configuration.
terraform import datocms_webhook.store_publish 12345
