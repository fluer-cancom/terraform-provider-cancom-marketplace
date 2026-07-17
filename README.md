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
      source = "fluer-cancom/cancom-marketplace"
    }
  }
}

provider "cancom-marketplace" {
  api_client_id          = "your-api-client-id"
  api_client_secret      = "your-api-client-secret"
  marketplace_user_email = "user@example.com"
  api_scope              = "AT-PROD" # Optional, defaults to AT-PROD
  # endpoint             = "https://marketplace-apigateway.cancom.de" # Optional, defaults to production

  azure_client_id     = "your-azure-client-id" # Optional; required for Azure-backed operations unless `az login` supplies credentials
  azure_client_secret = "your-azure-client-secret" # Optional; required for Azure-backed operations unless `az login` supplies credentials
  azure_tenant_id     = "your-azure-tenant-id" # Optional; required for Azure-backed operations unless `az login` supplies credentials
}
```

The same values can also be supplied through environment variables:

```shell
export CANCOM_MARKETPLACE_API_CLIENT_ID="your-api-client-id"
export CANCOM_MARKETPLACE_API_CLIENT_SECRET="your-api-client-secret"
export CANCOM_MARKETPLACE_USER_EMAIL="user@example.com"
export CANCOM_MARKETPLACE_API_SCOPE="AT-PROD"
export CANCOM_MARKETPLACE_ENDPOINT="https://marketplace-apigateway.cancom.de"

export CANCOM_MARKETPLACE_AZURE_CLIENT_ID="your-azure-client-id"
export CANCOM_MARKETPLACE_AZURE_CLIENT_SECRET="your-azure-client-secret"
export CANCOM_MARKETPLACE_AZURE_TENANT_ID="your-azure-tenant-id"
```

When using a Terraform CLI `dev_overrides` entry for local provider development, do not run `terraform init` in the example directory. Use `terraform plan` or `terraform apply` directly so Terraform loads the provider binary from the override path instead of querying the public registry.

### Provider Arguments

*   `api_client_id` (String, Required) The API client ID for the Cancom Marketplace. Can also be set with `CANCOM_MARKETPLACE_API_CLIENT_ID`.
*   `api_client_secret` (String, Required) The API client secret for the Cancom Marketplace. Can also be set with `CANCOM_MARKETPLACE_API_CLIENT_SECRET`.
*   `marketplace_user_email` (String, Required) The email address of the CANCOM Marketplace user for which subscriptions are created. Can also be set with `CANCOM_MARKETPLACE_USER_EMAIL`.
*   `api_scope` (String, Optional) The API scope for the Cancom Marketplace. Can also be set with `CANCOM_MARKETPLACE_API_SCOPE`. Defaults to `AT-PROD`.
*   `endpoint` (String, Optional) The API endpoint. Can also be set with `CANCOM_MARKETPLACE_ENDPOINT`. Defaults to `https://marketplace-apigateway.cancom.de`.
*   `azure_client_id` (String, Optional) The Azure client ID for the customers tenant. Can also be set with `CANCOM_MARKETPLACE_AZURE_CLIENT_ID`; alternatively use `az login` or Azure SDK default credentials.
*   `azure_client_secret` (String, Optional) The Azure client secret for the customers tenant. Can also be set with `CANCOM_MARKETPLACE_AZURE_CLIENT_SECRET`; alternatively use `az login` or Azure SDK default credentials.
*   `azure_tenant_id` (String, Optional) The Azure tenant ID for the customers tenant. Can also be set with `CANCOM_MARKETPLACE_AZURE_TENANT_ID`; alternatively use `az login` or Azure SDK default credentials.

## Resources

### cancom-marketplace_az_subscription

This resource allows you to create and manage Azure Subscriptions.

Before creating a subscription that uses Azure-backed properties, the provider verifies Azure authentication and reads the Default Management Group hierarchy settings. This prevents creating a Marketplace subscription when the Azure follow-up operation would fail. After creation, the provider polls the Marketplace subscription every five seconds until `data.order.status` is `ACTIVE`. Azure-backed follow-up operations, such as renaming the Azure subscription or assigning the Owner role, only run after that point. The default create timeout is 30 minutes and can be overridden with a Terraform `timeouts` block.

#### Example Usage

```hcl
resource "cancom-marketplace_az_subscription" "example" {
  display_name          = "My new CANCOM Azure Subscription"
  azure_owner_object_id = "11111111-1111-1111-1111-111111111111"
}
```

#### Argument Reference

*   `azure_owner_object_id` (String, Optional) The Azure principal object ID that receives the `Owner` role on the created Azure subscription. Requires Azure authentication and permissions inherited from the Default Management Group.
*   `display_name` (String, Optional) The Azure subscription display name. If set, usage of `az login` command or `azure_client_id`, `azure_client_secret` and `azure_tenant_id` is required.

#### Attribute Reference

*   `subscription_id` (String) The Azure subscription ID returned in `externalAccountId`.
*   `marketplace_subscription_id` (String) The CANCOM Marketplace subscription ID returned in `id` and used for Marketplace API operations.
*   `payment_plan_id` (Int) The fixed payment plan ID (`172495`) used for Azure subscriptions.

## Development

If you wish to work on the provider, you'll first need Go installed on your machine (version 1.20+ is recommended).

### Building

To compile the provider for Terraform CLI `dev_overrides`, build the binary with the provider type in the filename so Terraform can discover it:

```bash
go build -o terraform-provider-cancom-marketplace
```

If your Terraform CLI config contains a `dev_overrides` entry, point it at the directory containing that binary and then run `terraform plan` or `terraform apply` directly without `terraform init`.
