resource "cancom-marketplace_az_subscription" "example" {
  display_name = "CC Test Subscription"
}

output "subscription_id" {
  value = cancom-marketplace_az_subscription.example.subscription_id
}
