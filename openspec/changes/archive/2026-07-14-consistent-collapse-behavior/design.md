## Decisions

**Decision: Remove custom label, keep native arrow**

All sections already use `<details>`. The native disclosure arrow is universally
recognized and accessible. The custom "collapsed"/"expanded" text label on
Models/Sessions was redundant -- it duplicates what the arrow already
communicates. Removing it makes all sections consistent with Anomalies and
Routes, which never had the label.

**Risk:** None. Pure removal of redundant UI elements.
