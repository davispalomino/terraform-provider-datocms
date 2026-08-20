# Access tokens can be imported using the access token ID
# (shown in the URL when editing the token in the DatoCMS UI, or returned by GET /access_tokens).
# Note: the token secret is populated only if the credential used by the provider
# has can_manage_access_tokens; otherwise the API masks it and `token` stays empty.
terraform import datocms_access_token.frontend 12345
