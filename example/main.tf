resource "cancom-marketplace_az_subscription" "example" {
  user_uuid    = "4cca3d47-0776-4f3f-8f75-5ed605adf7a3"
  display_name = "CC Test Subscription"
}

output "subscription_id" {
  value = cancom-marketplace_az_subscription.example.subscription_id
}
