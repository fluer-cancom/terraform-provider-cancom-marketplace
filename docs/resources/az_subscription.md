---
page_title: "Azure Subscription – by CANCOM Marketplace"
subcategory: ""
description: |-
  Manages an Azure Subscription within the Cancom Marketplace.
---

# Azure Subscription – by CANCOM Marketplace

Manages an Azure Subscription within the Cancom Marketplace.

> **Note:**  Destroying the resource will cancel the subscription. You can delete the subscription after a period of 7 days manually from the Azure Portal. After 30 days, the subscription will be deleted automatically.

## Example Usage

```terraform
resource "cancom-marketplace_az_subscription" "example" {
  order_number        = "123456789"
  azure_owner_object_id = "12345678-1234-1234-1234-123456789012"
  azure_discount = 0

  display_name = "My new CANCOM Azure Subscription"
}
```

## Attributes

### Required

- `azure_owner_object_id` (string) - The object ID of the principal, which recieves owner permissions after subscription creation.

### Optional

- `order_number` (string) - The PO number of the subscription. (Change forces redeployment)
- `azure_discount` (int) - The marketplace discount ID for the Azure Plan. (Change forces redeployment)
- `display_name` (string) - The display name of the subscription.

> **Note:** If using `display_name`, the subscription will be renamed after creation by using the Azure API with the context of `az login` or by using the Service Principal configuration of the provider. Ensure that the principal used for `az login` or the Service Principal configuration of the provider has the necessary permissions to rename the subscription (either on management group level, or by assuring that the principal associated with `azure_owner_object_id` is used with the Azure CLI context).

### Read-Only Attributes

- `subscription_id` (string) - The subscription ID of the Azure subscription.
- `request_id` (string) - The request ID of the Azure subscription.


## Import

Import is not supported, as existing subscription can be managed by using AzureRM resource `azurerm_subscription`.