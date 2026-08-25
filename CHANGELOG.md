## 1.2.0

FEATURES:

* resource/datocms_role: omitted item/upload permission lists (`positive_item_type_permissions`, `negative_item_type_permissions`, `positive_upload_permissions`, `negative_upload_permissions`) are now preserved as-is on the platform (Computed with `UseStateForUnknown`); declare them (even as `[]`) to have Terraform manage them. This lets Terraform manage only the role form ("first screen") while content permission rules keep being edited in the DatoCMS UI.

## 1.1.0

FEATURES:

* provider: new optional `api_tokens` attribute (map of project keys to API tokens) to manage multiple DatoCMS projects from a single provider configuration
* resource/datocms_role, resource/datocms_webhook, resource/datocms_access_token: new optional `project` attribute selecting the `api_tokens` entry used for the resource; changing it forces replacement
* all resources: import now accepts a compound `project/id` import ID (a plain ID keeps using the default token)

## 0.1.0 

FEATURES:
