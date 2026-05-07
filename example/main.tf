resource "cancom-marketplace_az_subscription" "example" {
  user_uuid    = "9b3ba21d-4bd8-4ee5-a3eb-826b1677b404"
  display_name = "CC Test Subscription"
}

output "subscription_id" {
  value = cancom-marketplace_az_subscription.example.subscription_id
}
