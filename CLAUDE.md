# AGENTS.md

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

## Session workflow
- Begin each work session by reviewing:
  - ARCHITECTURE.md
- Create a deployment document
  - DEPLOYMENT.md
- If issues are discovered, add concise troubleshooting notes and prevention tips to:
  - DEPLOYMENT.md
- Keep relevant README files up to date when behavior, configuration, or workflows change.

## Instruction precedence
- Resolve conflicts in this order:
  - Direct user request
  - AGENTS.md
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
  - Docs/md/Architecture.md
- When a design choice is not yet final, capture it in the Open Decisions section.

## Testing and verification
- Add tests for new behavior and regressions when feasible.
- Run or suggest relevant checks (unit tests, lint, build) after changes.
- Note any gaps or risks if tests are not available.

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
- Add ADRs for significant architectural decisions.
- Add a lightweight runbook for incident triage.
 