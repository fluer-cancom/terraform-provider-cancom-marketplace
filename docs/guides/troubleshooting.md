---
page_title: "Troubleshooting"
subcategory: ""
description: |-
  Troubleshooting common CANCOM Marketplace Provider errors.
---

# Troubleshooting

## Marketplace User Not Found

Error:

```text
User not found in CANCOM Marketplace. Contact your Enterprise Administrator
```

The provider resolves `marketplace_user_email` during initialization by reading all pages from `/v1/users` and matching `content[].email`. Check that the email address is exactly the Marketplace user's email address and that the API client can list users.

## Marketplace User Is Ambiguous

Error:

```text
User is ambigous in CANCOM Marketplace. Contact your Enterprise Administrator
```

More than one Marketplace user has the configured email address. Terraform cannot safely decide which `content[].uuid` should be used for subscription creation. Contact your Enterprise Administrator to clean up or disambiguate the Marketplace users.

## Azure Authentication Is Required

Errors may mention `az login`, `azure_client_id`, `azure_client_secret`, or `azure_tenant_id`.

Azure credentials are needed for Azure-backed operations such as renaming a subscription from `display_name`, assigning the `Owner` role from `azure_owner_object_id`, or canceling a subscription during destroy. Run `az login` in the environment where Terraform runs, or set all three Azure provider arguments:

- `azure_client_id`
- `azure_client_secret`
- `azure_tenant_id`

If one of these provider arguments is set, all three must be set.

The provider-specific Azure arguments can also be supplied with:

- `CANCOM_MARKETPLACE_AZURE_CLIENT_ID`
- `CANCOM_MARKETPLACE_AZURE_CLIENT_SECRET`
- `CANCOM_MARKETPLACE_AZURE_TENANT_ID`

The Azure SDK default credential chain is also used when the provider-specific Azure arguments are not set.

## Missing Marketplace Provider Configuration

If `api_client_id`, `api_client_secret`, or `marketplace_user_email` is missing, set it in the provider block or use the corresponding environment variable:

- `CANCOM_MARKETPLACE_API_CLIENT_ID`
- `CANCOM_MARKETPLACE_API_CLIENT_SECRET`
- `CANCOM_MARKETPLACE_USER_EMAIL`

Optional Marketplace settings can also be supplied with:

- `CANCOM_MARKETPLACE_API_SCOPE`
- `CANCOM_MARKETPLACE_ENDPOINT`

## Azure Preflight Failed

The provider performs Azure preflight before creating a Marketplace subscription when Azure-backed properties are configured. This intentionally happens before the Marketplace create request to avoid a subscription being created remotely but not safely managed in Terraform state.

The preflight reads Default Management Group hierarchy settings through:

```text
GET /providers/Microsoft.Management/managementGroups/root/settings/default?api-version=2020-05-01
```

Ensure the Azure identity can authenticate to Azure Resource Manager and has the required permissions inherited from the Default Management Group.

## Owner Role Assignment Failed

`azure_owner_object_id` creates an Azure role assignment after the subscription becomes `ACTIVE` and the Azure subscription ID is available.

The provider assigns:

- Scope: `/subscriptions/{subscription_id}`
- Role: `Owner`
- Principal: `azure_owner_object_id`

The Azure identity used by the provider must be allowed to create role assignments at the new subscription scope, usually through permissions inherited from the Default Management Group.

## Display Name Rename Failed

`display_name` is applied through the Azure Management API, not through the Marketplace subscription document. The provider waits until the Marketplace subscription is `ACTIVE` and `externalAccountId` is available, then calls the Azure subscription rename operation.

The Azure identity used by the provider must be allowed to rename subscriptions.

## Subscription Cancel Failed

Destroying `cancom-marketplace_az_subscription` cancels the Azure subscription through the Azure Management API. Before canceling, the provider runs Azure preflight for the cancel operation. Ensure the Azure identity has permission to cancel the subscription.

## Quota Request Failed Or Timed Out

Quota resources poll Azure quota requests until they finish. If a request reaches `Failed` or `Canceled`, the provider returns an error with the request state. If polling exceeds the timeout, increase the resource timeout or inspect the quota request in Azure.

Example timeout override:

```terraform
resource "cancom-marketplace_az_subscription_quota" "example" {
  subscription_id    = cancom-marketplace_az_subscription.example.subscription_id
  provider_namespace = "Microsoft.Compute"
  location           = "westeurope"
  quota_resource     = "standardDSv3Family"
  limit              = 20

  timeouts {
    create = "60m"
    update = "60m"
  }
}
```
