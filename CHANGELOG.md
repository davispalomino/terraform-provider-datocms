## 1.1.0 (Unreleased)

FEATURES:

* provider: new optional `api_tokens` attribute (map of project keys to API tokens) to manage multiple DatoCMS projects from a single provider configuration
* resource/datocms_role, resource/datocms_webhook, resource/datocms_access_token: new optional `project` attribute selecting the `api_tokens` entry used for the resource; changing it forces replacement
* all resources: import now accepts a compound `project/id` import ID (a plain ID keeps using the default token)

## 0.1.0 (Unreleased)

FEATURES:
