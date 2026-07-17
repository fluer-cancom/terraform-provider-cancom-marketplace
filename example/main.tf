resource "cancom-marketplace_az_subscription" "example" {
  display_name          = "CC Test Subscription"
  azure_owner_object_id = "00000000-0000-0000-0000-000000000000"
}

output "subscription_id" {
  value = cancom-marketplace_az_subscription.example.subscription_id
}
