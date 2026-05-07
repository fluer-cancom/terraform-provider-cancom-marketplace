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
  user_uuid    = "12345678-1234-1234-1234-123456789012"
  # payment_plan_id = 123
  display_name = "My new CANCOM Azure Subscription"
}
```

## Attributes

### Required

- `user_uuid` (string) - The marketplace user UUID that receives owner permissions after subscription creation.

### Optional

- `payment_plan_id` (int) - The payment plan ID for the Azure subscription.
- `display_name` (string) - The display name of the subscription.
- `azure_owner_object_id` (string) - The Azure AD object ID of the subscription owner.

> **Note:** If using `display_name`, the subscription will be renamed after creation by using the Azure API with the context of `az login` or by using the Service Principal configuration of the provider. Ensure that the principal used for `az login` or the Service Principal configuration of the provider has the necessary permissions to rename the subscription (either on management group level, or by assuring that the principal associated with `azure_owner_object_id` is used with the Azure CLI context).

### Read-Only Attributes

- `subscription_id` (string) - The subscription ID of the Azure subscription.


## Import

Import is not supported, as existing subscription can be managed by using AzureRM resource `azurerm_subscription`.