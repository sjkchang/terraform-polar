data "polar_benefit" "example" {
  id = "00000000-0000-0000-0000-000000000000"
}

output "benefit_type" {
  value = data.polar_benefit.example.type
}

output "benefit_description" {
  value = data.polar_benefit.example.description
}
