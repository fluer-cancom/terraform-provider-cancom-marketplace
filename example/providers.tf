terraform {
  required_version = ">= 1.0.0"
}

provider "cancommarketplace" {
  endpoint     = "https://api.cancommarketplace.com"
  api_username = "test_user"
  api_password = "test_api_password"
}
