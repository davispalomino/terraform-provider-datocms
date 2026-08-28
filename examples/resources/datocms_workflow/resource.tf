# Editorial workflow mirroring the "New workflow" screen of the DatoCMS UI
# (Settings > Workflows): content moves from writing to review, approval and
# publication. Exactly one stage must set initial = true; records enter the
# workflow at that stage.
resource "datocms_workflow" "approval_by_publisher" {
  # UI: "Name" and "API identifier".
  name    = "Approval by publisher"
  api_key = "approval_by_publisher"

  # UI: "Workflow stages" > "Add new stage". Each element is one stage, in
  # order; description is optional.
  stages = [
    {
      id          = "work_in_progress"
      name        = "Work in progress"
      description = "Content is being written"
      initial     = true
    },
    {
      id          = "in_review"
      name        = "In review"
      description = "Editor has finished writing and is waiting for approval"
    },
    {
      id   = "approved"
      name = "Approved"
    },
    {
      id   = "ready_to_publish"
      name = "Ready to publish"
    },
    {
      id          = "changes_requested"
      name        = "Changes requested"
      description = "The reviewer asked for changes before approval"
    },
  ]
}
