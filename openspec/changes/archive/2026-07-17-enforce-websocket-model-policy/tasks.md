## 1. Frame relay and policy enforcement

- [x] 1.1 Represent and preserve WebSocket frame masking, fragmentation, FIN,
      opcode, and reserved bits.
- [x] 1.2 Inspect complete client text messages and block explicitly
      policy-disallowed models before forwarding.
- [x] 1.3 Persist blocked WebSocket attempts and return a policy-violation close
      frame.

## 2. Tests and documentation

- [x] 2.1 Add tests for allowed, blocked, fragmented, and masked client frames.
- [x] 2.2 Synchronize proxy and policy specifications and replace the security
      limitation note with the exact fail-open boundary.
- [x] 2.3 Update product and setup documentation to describe the policy scope.

## 3. Verification

- [x] 3.1 Run targeted proxy/policy tests and the repository fast test suite.
