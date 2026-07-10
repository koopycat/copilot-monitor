# Product

## Register

utility

## Users

Developers who use LLM APIs through any tool — VSCode with GitHub Copilot, pi agent, Claude Code,
aider, curl, or anything that speaks OpenAI-compatible or Anthropic-compatible HTTP.
Self-service, single-user, local tool.

## Product Purpose

A local reverse proxy that sits between your tools and LLM APIs.
It captures per-request metadata, token counts, latency, and estimated cost
without ever storing prompts, completions, source code, or auth material.
First-class built-in support for GitHub Copilot; configurable routes for any
OpenAI-compatible or Anthropic-compatible API.

All data stays in a local SQLite database.
No cloud, no telemetry.

You get a CLI for quick inspection and a local dashboard for browsing usage
patterns, session history, and cost breakdowns.
A built-in model policy lets you block or allow specific models across all your tools.

This is a utility, not a platform.
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
4. **Honest numbers.** Label cost estimates clearly. Tag fallback pricing. Mark zero-billed items as such.
5. **Fast feedback.** Auto-refresh every 30 seconds. Keep first paint fast and avoid a frontend build step.

## Accessibility & Inclusion

- WCAG AA minimum for all text (≥4.5:1 contrast against background)
- `prefers-reduced-motion` respected; chart transitions must be instant
- All data available as JSON API for screen readers and custom tooling
