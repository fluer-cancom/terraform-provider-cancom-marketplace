terraform {
  required_providers {
    cancom-marketplace = {
      source  = "fluer-cancom-dev/cancom-marketplace"
      version = "0.1.7"
    }
  }
}

provider "cancom-marketplace" {
  endpoint     = "https://cc-marketplace-ip.azure-api.net"
  api_username = var.CC_MP_USERNAME
  api_password = var.CC_MP_PASSWORD
  country      = "AT"
}
