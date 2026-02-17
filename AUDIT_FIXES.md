# Security & Reliability Audit — Fix List

Audit performed 2026-02-16 against commit `905974e`.

## P0 — Critical Bugs

- [x] **Product `Read` doesn't detect archived state** (`resource_product.go`)
  If a product is archived externally (UI/API), `Read` doesn't check `IsArchived` and won't
  remove the resource from state. Terraform plans show no drift, but the product is archived.
  Compare with the meter resource which correctly handles this.

- [x] **`prevent_trial_abuse` never read back from API** (`resource_organization_sdk.go`)
  `mapSupplementalSubscriptionSettings` is a no-op that preserves the planned value.
  `getOrgSupplemental` exists but is never called during `Read`. External changes to
  `prevent_trial_abuse` are invisible to Terraform.

- [x] **Response body double-close in `getOrgSupplemental`** (`resource_organization_sdk.go`)
  On 2xx, the body is closed by `defer resp.Body.Close()` inside the closure *and* by
  `doWithRetry` at line 311. Fragile ownership model — restructure so exactly one caller
  manages the body lifecycle.

## P1 — Security Hardening

- [x] **Add HTTPS validation for webhook URLs** (`resource_webhook_endpoint.go`)
  No scheme validation on the `url` attribute. Users could configure `http://` endpoints,
  sending payment webhook payloads over plaintext.

- [x] **Replace `http.DefaultClient` with configured client** (`resource_organization_sdk.go`)
  `http.DefaultClient` has no timeout and shares global transport. A hanging connection
  blocks Terraform indefinitely. Use a dedicated `*http.Client` with explicit timeout and
  TLS config.

- [x] **Save product ID to state before benefits update** (`resource_product.go`)
  Create flow: create product → update benefits. If benefits update fails, the product exists
  but isn't in state. Re-apply tries to create again → possible duplicate.

- [x] **Use `url.PathEscape` for org ID in URL construction** (`resource_organization_sdk.go`)
  Org ID is interpolated directly into URL paths without escaping. Low risk (ID comes from
  API), but trivial to fix.

## P2 — Reliability

- [ ] **Use decimal comparison instead of float for unit amounts** (`resource_product_sdk.go`)
  `numericStringsEqual` compares via `float64 ==`. IEEE 754 can't represent all decimals
  exactly. Works in practice for current use cases but architecturally fragile for a payment
  provider.

- [ ] **Align retry strategies** (`provider.go` vs `resource_organization_sdk.go`)
  SDK retry: up to ~120s. Supplemental HTTP retry: up to ~3.5s (5 attempts). Under load,
  SDK calls succeed but supplemental PATCH fails.

- [x] **Handle `io.ReadAll` error in `doWithRetry`** (`resource_organization_sdk.go:315`)
  Error is silently dropped, resulting in empty error messages on body-read failure.
  (Fixed as part of P0 `doWithRetry` rework.)

- [x] **Add missing test coverage** (partial)
  - [x] Benefit data source acceptance tests (custom + license_keys types)
  - [x] Webhook HTTPS validation unit test
  - [x] Product SDK unit tests (numericStringsEqual, normalizeDecimalString, preserveUnitAmountFormatting)
  - [ ] Error-path tests (API 500, partial failures) — requires mock server
  - [ ] State drift detection tests — requires mock server

## P3 — Low Priority

- [ ] **Consider requiring explicit `server` selection** (`provider.go`)
  Defaulting to sandbox is safe for dev but a misconfigured production workflow silently
  uses sandbox. Consider requiring explicit selection or emitting a warning.

- [ ] **Validate org ID on import matches discovered org** (`resource_organization.go`)
  `ImportStatePassthroughID` accepts any ID. Could silently adopt wrong org if token has
  broader access in the future.

- [ ] **Inconsistent polling timeout** (`helpers.go`)
  `pollMaxAttempts=10` × `pollInterval=500ms` = ~5s max wait. May be insufficient for
  production environments with higher latency.
