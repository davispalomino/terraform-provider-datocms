# Roles can be imported using the role ID
# (shown in the URL when editing the role in the DatoCMS UI, or returned by GET /roles).
terraform import datocms_role.store_developer 000003

# When the resource uses a project from the provider's api_tokens map,
# prefix the ID with the project key ("project/id").
terraform import datocms_role.store_developer store-one/000003
