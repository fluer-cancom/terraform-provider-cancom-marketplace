---
page_title: "Azure Subscription"
subcategory: ""
description: |-
  Manages an Azure Subscription within the Cancom Marketplace.
---

# Azure Subscription – by CANCOM Marketplace

Manages an Azure Subscription within the Cancom Marketplace.

Subscriptions are created for the Marketplace user configured on the provider with `marketplace_user_email`. The resource itself does not accept a user UUID.

Before creating a subscription that uses Azure-backed properties, the provider verifies Azure authentication and reads the Default Management Group hierarchy settings. This prevents creating a Marketplace subscription when the Azure follow-up operation would fail. After creation, the provider polls `GET /v1/subscriptions/{subscriptionId}` every five seconds until `data.order.status` is `ACTIVE`. Follow-up changes are only sent after the subscription is ready. Creation times out after 30 minutes by default.

> **Note:**  Destroying the resource will cancel the subscription. You can delete the subscription after a period of 7 days manually from the Azure Portal. After 30 days, the subscription will be deleted automatically.

## Example Usage

### Minimal Configuration

```terraform
resource "cancom-marketplace_az_subscription" "example" {}
```

### Full Configuration

```terraform
resource "cancom-marketplace_az_subscription" "example" {
  display_name          = "My new CANCOM Azure Subscription"
  azure_owner_object_id = "11111111-1111-1111-1111-111111111111"
}
```

## Attributes

### Optional

- `display_name` (string) - The display name of the subscription. If set during create, the provider performs Azure preflight before the Marketplace subscription is created.
- `azure_owner_object_id` (string) - The Azure principal object ID that receives the `Owner` role on the created Azure subscription.

> **Note:** `display_name` is updated through the Azure Management API.
> **Note:** `azure_owner_object_id` creates an Azure role assignment and requires Azure authentication plus sufficient permissions inherited from the Default Management Group.

## Azure-Backed Behavior

When `display_name` is set, the provider renames the Azure subscription through the Azure Management API after the Marketplace subscription is created and the Azure subscription ID is available.

When `azure_owner_object_id` is set, the provider creates an Azure role assignment after the Marketplace subscription is created and the Azure subscription ID is available.

The role assignment uses:

- Scope: `/subscriptions/{subscription_id}`
- Role: `Owner`
- Principal: `azure_owner_object_id`

Changing `display_name` or `azure_owner_object_id` only calls the Azure Management API. It does not modify the Marketplace subscription document.

Destroying this resource runs an Azure subscription cancel operation through the Azure Management API after an Azure preflight check.

### Read-Only Attributes

- `subscription_id` (string) - The Azure subscription ID.
- `marketplace_subscription_id` (string) - The CANCOM Marketplace subscription ID returned; only used for Marketplace API operations.
- `payment_plan_id` (int) - The payment plan ID (fixed on value `172495`).


## Import

To import a subscription use its CANCOM Marketplace subscription UUID (`data.id`), **not** its Azure subscription ID.

```shell
terraform import cancom-marketplace_az_subscription.example 00000000-0000-0000-0000-000000000000
```
