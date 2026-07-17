# Product

## Register

utility

## Positioning

**Copilot Monitor is the local flight recorder for AI coding traffic.** It lets
one developer see the model, token, cache, latency, status, and activity data
behind requests their tools already make.

It complements platform gateways and observability services; it is not a
multi-provider control plane, shared SaaS dashboard, or billing system. A single
running proxy forwards to one explicitly chosen upstream host and keeps its
monitoring data in local SQLite.

## Users

Individual developers who want a private, request-level record of the AI coding
tools they use. GitHub Copilot is the primary integration; pi/Kilo and other
clients that support a custom base URL are compatible where their request shape
can be forwarded unchanged. Self-service, single-user, loopback-local tool.

## Product Purpose

A local reverse proxy that sits between a configured client and one upstream
host. It captures per-request metadata, token counts, latency, HTTP status, and
estimated model-rate cost without storing prompts, completions, source code, or
auth material in the normal SQLite capture. GitHub Copilot traffic, including
its usage-bearing WebSocket events, is a first-class use case.

All monitoring data stays in a local SQLite database. There is no monitoring
cloud, account, analytics, or phone-home. The proxy still makes the API request
to the upstream host the user explicitly configures.

You get a CLI for quick inspection and a local dashboard for browsing usage
patterns, session history, and cost breakdowns. A built-in model policy applies
to model-bearing HTTP requests and Copilot WebSocket text messages before they
are forwarded.

This is a utility, not a platform. It should get out of the way and show the
data clearly.

## Brand Personality

Utilitarian, precise, understated. Three words: **data-first, developer,
quiet.**

## Anti-references

- Over-designed admin panels with heavy card nesting and decorative gradients
- Consumer SaaS dashboards that prioritize "delight" over clarity
- Flashy color schemes that distract from data
- White/cream backgrounds (anti-references from the skill itself; we chose dark
  deliberately)

## Design Principles

1. **Data first.** Every pixel serves readability of token counts, costs, and
   timelines. Remove anything that competes.
2. **Developer aesthetic.** Dark terminal-inspired palette.
   Monospace-compatible. No marketing language.
3. **Single screen, no chrome.** Everything visible without scrolling when
   possible. No sidebars, no tabs, no modals.
4. **Honest numbers.** Label cost estimates clearly. Tag fallback pricing. Mark
   zero-billed items as such.
5. **Fast feedback.** Auto-refresh every 30 seconds. Keep first paint fast and
   avoid a frontend build step.

## Accessibility & Inclusion

- WCAG AA minimum for all text (≥4.5:1 contrast against background)
- `prefers-reduced-motion` respected; chart transitions must be instant
- All data available as JSON API for screen readers and custom tooling
