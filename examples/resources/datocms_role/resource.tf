# Developer role mirroring the "Create a new role" screen of the DatoCMS UI:
# no project administration flags, access to the primary environment only,
# full content access on "main" and workflow-restricted access on
# "production"/"staging", inheriting from a base model-editor role.
# Comments reference the UI section each attribute group belongs to.
resource "datocms_role" "store_developer" {
  name = "datocms_store_developer"

  # UI: Activity tracking > "Access Search index activity log"
  can_access_search_index_events_log = true

  # UI: Environment access. "On primary environment" on and
  # "On sandbox environments" off maps to "primary_only".
  environments_access = "primary_only"

  # UI: Content permissions > "Records and assets permissions" (records rules)
  positive_item_type_permissions = [
    # Full access to every model in the "main" environment.
    {
      action             = "all"
      environment        = "main"
      on_creator         = "anyone"
      localization_scope = "all"
    },
    # In "production", access restricted to records under workflow 000002.
    {
      action             = "all"
      environment        = "production"
      workflow           = "000002"
      on_creator         = "anyone"
      localization_scope = "all"
    },
    {
      action             = "all"
      environment        = "staging"
      workflow           = "000002"
      on_creator         = "anyone"
      localization_scope = "all"
    },
    {
      action             = "update"
      environment        = "staging"
      workflow           = "000002"
      on_creator         = "anyone"
      localization_scope = "all"
    },
  ]

  # UI: Content permissions > "Records and assets permissions" (media rules)
  positive_upload_permissions = [
    # Full access to uploads of every collection in "staging".
    {
      action             = "all"
      environment        = "staging"
      on_creator         = "anyone"
      localization_scope = "all"
    },
  ]

  # UI: Build permissions > "Add new rule".
  # An entry without build_trigger covers every build trigger.
  positive_build_trigger_permissions = [
    {}
  ]

  # UI: Permissions to index with Search Indexes > "Add new rule".
  # An entry without search_index covers every search index.
  positive_search_index_permissions = [
    {}
  ]

  # UI: "Inherits permissions from" select.
  # Union the permissions of the model-editor role into this one.
  inherits_permissions_from = [
    "000001",
  ]
}
