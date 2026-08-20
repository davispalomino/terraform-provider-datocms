variable "webhook_http_basic_user" {
  description = "HTTP Basic Authorization username sent with the webhook call."
  type        = string
  sensitive   = true
}

variable "webhook_http_basic_password" {
  description = "HTTP Basic Authorization password sent with the webhook call."
  type        = string
  sensitive   = true
}

# Webhook that notifies an event gateway whenever a record of the blog
# models is published or unpublished, using a custom Mustache payload.
# Comments reference the section of the "Create a new webhook" form in the
# DatoCMS UI each attribute group belongs to.
resource "datocms_webhook" "records_publish" {
  # UI: "Name", "Enabled?" and "Automatic retries?" (general fields).
  name       = "Publish/Unpublish records webhook"
  enabled    = true
  auto_retry = false

  # UI: Triggers > "Add new trigger". Each element is one trigger.
  events = [
    {
      entity_type = "item"
      event_types = ["publish", "unpublish"]

      # Only trigger for records of these models.
      filters = [
        {
          entity_type = "item_type"
          entity_ids  = ["article", "author", "category"]
        }
      ]
    }
  ]

  # UI: HTTP Settings > "URL", "HTTP basic auth" and "Custom headers".
  url                 = "https://events.example.com/datocms/records"
  http_basic_user     = var.webhook_http_basic_user
  http_basic_password = var.webhook_http_basic_password
  headers             = {}

  # UI: HTTP Body > "Payload format", "Expand nested blocks in records?" and
  # "Send a custom payload?". Setting custom_payload turns the toggle on; the
  # template is rendered with Mustache against the default webhook payload.
  payload_api_version     = "3"
  nested_items_in_payload = false
  custom_payload          = <<-EOT
    {
      "message": "{{event_type}} event triggered on {{entity_type}}!",
      "entity_id": "{{#entity}}{{id}}{{/entity}}"
    }
  EOT
}
