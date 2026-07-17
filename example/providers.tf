terraform {
  required_providers {
    cancom-marketplace = {
      source  = "fluer-cancom-dev/cancom-marketplace"
      version = "1.0.0"
    }
  }
}

provider "cancom-marketplace" {
  api_client_id          = var.api_client_id
  api_client_secret      = var.api_client_secret
  api_scope              = var.api_scope
  marketplace_user_email = "bernhard.fluer@cancom.com"
  endpoint               = "https://marketplace-apigateway.cancom.de"
}
