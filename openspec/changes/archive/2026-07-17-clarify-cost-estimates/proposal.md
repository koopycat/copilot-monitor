## Why

The dashboard and CLI currently use language such as "AI-credit list price" and
"equivalent provider list-price" even though the product applies an embedded
per-token USD rate catalog to observed token usage. That can be mistaken for
GitHub billing, included-credit consumption, or invoice reconciliation.

## What Changes

- Add explicit estimate metadata to cost calculation and `/api/cost` JSON:
  currency, rate source, calculation basis, and billing scope.
- Rename CLI and dashboard labels to "published token-rate estimate" and make
  the non-invoice boundary visible wherever the total is presented.
- Retain row-level fallback and not-billed signals, with no changes to captured
  request data or pricing math.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `reporting`: Cost reports and API output identify the estimate's provenance
  and limits.
- `dashboard`: Cost labels distinguish a local estimate from a provider bill.

## Impact

- Adds backward-compatible fields to CLI/API JSON.
- Updates the embedded catalog model to retain its existing source URL.
- Changes only presentation and metadata, not cost computation or persistence.
