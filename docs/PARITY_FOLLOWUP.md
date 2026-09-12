# Follow-up: complete the `kubectl get` parity fix (post-v0.9.5)

**v0.9.5 shipped the parity fix only partially.** The merge
`8e32235 Merge fix/default-template-matches-kubectl-get for v0.9.5` landed at
commit `7ed2b91`, which contains only the **built-in column-set** fix (default
template uses the non-wide column set instead of `-o wide`). Two later commits
on the same branch — pushed after that merge point — did **not** make it into
the v0.9.5 tag.

## What still needs to land

Branch `fix/default-template-matches-kubectl-get` is rebased onto the current
`main` (2 commits ahead, 0 behind) and carries exactly the missing work:

1. **CRD default = NAME + AGE.** For a CRD with no `additionalPrinterColumns`,
   `kubectl get <cr>` shows NAME + AGE (the apiserver table convertor injects an
   Age column). `init` was emitting only NAME, so `kc cwide get <cr>` showed
   fewer columns. Fix appends an AGE column in that case.

2. **Value parity for wide-shaped columns (`wantsWide`).** `printOneObject`
   rendered the default-printer table with `Wide: true` hardcoded. A few
   printers fold `Wide` into a base cell's *value* (not just column count) — a
   LoadBalancer Service's EXTERNAL-IP is truncated with `...` under non-wide but
   full under wide. So `kc cwide get svc` diverged from `kubectl get svc`. Fix
   adds `wantsWide(kind)`: render `Wide: false` unless a template column
   references a wide-only column. Combined cleanly with main's
   `needsDefaultPrinter()` guard (build the table only when needed, at the width
   actually needed).

Both come with tests (`wants_wide_test.go`, `BuildYAMLColumnTemplate` cases).

## Why it matters

Without these, the "`kc cwide get <alias>` == `kubectl get <resource-name>`
unless `-t` is given" guarantee holds for built-in resources' column *set* but
not for CRDs or for Service/Ingress EXTERNAL-IP *values*. Landing this branch
completes the guarantee.

## Coverage across render paths (verified by inspection)

The fix lives in `CustomColumnsPrinter.printOneObject` (`wantsWide` +
`needsDefaultPrinter`), and every `get` render path routes through it via
`createPrinter` → `PrintObj`:

- single-kind table (`list`),
- multi-kind / alias-group blocks (`listMultiKind`, which renders via
  `RowSink` + `PrintObj(info, io.Discard)` — the `needsDefaultPrinter` guard is
  what keeps that generator-less path from panicking),
- structured output (`emitStructured`: `-o csv|template-json|template-yaml`),
- watch (`watch`).

So the parity fix applies uniformly — no path has its own divergent
`GenerateTable` call that would bypass the width decision.

## Suggested action

Merge `fix/default-template-matches-kubectl-get` into `main` and include it in
the next patch (v0.9.6). It's already rebased and conflict-free; CI just needs
to confirm build/test (this batch was authored while the local Go toolchain was
unavailable, so it is inspection-verified but not locally compiled — the earlier
built-in part of the same branch did pass a local build when the toolchain was
briefly up).
