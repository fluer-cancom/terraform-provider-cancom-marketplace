---
page_title: "<no value> Resource - <no value>"
subcategory: ""
description: |-
  Manages an Azure Subscription within the Cancom Marketplace.
---

# <no value> (Resource)

Manages an Azure Subscription within the Cancom Marketplace.

## Example Usage

```terraform
resource "cancommarketplace_az_subscription" "example" {
  orderNumber        = "123456789"
  azureOwnerObjectId = "12345678-1234-1234-1234-123456789012"
  country            = "DE"
}
```


