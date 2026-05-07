# Terraform Provider for Cancom Marketplace

The Cancom Marketplace Terraform Provider allows you to manage Cancom Marketplace resources, specifically Azure subscriptions, via Terraform.
For becoming a CSP customer, please contact us via https://marketplace.cancom.at (Austria) or https://marketplace.cancom.de (Germany).

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.0.0
- [Go](https://golang.org/doc/install) >= 1.20 (to build the provider plugin)

## Configuration

To use the provider, you need to configure it with credentials. These are typically received via a OneTime Link from Cancom.

```hcl
terraform {
  required_providers {
    cancom-marketplace = {
      source = "fluer-cancom-dev/cancom-marketplace"
    }
  }
}

provider "cancom-marketplace" {
  api_client_id     = "your-api-client-id"
  api_client_secret = "your-api-client-secret"
  api_scope         = "AT-PROD" # Optional, defaults to AT-PROD
  # endpoint        = "https://marketplace-apigateway.cancom.de" # Optional, defaults to production

  azure_client_id     = "your-azure-client-id" # Optional if using display_name / terraform support cancellation – alternatively use `az login`
  azure_client_secret = "your-azure-client-secret" # Optional if using display_name / terraform support cancellation – alternatively use `az login`
  azure_tenant_id     = "your-azure-tenant-id" # Optional if using display_name / terraform support cancellation – alternatively use `az login`
}
```

When using a Terraform CLI `dev_overrides` entry for local provider development, do not run `terraform init` in the example directory. Use `terraform plan` or `terraform apply` directly so Terraform loads the provider binary from the override path instead of querying the public registry.

### Provider Arguments

*   `api_client_id` (String, Required) The API client ID for the Cancom Marketplace.
*   `api_client_secret` (String, Required) The API client secret for the Cancom Marketplace.
*   `api_scope` (String, Optional) The API scope for the Cancom Marketplace. Defaults to `AT-PROD`.
*   `endpoint` (String, Optional) The API endpoint. Defaults to `https://marketplace-apigateway.cancom.de`.
*   `azure_client_id` (String, Optional) The Azure client ID for the customers tenant. – alternatively use `az login`
*   `azure_client_secret` (String, Optional) The Azure client secret for the customers tenant. – alternatively use `az login`
*   `azure_tenant_id` (String, Optional) The Azure tenant ID for the customers tenant. – alternatively use `az login`

## Resources

### cancom-marketplace_az_subscription

This resource allows you to create and manage Azure Subscriptions.

#### Example Usage

```hcl
resource "cancom-marketplace_az_subscription" "example" {
  user_uuid    = "00000000-0000-0000-0000-000000000000"
  # payment_plan_id = 123 # Optional
  display_name = "My new CANCOM Azure Subscription"
}
```

#### Argument Reference

*   `user_uuid` (String, Required) The marketplace user UUID that will own the created subscription.
*   `payment_plan_id` (Int, Optional) The payment plan ID for the Azure subscription.
*   `azure_owner_object_id` (String, Optional) The Azure AD object ID of the subscription owner.
*   `display_name` (String, Optional) The display name of the subscription. – if set, usage of `az login` command or `azure_client_id`, `azure_client_secret` and `azure_tenant_id` is required.

#### Attribute Reference

*   `subscription_id` (String) The ID of the created Azure subscription.

## Development

If you wish to work on the provider, you'll first need Go installed on your machine (version 1.20+ is recommended).

### Building

To compile the provider for Terraform CLI `dev_overrides`, build the binary with the provider type in the filename so Terraform can discover it:

```bash
go build -o terraform-provider-cancom-marketplace
```

If your Terraform CLI config contains a `dev_overrides` entry, point it at the directory containing that binary and then run `terraform plan` or `terraform apply` directly without `terraform init`.
