# Track API call events with a count aggregation
resource "polar_meter" "api_calls" {
  name = "API Calls"

  filter = {
    conjunction = "and"
    clauses = [{
      property = "name"
      operator = "eq"
      value    = "api_call"
    }]
  }

  aggregation = {
    func = "count"
  }
}

# Track usage amount with a sum aggregation
resource "polar_meter" "data_transfer" {
  name = "Data Transfer"

  filter = {
    conjunction = "and"
    clauses = [{
      property = "type"
      operator = "eq"
      value    = "data_transfer"
    }]
  }

  aggregation = {
    func     = "sum"
    property = "bytes"
  }

  metadata = {
    unit = "bytes"
  }
}
