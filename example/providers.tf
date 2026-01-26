terraform {
  required_version = ">= 1.0.0"
  required_providers {
    cancom-marketplace = {
      source  = "fluer-cancom/cancom-marketplace"
      version = "1.0.2"
    }
  }
}

provider "cancom-marketplace" {
  endpoint     = "https://cc-marketplace-ip.azure-api.net"
  api_username = var.CC_MP_USERNAME
  api_password = var.CC_MP_PASSWORD
  country      = "AT"
}
