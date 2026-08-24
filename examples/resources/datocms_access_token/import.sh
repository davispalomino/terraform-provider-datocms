# Access tokens can be imported using the access token ID
# (shown in the URL when editing the token in the DatoCMS UI, or returned by GET /access_tokens).
# Note: the token secret is populated only if the credential used by the provider
# has can_manage_access_tokens; otherwise the API masks it and `token` stays empty.
terraform import datocms_access_token.frontend 12345

# When the resource uses a project from the provider's api_tokens map,
# prefix the ID with the project key ("project/id").
terraform import datocms_access_token.frontend store-one/12345
