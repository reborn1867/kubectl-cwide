# Bug + fix design: default-printer render uses Wide=true, breaking value parity

Status: **diagnosed, fix deferred to a compiler-in-the-loop session.** This is a
follow-on to `fix/default-template-matches-kubectl-get` (which fixed the column
*set*); this note is about a column *value* difference.

## Symptom

Even with the correct non-wide column set, `kc cwide get svc` can show a
different EXTERNAL-IP value than `kubectl get svc` for a LoadBalancer service:
cwide shows the full address, kubectl (non-wide) shows it truncated with `...`.

## Root cause

`printOneObject` (pkg/cmd/get/customcolumn.go) calls the default-printer table
generator with `Wide: true` hardcoded:

    t, _ := s.GenerateTable(obj, k8sprinters.GenerateOptions{NoHeaders: ..., Wide: true})

For most resources `Wide` only *appends* extra columns (safe — selecting the
non-wide columns yields identical cells). But a few printers feed `Wide` into a
base column's *value*, not just column count:

- Service EXTERNAL-IP: `getServiceExternalIP` → `loadBalancerStatusStringer(s, wide)`
  truncates to `loadBalancerWidth` with `...` when `!wide`
  (vendor/k8s.io/kubernetes/pkg/printers/internalversion/printers.go:1311).
- Ingress ADDRESS: `ingressLoadBalancerStatusStringer(s, wide)` — same pattern
  (printers.go:1435 / 1444).

So rendering with `Wide: true` diverges from `kubectl get` (non-wide) for these
columns.

## Fix (implement with `go test` running)

Render with `Wide` matching what the template actually needs:

- Default rule: `Wide: false`, so value-shaping (LB truncation, etc.) matches
  `kubectl get`. With the init parity fix, default templates only contain
  non-wide columns, so `false` is correct and sufficient for them.
- Escalate to `Wide: true` only when the template references a column that
  exists *only* in the wide set. Detect by comparing each default-printer
  column's header against the kind's non-wide header set (derivable from
  `handler.columnDefinitions` + `Priority`, the same data
  `ResourceColumnDefinitionFiltered` already filters on).

Sketch: add `CustomColumnsPrinter.wantsWide()` that returns true iff some
`$_defaultPrinterField` column header is absent from the non-wide column set for
the object's kind; pass its result as `GenerateOptions.Wide`. Compute the
non-wide header set once per kind and memoize.

## Why deferred

This is render-path logic with per-resource column semantics and a real risk of
regressing the common case (or leaving a user's added wide column empty). It
needs a build + the get tests (and ideally a live cluster spot-check on a
LoadBalancer service) to land safely — not blind authoring. Scoped here so the
future implementation is mechanical.

## Test plan

- LoadBalancer service with a long ingress hostname: default template →
  truncated EXTERNAL-IP matching `kubectl get svc`; a template with an explicit
  wide column → full value.
- Pod default template unaffected (wide is purely additive there).
- `wantsWide()` true only when a wide-only header is referenced.
