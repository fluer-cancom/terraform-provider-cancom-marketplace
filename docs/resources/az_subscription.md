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
  orderNumber        = "123456789"
  azureOwnerObjectId = "12345678-1234-1234-1234-123456789012"
}
```

## Attributes

### Required

azureOwnerObjectId (string) - The object ID of the principal, which recieves owner permissions after subscription creation.

### Optional

orderNumber (string) - The PO number of the subscription.
azureDiscount (int) - The marketplace discount ID for the Azure Plan.

## Read-Only Attributes

subscriptionId (string) - The subscription ID of the Azure subscription.




