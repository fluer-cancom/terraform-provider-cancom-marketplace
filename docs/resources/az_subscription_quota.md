---
page_title: "Azure Quota"
subcategory: ""
description: |-
  Manages quota requests for an Azure Subscription through the CANCOM Marketplace gateway.
---

# Azure Subscription Quota - by CANCOM Marketplace

Manages quota limits for an Azure subscription through the CANCOM Marketplace gateway and the Microsoft Quota API proxy.

Create and update operations submit a quota request and poll until the request finishes. The default create and update timeout is 30 minutes.

Delete does not remove quota from Azure. Quotas cannot be deleted through this resource, so delete only removes the resource from Terraform state.

## Example Usage

```terraform
resource "cancom-marketplace_az_subscription_quota" "compute_family" {
  subscription_id    = cancom-marketplace_az_subscription.example.subscription_id
  provider_namespace = "Microsoft.Compute"
  location           = "westeurope"
  quota_resource     = "standardDSv3Family"
  limit              = 20
}
```

## Attributes

### Required

- `subscription_id` (string) - The Azure subscription ID.
- `provider_namespace` (string) - The resource provider namespace for the quota, for example `Microsoft.Compute`.
- `location` (string) - The Azure region for the quota.
- `quota_resource` (string) - The Microsoft Quota resource name to manage.
- `limit` (int) - The desired quota limit.

### Optional

- `quota_family` (string) - Deprecated legacy field. Use `quota_resource`.

### Read-Only Attributes

- `provisioning_state` (string) - Provisioning state returned by the quota request, for example `InProgress`, `Succeeded`, `Failed`, or `Canceled`.
- `request_id` (string) - The quota request ID returned by Azure.

## Import

Import a quota request using:

```text
subscription_id,provider_namespace,location,quota_request_id
```

Example:

```shell
terraform import cancom-marketplace_az_subscription_quota.example 00000000-0000-0000-0000-000000000000,Microsoft.Compute,westeurope,request-id
```
