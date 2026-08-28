# Workflows can be imported using the workflow ID
# (shown in the URL when editing the workflow in the DatoCMS UI, or returned by GET /workflows).
terraform import datocms_workflow.approval_by_publisher 949

# When the resource uses a project from the provider's api_tokens map,
# prefix the ID with the project key ("project/id").
terraform import datocms_workflow.approval_by_publisher store-one/949
