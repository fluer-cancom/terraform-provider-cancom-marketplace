variable "api_client_id" {
  type = string
}

variable "api_client_secret" {
  type      = string
  sensitive = true
}

variable "api_scope" {
  type    = string
  default = "AT-PROD"
}
