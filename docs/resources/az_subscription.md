---
page_title: "Azure Subscription – by CANCOM Marketplace"
subcategory: ""
description: |-
  Manages an Azure Subscription within the Cancom Marketplace.
---

# Azure Subscription – by CANCOM Marketplace

Manages an Azure Subscription within the Cancom Marketplace.

Before creating a subscription that uses Azure-backed properties, the provider verifies Azure authentication and reads the Default Management Group hierarchy settings. This prevents creating a Marketplace subscription when the Azure follow-up operation would fail. After creation, the provider polls `GET /v1/subscriptions/{subscriptionId}` every five seconds until `data.order.status` is `ACTIVE`. Follow-up changes are only sent after the subscription is ready. Creation times out after 30 minutes by default.

> **Note:**  Destroying the resource will cancel the subscription. You can delete the subscription after a period of 7 days manually from the Azure Portal. After 30 days, the subscription will be deleted automatically.

## Example Usage

```terraform
resource "cancom-marketplace_az_subscription" "example" {
  display_name          = "My new CANCOM Azure Subscription"
  azure_owner_object_id = "11111111-1111-1111-1111-111111111111"
}
```

## Attributes

### Optional

- `display_name` (string) - The display name of the subscription.
- `azure_owner_object_id` (string) - The Azure principal object ID that receives the `Owner` role on the created Azure subscription.

> **Note:** `display_name` is updated through the CANCOM Marketplace subscription API.
> **Note:** `azure_owner_object_id` creates an Azure role assignment and requires Azure authentication plus sufficient permissions inherited from the Default Management Group.

### Read-Only Attributes

- `subscription_id` (string) - The Azure subscription ID returned in `externalAccountId`.
- `marketplace_subscription_id` (string) - The CANCOM Marketplace subscription ID returned in `id` and used for Marketplace API operations.
- `payment_plan_id` (int) - The fixed payment plan ID (`172495`) used for Azure subscriptions.


## Import

Import a subscription using its CANCOM Marketplace subscription UUID (`data.id`), not its Azure subscription ID.

```shell
terraform import cancom-marketplace_az_subscription.example 00000000-0000-0000-0000-000000000000
```
