terraform {
  required_version = ">= 1.0.0"
  required_providers {
    cancommarketplace = {
      source  = "fluer-cancom/cancom-marketplace"
      version = "1.0.1"
    }
  }
}

provider "cancommarketplace" {
  endpoint     = "https://cc-marketplace-ip.azure-api.net"
  api_username = var.CC_MP_USERNAME
  api_password = var.CC_MP_PASSWORD
  country      = "AT"
}
