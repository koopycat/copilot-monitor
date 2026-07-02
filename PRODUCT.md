# Product

## Register

product

## Users

Developers using GitHub Copilot in VSCode who want visibility into their own model usage, token consumption, and estimated AI-credit cost.
Self-service, single-user, local tool.

## Product Purpose

Monitor GitHub Copilot API calls transparently.
Capture per-model token counts, latency, and estimated cost.
Provide a CLI and a local dashboard for inspecting usage patterns.
The product is a utility, not a platform.
It should get out of the way and show the data clearly.

## Brand Personality

Utilitarian, precise, understated.
Three words: **data-first, developer, quiet.**

## Anti-references

- Over-designed admin panels with heavy card nesting and decorative gradients
- Consumer SaaS dashboards that prioritize "delight" over clarity
- Flashy color schemes that distract from data
- White/cream backgrounds (anti-references from the skill itself; we chose dark deliberately)

## Design Principles

1. **Data first.** Every pixel serves readability of token counts, costs, and timelines. Remove anything that competes.
2. **Developer aesthetic.** Dark terminal-inspired palette. Monospace-compatible. No marketing language.
3. **Single screen, no chrome.** Everything visible without scrolling when possible. No sidebars, no tabs, no modals.
4. **Honest numbers.** Cost estimates are clearly labeled as estimates. Fallbacks are tagged. Zero-billed items are marked.
5. **Fast feedback.** Auto-refresh every 30 seconds. First paint in under 100ms (embedded, no CDN).

## Accessibility & Inclusion

- WCAG AA minimum for all text (≥4.5:1 contrast against background)
- `prefers-reduced-motion` respected; chart transitions must be instant
- All data available as JSON API for screen readers and custom tooling
