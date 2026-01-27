# Terraform Provider for Cancom Marketplace

The Cancom Marketplace Terraform Provider allows you to manage Cancom Marketplace resources, specifically Azure subscriptions, via Terraform.
For becoming a CSP customer, please contact us via https://marketplace.cancom.at (Austria) or https://marketplace.cancom.de (Germany).

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.0.0
- [Go](https://golang.org/doc/install) >= 1.20 (to build the provider plugin)

## Configuration

To use the provider, you need to configure it with credentials. These are typically received via a OneTime Link from Cancom.

```hcl
provider "fluer-cancom/cancom-marketplace" {
  api_username = "your-api-username"
  api_password = "your-api-password"
  country      = "DE" # or AT
  # endpoint   = "https://cc-marketplace-ip.azure-api.net" # Optional, defaults to production

  azure_client_id     = "your-azure-client-id" # Optional if using display_name / terraform support cancellation – alternatively use `az login`
  azure_client_secret = "your-azure-client-secret" # Optional if using display_name / terraform support cancellation – alternatively use `az login`
  azure_tenant_id     = "your-azure-tenant-id" # Optional if using display_name / terraform support cancellation – alternatively use `az login`
}
```

### Provider Arguments

*   `api_username` (String, Required) The API username for the Cancom Marketplace.
*   `api_password` (String, Required) The API password for the Cancom Marketplace.
*   `country` (String, Required) The country of the customer (e.g., "DE", "AT").
*   `endpoint` (String, Optional) The API endpoint. Defaults to `https://cc-marketplace-ip.azure-api.net`.
*   `azure_client_id` (String, Optional) The Azure client ID for the customers tenant. – alternatively use `az login`
*   `azure_client_secret` (String, Optional) The Azure client secret for the customers tenant. – alternatively use `az login`
*   `azure_tenant_id` (String, Optional) The Azure tenant ID for the customers tenant. – alternatively use `az login`

## Resources

### az_subscription

This resource allows you to create and manage Azure Subscriptions.

#### Example Usage

```hcl
resource "az_subscription" "example" {
  order_number        = "123456789"
  azure_owner_object_id = "00000000-0000-0000-0000-000000000000"
  # azureDiscount    = 123 # Optional

  display_name = "My new CANCOM Azure Subscription"
}
```

#### Argument Reference

*   `order_number` (String, Optional) The PO number of the subscription.
*   `azure_owner_object_id` (String, Required) The object ID of the principal (User, Service Principal, etc.) who will receive Owner permissions on the created subscription.
*   `azure_discount` (Int, Optional) The marketplace discount ID for the Azure Plan.
*   `display_name` (String, Optional) The display name of the subscription. – if set, usage of `az login` command or `azure_client_id`, `azure_client_secret` and `azure_tenant_id` is required.

#### Attribute Reference

*   `subscription_id` (String) The ID of the created Azure subscription.
*   `request_id` (String) The request ID of the created Azure subscription.

## Development

If you wish to work on the provider, you'll first need Go installed on your machine (version 1.20+ is recommended).

### Building

To compile the provider, run `go build`. This will build the provider binary in the root of the project:

```bash
go build
```
