# Canonicalization spec

The harness-engineer implements this exactly; the determinism-auditor owns the
rules it references. Canonicalization runs on BOTH the source capture and the
target capture before any byte comparison. Its job: erase nondeterministic
*values* while preserving everything that carries meaning (cardinality, order,
references, format).

## Order of operations (per capture)

A capture = `{ stdout: bytes, stderr: bytes, exit: int, files: { path: bytes } }`.

For each stream and each file body, in this order:

1. **Sandbox path rewrite.** Replace the absolute temp sandbox path with
   `sandboxPathVar` (e.g. `<ROOT>`). Do this first so later rules don't match
   inside paths.

2. **Format assertions (before masking).** For every `maskRule` with
   `assertFormat: true`, verify each raw match conforms to the expected shape
   (the full regex, expected length/charset). A failed assertion is a **test
   failure**, not a mask — it is exactly how a wrong ID length or malformed
   timestamp is caught despite masking.

3. **Ordinal masking.** For each `maskRule`, scan left-to-right across the
   capture in a stable stream order (define one and keep it fixed: e.g. stdout,
   then stderr, then files sorted by path). Assign the Nth *distinct* match the
   token `<NAME:n>` in first-seen order. The SAME distinct value always maps to
   the SAME token within a capture, so a generated ID echoed in stdout and then
   written to a file collapses to one token in both places — preserving the
   reference.

4. **Ordering assertions.** For rules with `ordered: true` (e.g. timestamps),
   after assigning ordinals, assert the documented ordering relationship holds
   among the distinct values (e.g. `<TS:1> <= <TS:2> <= ...` in capture order).
   A violation is a test failure.

## Cross-capture consistency

Ordinal assignment is **per capture, independently**. Because both source and
target are driven with identical input under identical env pins, the Nth distinct
ID in the source capture corresponds to the Nth distinct ID in the target
capture. If they don't line up after canonicalization, that IS the divergence —
do not "fix" it by aligning tokens across captures.

## What canonicalization must NOT do

- Must not sort or reorder output (ordering is part of the contract).
- Must not collapse whitespace, normalize unicode, or trim newlines.
- Must not be applied differently to source vs target.
- Must not mask anything not covered by an explicit `maskRule`.

## Output

Canonicalization yields a deterministic byte string per stream/file. The
comparator diffs source-canonical vs target-canonical and reports the first
differing offset with a windowed hex + visible diff.
