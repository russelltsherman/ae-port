# JS → Go byte-parity hazards

A checklist of JavaScript/V8/JSC vs Go semantic differences that produce byte
divergences when porting a JS/TS CLI to Go. ~80% of adversarial findings in the
seeds port were on this list. The determinism-auditor and port-engineers should
consult it UP FRONT — before the sweep — so these are designed-for, not
discovered one fixture at a time.

For each: the JS behavior, the naive-Go behavior, and the fix.

## Strings

- **`toLowerCase`/`toUpperCase` special-casing.** JS does full Unicode case
  mapping (SpecialCasing.txt): `İ` (U+0130) → `i` + U+0307 combining dot;
  trailing `Σ` → final sigma `ς`. Go `strings.ToLower` does simple mapping →
  diverges. Fix: a JS-faithful case-fold (golang.org/x/text or a SpecialCasing
  table for the divergent codepoints). Affects case-insensitive search, label
  normalization (which persists to disk!).
- **String sort order = UTF-16 code units.** JS `Array.prototype.sort()` default
  compares by UTF-16 code units, so an astral char (U+1F600, surrogate pair
  0xD83D…) sorts ABOVE a BMP char (U+F8FF). Go `sort.Strings` compares UTF-8
  bytes → different order. Fix: a UTF-16 code-unit comparator.
- **`localeCompare` ≠ codepoint/byte order, and ignores `LC_ALL`.** Bun's
  `localeCompare` uses a fixed ICU root locale (DUCET) regardless of `LC_ALL=C`.
  Used for id/created/updated sorts. Fix: ICU collation
  (`golang.org/x/text/collate`, `language.Und`) — matches byte-for-byte; do NOT
  assume `LC_ALL=C` gives codepoint order.
- **`String.prototype.trim()` whitespace set ≠ Go `strings.TrimSpace`.** JS trim
  strips U+FEFF (BOM) but NOT U+0085 (NEL); Go TrimSpace strips NEL but not BOM.
  This bites EVERY `.trim()` call site: JSONL line parsing, title normalization,
  label normalization, doctor's empty-field checks — on both read and write
  paths, and it changes success/failure (a NEL-only title is valid in JS, blank
  in Go). Fix: a `jsTrim` with the exact ECMAScript WhiteSpace+LineTerminator set.
- **`padEnd`/`padStart` count UTF-16 code units, not bytes or runes.** An emoji
  is width-2 in UTF-16. Fix: measure pad width in UTF-16 units.
- **Lone surrogates round-trip.** `JSON.parse`/`stringify` preserve a lone
  surrogate `\ud800`; Go `encoding/json` replaces it with U+FFFD. In HUMAN/console
  output, JS emits U+FFFD for a lone surrogate. So: JSON path must PRESERVE
  (WTF-8 storage + re-emit `\udXXX`), human path must REPLACE with U+FFFD. Fix:
  a hand-rolled JSON string parse/encode that carries lone surrogates, plus a
  display-sanitizer for console paths.

## Numbers

- **`JSON.stringify` / `Number.prototype.toString` formatting.** `-0` → `"0"`;
  `1e-7` → `"1e-7"` (no padded exponent, unlike Go's `1e-07`); exponent
  thresholds (`1e21` switches to exponent form, `1e-7` for small). Fix: port the
  ECMAScript Number-to-string algorithm over Go's shortest-round-trip digits.
- **Numbers are float64; no silent int overflow/truncation.** Parsing a priority
  of `2^64` keeps it as a float in JS; a Go `int` parse overflows/truncates.
  Fractional values sort by raw float, not truncated int. Fix: model JS Numbers
  as float64 through parse + compare.

## Object / map semantics

- **Object key order = integer-index keys ascending FIRST, then string keys in
  insertion order** (ES2015 OrdinaryOwnPropertyKeys). A JS object/`Record` with
  keys `"10","2"` serializes as `2,10`. An "integer index" is a canonical
  array-index string (`String(Number(k))===k`, `0 ≤ n < 2^32-1`) — `"-1"`,
  `"2.5"`, `"01"`, `"-0"` are NOT indices and stay in insertion order. Go maps
  are unordered; an insertion-ordered port object must additionally apply this
  index-first rule on emit. Bites byPriority/byType/byLabel, extensions, config.
- **Map keys are type-distinct.** JS `Map` keys `1` (number) and `"1"` (string)
  are DISTINCT; `1` and `1.0` collapse. A dedup keyed on a stringified id
  mis-merges numeric ids. Fix: a type-tagged dedup key.
- **`undefined` vs `null` vs absent.** JS omits `undefined`-valued fields from
  JSON entirely (never `null`); accessing a missing property yields `undefined`,
  which `String(...)` renders `"undefined"`. A non-object JSONL line ("any valid
  JSON value is an issue") makes property access yield `undefined` everywhere.

## Parsing / runtime error strings

- **Runtime JSON-parse error wording is non-portable.** JSC says
  `JSON Parse error: Unexpected identifier "not"`; Go `encoding/json` says
  something else entirely. Mask both to a token, but note the mask is bounded at
  the first quote (to protect JSON-string contexts), so JSC's quoted-token echo
  survives — choose fixture inputs whose runtime error has no double-quote, and
  have the port emit a fixed quote-free detail behind the seeds-authored prefix.

## Process / IO

- **`--timing`-style wall-clock output is genuine entropy** — exclude from the
  corpus (don't try to mask `⏱ <N>ms`).
- **Broken pipe / SIGPIPE, concurrent lock contention, partial writes** — probe
  these but verify reproducibility (TS-vs-TS) before treating as a port bug.
</content>
