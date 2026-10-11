## 1. Usage normalization

- [x] 1.1 Add cache read tokens to prompt_tokens in usage parsing when the
      prompt count comes from `input_tokens`; keep reported `prompt_tokens` as
      reported.
- [x] 1.2 Derive total tokens as prompt + completion + cache write for every
      usage shape.
- [x] 1.3 Add tests for Anthropic, fully cached Anthropic, Responses API, and
      gateway usage shapes, and a cost check for the #68 example.

## 2. Stream usage merge

- [x] 2.1 Merge usage objects across stream events field by field (larger value
      wins, missing or `null` keeps the earlier value) and parse the merged
      object once.
- [x] 2.2 Add stream tests for output-only, cumulative, `null`, and zero
      `message_delta` usage, merged detail fields, a Responses API stream, and
      an OpenAI final chunk with a reported total.

## 3. Persistence after client disconnect

- [x] 3.1 Persist the row for a completed response with a non-cancelable
      context.
- [x] 3.2 Add a handler test where the client context is canceled after the
      response is written.

## 4. Specs and verification

- [x] 4.1 Update the capture requirement scenarios.
- [x] 4.2 Reproduce end to end against a fake Anthropic upstream before and
      after the fix.
- [x] 4.3 Run `just all`, `just integration`, and race-enabled proxy tests.
