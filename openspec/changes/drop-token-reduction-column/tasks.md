## 1. Remove column from Models table

- [x] 1.1 Remove `<th class="col-optional">Token Reduction</th>` from the thead
- [x] 1.2 Remove the `<td class="num col-optional">` block containing the `compression_removed_tokens` display from the tbody

## 2. Verification

- [x] 2.1 Run `just dashboard-check` — 0 errors, 0 warnings
- [x] 2.2 Run `just build` — passes
- [x] 2.3 Smoke-test: page loads, Models table renders with one fewer column, no broken layout
