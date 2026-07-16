# template-helper skill

An agent-callable skill that turns natural-language column descriptions into a valid `kubectl-cwide` template file.

## What it is

A pair of markdown files:

- `SKILL.md` — the agent instructions. Frontmatter tells host agents when to invoke this skill; the body enumerates capabilities and restrictions.
- `references/examples.md` — worked before/after mappings the agent loads on demand.

Together they encode everything a fresh LLM needs to know to produce a template that will actually parse and render.

## How to use

**With Claude Code (or any Anthropic Agent SDK host):** the skill lives at `.claude/skills/template-helper/` and is auto-discovered. Say:

- `/template-helper` — the agent prompts you for the kind and columns
- *"Write a cwide template for pods with name, status, and node."* — the agent picks up the trigger via the description
- *"I want a template showing Secret data previews."* — same

The agent emits one code block with the template file body and a `Save to:` line telling you where to drop it.

## What the skill covers

- YAML vs `.tpl` format choice
- `fieldSpec` vs `template` per column
- All 26 built-in template functions (`humanBytes`, `age`, `truncate`, `b64dec`, `colorIf`, `safeIndex`, `lookup`, `lookupByLabel`, `probeCheck`, `toYaml`/`fromYaml`, `toJson`/`fromJson`, Helm-style pipeline helpers)
- Restrictions: no file/exec/env access, one expression kind per column, ALL_CAPS headers, API-cost warnings for `lookup`/`probeCheck`
- 10 worked examples covering simple JSONPath, computed `template:` bodies, `helpers:`/`funcs:` reuse, cross-resource lookups, live probe checks, human bytes, and `.tpl` format

## For other LLM hosts

The skill is plain markdown — copy the folder into any tool that resolves file-based skills, or paste `SKILL.md` + `references/examples.md` into a system prompt. No harness assumptions beyond "the agent can read files."
