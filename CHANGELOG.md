# Changelog

## 2.0.0

Changes since `v1.0.7`.

### Breaking Changes

- Removed `user_uuid` from `cancom-marketplace_az_subscription`.
  Subscriptions are now created for the Marketplace user configured on the provider with `marketplace_user_email`.
- Added required provider argument `marketplace_user_email`.
  During provider initialization, the provider resolves this email through paginated `GET /v1/users` calls and uses the matching Marketplace user UUID for subscription creation.
- Changed `azure_owner_object_id` behavior.
  It no longer mutates the Marketplace subscription document. It is now treated as an Azure principal object ID and creates an Azure `Owner` role assignment on the created subscription.
- Changed `display_name` behavior.
  It is now applied through the Azure Management API subscription rename operation instead of the Marketplace subscription API.
- Subscription updates no longer modify the Marketplace subscription document.
  Updates only call Azure APIs for `display_name` and `azure_owner_object_id`.
- Destroy now performs an Azure preflight check before canceling the Azure subscription through the Azure Management API.
- Provider module path was corrected to `terraform-provider-cancom-marketplace`.

### Migration Notes

- Move the subscription owner selection from the resource to the provider:

  ```hcl
  provider "cancom-marketplace" {
    api_client_id          = "..."
    api_client_secret      = "..."
    marketplace_user_email = "user@example.com"
  }

  resource "cancom-marketplace_az_subscription" "example" {
    display_name = "My Azure Subscription"
  }
  ```

- Remove `user_uuid` from all `cancom-marketplace_az_subscription` resources.
- If you use `display_name`, `azure_owner_object_id`, or destroy subscriptions, make sure Azure authentication is available through `az login`, Azure SDK default credentials, or the provider Azure credentials.
- If one of `azure_client_id`, `azure_client_secret`, or `azure_tenant_id` is set, all three must be set.
- The Azure identity must have the required permissions inherited from the Default Management Group for Azure-backed operations.

### Added

- Added environment variable support for provider configuration:
  - `CANCOM_MARKETPLACE_API_CLIENT_ID`
  - `CANCOM_MARKETPLACE_API_CLIENT_SECRET`
  - `CANCOM_MARKETPLACE_USER_EMAIL`
  - `CANCOM_MARKETPLACE_API_SCOPE`
  - `CANCOM_MARKETPLACE_ENDPOINT`
  - `CANCOM_MARKETPLACE_AZURE_CLIENT_ID`
  - `CANCOM_MARKETPLACE_AZURE_CLIENT_SECRET`
  - `CANCOM_MARKETPLACE_AZURE_TENANT_ID`
- Added paginated Marketplace user lookup through `/v1/users`.
- Added Azure preflight checks before Azure-backed subscription create/update/destroy operations.
- Added Default Management Group hierarchy settings check before Azure-backed subscription creation.
- Added Azure Owner role assignment support via `azure_owner_object_id`.
- Added Azure subscription rename support via `display_name`.
- Added Azure subscription quota resource `cancom-marketplace_az_subscription_quota`.
- Added Azure subscription data source documentation.
- Added troubleshooting documentation for Marketplace user lookup, Azure authentication, Azure preflight, role assignment, rename, cancel, and quota request failures.
- Added focused tests for provider configuration, Marketplace pagination, Azure REST helpers, subscription lifecycle behavior, quota behavior, and docs-backed schema changes.

### Changed

- Refactored provider internals into `internal/provider`, `internal/marketplace`, and `internal/azure`.
- Marketplace API logic is now isolated in a Marketplace client.
- Azure Management API logic is now isolated in an Azure client.
- Subscription create now sets Terraform state immediately after the Marketplace create response before waiting for `ACTIVE`.
- Subscription create waits for Marketplace order status `ACTIVE` before running Azure-backed follow-up actions.
- `display_name` reads are sourced from Azure when the field is tracked by Terraform.
- Quota operations use the Marketplace gateway Microsoft Quota API proxy and poll quota request state.
- Documentation has been expanded under `docs/`, including provider configuration, resources, data sources, and troubleshooting.

### Fixed

- Prevented Marketplace subscriptions from being created when Azure authentication or Azure preflight fails for Azure-backed properties.
- Prevented tainted/orphaned state caused by Azure follow-up failures occurring before Terraform could persist the Marketplace subscription ID.
- Preserved unknown Marketplace subscription API fields when reading subscription responses.
- Accepted both wrapped `{"data": ...}` and bare subscription objects from Marketplace subscription responses.
- Improved error handling for missing Marketplace subscription IDs, missing Azure subscription IDs, non-200 Marketplace responses, and quota request failures.

### Removed

- Removed root-level provider/resource helper files in favor of internal packages.
- Removed Marketplace document mutation for `azure_owner_object_id`.
- Removed Marketplace document mutation for `display_name`.
