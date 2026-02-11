resource "polar_webhook_endpoint" "order_notifications" {
  url    = "https://example.com/webhooks/polar"
  format = "raw"
  events = [
    "order.created",
    "order.paid",
    "subscription.created",
    "subscription.canceled",
  ]
}

# Discord-formatted webhook
resource "polar_webhook_endpoint" "discord" {
  url    = "https://discord.com/api/webhooks/1234567890/abcdef"
  format = "discord"
  events = ["order.created"]
}
