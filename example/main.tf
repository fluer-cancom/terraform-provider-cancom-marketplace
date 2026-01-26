resource "cancom-marketplace_az_subscription" "example" {
  order_number          = "DEMOORDER"
  azure_owner_object_id = "09c2203b-2ec2-4b55-a062-8378b32e65dd"
}

output "subscription_id" {
  value = cancom-marketplace_az_subscription.example.subscription_id
}
