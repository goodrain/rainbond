# Dameng Complete Platform Execution Specification

Source design: [`docs/plans/2026-08-19-dameng-complete-platform-design.md`](../../docs/plans/2026-08-19-dameng-complete-platform-design.md)

This specification deliberately treats DM8 support as an installation path, not as a collection of pod-level overrides. The delivery boundary is one compatible image set and one declarative `RainbondCluster` configuration.

## Commit sequence

1. `feat: configure Dameng databases through RainbondCluster` — Operator type, manifest rendering, precheck and one gated Region migration Job.
2. `fix: migrate Dameng schemas before region startup` — Core opens/migrates/verifies separately, makes bootstrapping idempotent, fixes Region pagination, and ships the Job executable.
3. `fix: support Dameng queries in console workflows` — Console capabilities, parameterized SQL and every audited database-specific path.
4. `test: verify MySQL and Dameng platform startup` — fresh-schema integration against DM and non-regression MySQL verification.

## Execution safeguards

- Tests are written before behavior changes and registered in each repository’s `test-manifest.json`.
- The migration Job is the only DM Region DDL writer. API, Worker and Chaos wait for it and then verify schema only.
- Existing MySQL defaults, `--mysql` argument and `MYSQL_*` variables remain unchanged.
- A fresh DM schema/instance is used for acceptance; failed experiments are not reused as test evidence.
- Database passwords are read only from Kubernetes Secrets/environment at runtime and never logged or rendered into repository fixtures.
- The final release workflow must identify the Operator source branch explicitly because the Core development branch is not currently present in the Operator repository.

Detailed task steps, exact source locations, test commands, and acceptance conditions are in [`dameng-complete-platform.yaml`](dameng-complete-platform.yaml).
