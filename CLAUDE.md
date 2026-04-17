# CLAUDE.md

## Purpose
This file defines how contributors (including AI agents) should work in this repo.
It aims to keep changes safe, maintainable, and aligned with the documented architecture.

## Core principles
- Prefer simple, readable solutions over clever ones.
- Optimize for correctness and clarity first, then performance.
- Make changes in small, reviewable steps.
- Keep behavior changes explicit and documented.
- Use coding agents using Sonnet
- Only the architecture agent should use Opus

## Documentation
- Full project documentation: https://github.com/badskater/encodeswarmr/wiki
- Architecture: ARCHITECTURE.md (in repo)
- Deployment (canonical entry point): DEPLOYMENT.md (in repo) — links out to wiki deep dives
- Agent details: https://github.com/badskater/encodeswarmr/wiki/Agents
- Deployment deep dive: https://github.com/badskater/encodeswarmr/wiki/Deployment
- Incident triage runbook: https://github.com/badskater/encodeswarmr/wiki/Runbook
- Operational semantics: https://github.com/badskater/encodeswarmr/wiki/Operational-Semantics
- Testing guide: https://github.com/badskater/encodeswarmr/wiki/Testing-Guide
- gRPC reference: https://github.com/badskater/encodeswarmr/wiki/gRPC-Reference
- Architecture Decision Records: https://github.com/badskater/encodeswarmr/wiki/ADR-Index

## Session workflow
- Begin each work session by reviewing:
  - ARCHITECTURE.md
  - DEPLOYMENT.md (canonical deployment entry point)
  - The [Wiki](https://github.com/badskater/encodeswarmr/wiki) for current roadmap and feature status
- If deployment issues are discovered, add concise troubleshooting notes and prevention tips to:
  - DEPLOYMENT.md §10 (quick list), and
  - The [Runbook](https://github.com/badskater/encodeswarmr/wiki/Runbook) wiki page for the full scenario
- Keep relevant README files and Wiki pages up to date when behavior, configuration, or workflows change.
- When the proto contract changes, regenerate stubs (`make proto`) AND update the [gRPC Reference](https://github.com/badskater/encodeswarmr/wiki/gRPC-Reference) wiki page.

## Instruction precedence
- Resolve conflicts in this order:
  - Direct user request
  - CLAUDE.md
  - Repository READMEs and defaults
- If instructions conflict and priority is unclear, call out the conflict explicitly before proceeding.

## Change scope and isolation
- Edit only files required to complete the requested task.
- Avoid unrelated refactors in the same change.
- Keep commits focused to one logical change whenever practical.

## Review output standard
- For review requests, present findings first.
- Include severity, file path + line reference, impact, and recommended fix.
- Keep summaries brief and secondary to findings.

## Coding practices
- Follow existing patterns, naming, and structure in the codebase.
- Validate inputs, handle errors, and avoid silent failures.
- Keep functions focused; extract shared logic when it improves clarity.
- Avoid magic values; use constants or configuration when appropriate.
- Maintain idempotency where the workflow requires it.
- Always use .env files instead of including secrets in code.
- Coding should be done using teams.

## Documentation expectations
- Add or update docstrings, README, or inline comments when behavior is not obvious.
- For scripts and automation, include usage notes, parameters, and examples.
- Keep documentation concise and actionable.
- For new/changed configuration:
  - Update files in the relevant app.
  - Update the relevant app README.
- Do not rely on undocumented environment variable defaults for production behavior.

## Architecture documentation
- If a change impacts system flows, data contracts, or infrastructure, update:
  - ARCHITECTURE.md
- When a design choice is not yet final, capture it in the Open Decisions section.

## Testing and verification
- Tests are part of the code, not an afterthought. Every change — new feature, bug fix, or refactor — ships with tests covering its behavior. Code without tests does not merge.
- All tests run as part of CI on every push and PR (`go test ./... -race -cover`, the `integration`-tagged suite against a real PostgreSQL, `npm test` for the web, and cross-compile builds for controller/agent/desktop on both linux and windows). No code merges without CI green.
- Before marking work complete, locally run:
  - `make test` (Go unit + race + cover)
  - `cd web && npm test && npx tsc --noEmit` when web is touched
  - `make lint` when practical
- New features require tests at the appropriate layer (unit in-package, integration under `tests/integration/` with `//go:build integration`, or web `*.test.{ts,tsx}` via Vitest). See the [Testing Guide](https://github.com/badskater/encodeswarmr/wiki/Testing-Guide) wiki page for conventions, patterns, and examples.
- The only acceptable exceptions are code paths that genuinely cannot be tested without a live display or an external system — these must be called out explicitly in the PR description along with why.
- Note any coverage gaps or risks explicitly when they exist.

## Security and secrets
- Never hardcode secrets.
- Use environment files and managed identities.
- Follow least-privilege access for new resources.

## Observability
- Include correlation identifiers in logs where applicable.
- Log failures with enough context to diagnose issues.

## Commit hygiene
- Use small, focused commits with clear messages.
- Do not mix feature changes with unrelated formatting or cleanup.
- After each commit, confirm the working tree is clean unless explicitly leaving staged follow-up work.

## Suggested future additions
- Dynamic / external plugin loading (see [Plugin Loading](https://github.com/badskater/encodeswarmr/wiki/Plugin-Loading) for current compile-time model).
 
