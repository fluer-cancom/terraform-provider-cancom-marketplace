terraform {
  required_version = ">= 1.0.0"
}

provider "cancommarketplace" {
  endpoint     = "https://cc-marketplace-ip.azure-api.net"
  api_username = "test_user"
  api_password = "test_api_password"
  country      = "AT"
}
