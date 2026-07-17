## Context

Captured token counts can be multiplied by published per-token rates, but the
result cannot know a user's plan, included credits, discounts, provider billing
rules, or final invoice. The current catalog already carries a source URL in
JSON, but the Go catalog type discards it and no API response describes the
meaning of the total.

## Goals / Non-Goals

**Goals:**

- Make a cost total self-describing in CLI, API, and dashboard contexts.
- Preserve a stable, machine-readable way for scripts to recognize the estimate
  rather than treating it as billing truth.
- Surface the catalog's published rate source without adding a network lookup.

**Non-Goals:**

- Fetch account-level billing or consume GitHub billing APIs.
- Convert estimates into AI credits, Premium Requests, or invoice amounts.
- Change model pricing, fallback selection, or request persistence.

## Decision

Add an `estimate` object to the existing cost total:

```json
{
  "currency": "USD",
  "rate_source": "https://…",
  "basis": "published_token_rates",
  "billing_scope": "not_invoice_reconciliation"
}
```

`currency` and `basis` make the unit and calculation clear. `rate_source` is
omitted only when a custom/test catalog does not declare one. `billing_scope` is
intentionally a stable machine value rather than prose. Human-facing output uses
the phrase "published token-rate estimate" and says that it is not invoice
reconciliation.

The dashboard keeps the compact dollar metric but names it
`est. token-rate cost`; the footer supplies the semantic boundary. This avoids
making the main screen denser while removing the misleading AI-credit language.
