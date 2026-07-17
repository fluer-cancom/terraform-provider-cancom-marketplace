---
page_title: "Azure Subscription"
subcategory: ""
description: |-
  Reads Azure Subscription information from the CANCOM Marketplace.
---

# Azure Subscription Data Source - by CANCOM Marketplace

Reads a CANCOM Marketplace subscription and exposes the related Azure subscription ID and metadata.

## Example Usage

```terraform
data "cancom-marketplace_az_subscription" "example" {
  marketplace_subscription_id = "00000000-0000-0000-0000-000000000000"
}
```

## Attributes

### Required

- `marketplace_subscription_id` (string) - The CANCOM Marketplace subscription ID used to query the Marketplace API.

### Read-Only Attributes

- `subscription_id` (string) - The Azure subscription ID returned as `externalAccountId` by the Marketplace API.
- `display_name` (string) - The display name of the subscription.
- `payment_plan_id` (int) - The payment plan ID associated with the subscription order.
- `owner_id` (string) - The Marketplace user ID that owns the subscription.
