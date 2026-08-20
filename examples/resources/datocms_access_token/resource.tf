# Role the token will be bound to. The token's effective capabilities are the
# intersection of this role and the can_access_* surface flags below.
resource "datocms_role" "example" {
  name                = "frontend_readonly"
  environments_access = "primary_only"
}

# API token for a frontend that only reads published content.
# Comments reference the fields of the "New API Token" form in the DatoCMS UI
# (Project Settings > API Tokens) each attribute belongs to.
resource "datocms_access_token" "frontend" {
  # UI: "Name".
  name = "Frontend CDA token"

  # UI: Permissions > "Role associated with this API token".
  # Required: the Content Management API rejects tokens without a role.
  role_id = datocms_role.example.id

  # UI: Permissions section, the three API surface toggles (all default to
  # false when omitted):
  # - "Access the Content Delivery API" (GraphQL endpoint
  #   https://graphql.datocms.com/, published content only)
  # - "Access the Content Delivery API in Preview Mode" (draft content)
  # - "Access the Content Management API" (the UI warns this may allow write
  #   or administrative operations depending on the role; use with caution)
  can_access_cda         = true
  can_access_cda_preview = false
  can_access_cma         = false
}

# The generated secret is available as the sensitive `token` attribute (it is
# stored in the Terraform state; protect the state backend accordingly).
output "frontend_token" {
  value     = datocms_access_token.frontend.token
  sensitive = true
}
