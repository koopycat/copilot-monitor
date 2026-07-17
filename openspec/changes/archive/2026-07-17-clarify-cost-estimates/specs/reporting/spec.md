## MODIFIED Requirements

### Requirement: Provider-specific cost fallback

Cost output SHALL be labeled as a published token-rate estimate and include
machine-readable metadata describing its currency, rate source when available,
calculation basis, and non-invoice billing scope. The estimate SHALL use a
two-tier fallback: when a model is not in the catalog, provider-specific
fallback rates are tried first, then a generic fallback.

#### Scenario: Exact model pricing

- **WHEN** the model is in the pricing catalog
- **THEN** the model's exact pricing rates are used
- **AND** the cost total identifies the result as a published token-rate
  estimate rather than invoice reconciliation

#### Scenario: Provider fallback

- **WHEN** the model is not in the catalog but a provider-specific fallback rate
  exists
- **THEN** the provider fallback rate is used and the row is marked as fallback

#### Scenario: Generic fallback

- **WHEN** the model is not in the catalog and no provider fallback exists
- **THEN** the global fallback rate is used and the row is marked as fallback

#### Scenario: Machine-readable estimate semantics

- **WHEN** the cost command or `/api/cost` returns JSON
- **THEN** the total includes `estimate.currency`, `estimate.basis`, and
  `estimate.billing_scope`
- **AND** `estimate.rate_source` is included when the catalog declares one
