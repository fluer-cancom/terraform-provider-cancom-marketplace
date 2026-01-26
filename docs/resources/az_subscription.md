---
page_title: "Azure Subscription – by CANCOM Marketplace"
subcategory: ""
description: |-
  Manages an Azure Subscription within the Cancom Marketplace.
---

# Azure Subscription – by CANCOM Marketplace

Manages an Azure Subscription within the Cancom Marketplace.

## Example Usage

```terraform
resource "cancom-marketplace_az_subscription" "example" {
  order_number        = "123456789"
  azure_owner_object_id = "12345678-1234-1234-1234-123456789012"
  azure_discount = 0
}
```

## Attributes

### Required

azure_owner_object_id (string) - The object ID of the principal, which recieves owner permissions after subscription creation.

### Optional

order_number (string) - The PO number of the subscription.
azure_discount (int) - The marketplace discount ID for the Azure Plan.

## Read-Only Attributes

subscription_id (string) - The subscription ID of the Azure subscription.




