---
page_title: "Azure Subscription – by CANCOM Marketplace"
subcategory: ""
description: |-
  Manages an Azure Subscription within the Cancom Marketplace.
---

# Azure Subscription – by CANCOM Marketplace

Manages an Azure Subscription within the Cancom Marketplace.

After creation, the provider polls `GET /v1/subscriptions/{subscriptionId}` every five seconds until `data.order.status` is `ACTIVE`. Follow-up changes are only sent after the subscription is ready. Creation times out after 30 minutes by default.

> **Note:**  Destroying the resource will cancel the subscription. You can delete the subscription after a period of 7 days manually from the Azure Portal. After 30 days, the subscription will be deleted automatically.

## Example Usage

```terraform
resource "cancom-marketplace_az_subscription" "example" {
  user_uuid    = "12345678-1234-1234-1234-123456789012"
  display_name = "My new CANCOM Azure Subscription"
}
```

## Attributes

### Required

- `user_uuid` (string) - The marketplace user UUID for which the subscription is created.

### Optional

- `display_name` (string) - The display name of the subscription.
- `azure_owner_object_id` (string) - Deprecated legacy alias for the marketplace user UUID. It is not an Azure AD object ID.

> **Note:** `display_name` is updated through the CANCOM Marketplace subscription API.

### Read-Only Attributes

- `subscription_id` (string) - The Azure subscription ID returned in `externalAccountId`.
- `marketplace_subscription_id` (string) - The CANCOM Marketplace subscription ID returned in `id` and used for Marketplace API operations.
- `payment_plan_id` (int) - The fixed payment plan ID (`172495`) used for Azure subscriptions.


## Import

Import a subscription using its CANCOM Marketplace subscription UUID (`data.id`), not its Azure subscription ID.

```shell
terraform import cancom-marketplace_az_subscription.example 00000000-0000-0000-0000-000000000000
```
