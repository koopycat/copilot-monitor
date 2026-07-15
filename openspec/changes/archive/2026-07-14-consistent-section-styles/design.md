## Decisions

**Decision: Share anomaly-feed card style via CSS properties only**

No shared class needed -- `.table-section` and `.anomaly-feed` use the same CSS
properties independently. This avoids refactoring the markup and keeps sections
independently stylable in the future.

**Risk:** None. Additive CSS properties only.
