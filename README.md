# Terraform Provider for Cancom Marketplace

The Cancom Marketplace Terraform Provider allows you to manage Cancom Marketplace resources, specifically Azure subscriptions, via Terraform.

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.0.0
- [Go](https://golang.org/doc/install) >= 1.20 (to build the provider plugin)

## Installation

### Building from Source

1. Clone the repository
2. Build the provider:
   ```bash
   go build -o terraform-provider-cancommarketplace
   ```

### Local Installation

To use the provider locally without publishing to the registry, you can configure Terraform to use a local plugin path or use development overrides.

#### Method 1: Development Overrides (Recommended for dev)

Add the following to your `~/.terraformrc` (Unix) or `%APPDATA%\terraform.rc` (Windows) file:

```hcl
provider_installation {
  dev_overrides {
    "cancom/cancommarketplace" = "/path/to/extracted/provider/directory"
  }
  # For other providers, use direct registry installation
  direct {}
}
```

Then, you can skip `terraform init`. Be warned that `dev_overrides` prevents `terraform init` from caching other providers, so you might need to run `terraform init` before enabling the override if you use other providers.

## Configuration

To use the provider, you need to configure it with credentials. These are typically received via a OneTime Link from Cancom.

```hcl
provider "cancommarketplace" {
  api_username = "your-api-username"
  api_password = "your-api-password"
  country      = "DE" # or other ISO country code
  # endpoint   = "https://cc-marketplace-ip.azure-api.net" # Optional, defaults to production
}
```

### Provider Arguments

*   `api_username` (String, Required) The API username for the Cancom Marketplace.
*   `api_password` (String, Required) The API password for the Cancom Marketplace.
*   `country` (String, Required) The country of the customer (e.g., "DE", "AT").
*   `endpoint` (String, Optional) The API endpoint. Defaults to `https://cc-marketplace-ip.azure-api.net`.

## Resources

### az_subscription

This resource allows you to create and manage Azure Subscriptions.

#### Example Usage

```hcl
resource "az_subscription" "example" {
  orderNumber        = "123456789"
  azureOwnerObjectId = "00000000-0000-0000-0000-000000000000"
  country            = "DE"
  # azureDiscount    = 123 # Optional
}
```

#### Argument Reference

*   `orderNumber` (String, Optional) The PO number of the subscription.
*   `azureOwnerObjectId` (String, Required) The object ID of the principal (User, Service Principal, etc.) who will receive Owner permissions on the created subscription.
*   `country` (String, Required) The country of the customer. Defaults to "DE".
*   `azureDiscount` (Int, Optional) The marketplace discount ID for the Azure Plan.

#### Attribute Reference

*   `subscriptionId` (String) The ID of the created Azure subscription.

## Development

If you wish to work on the provider, you'll first need Go installed on your machine (version 1.20+ is recommended).

### Building

To compile the provider, run `go build`. This will build the provider binary in the root of the project:

```bash
go build
```

### Testing

In order to test the provider, you can simply run `go test ./...`.

```bash
make test
```

(Note: You may need to create a `Makefile` or run `go test ./...` directly if no Makefile exists).
