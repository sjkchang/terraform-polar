data "polar_product" "example" {
  id = "00000000-0000-0000-0000-000000000000"
}

output "product_name" {
  value = data.polar_product.example.name
}
