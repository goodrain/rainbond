# Dameng External Database Compatibility Execution Specification

Source design: [`docs/plans/2026-08-19-dameng-external-database-compatibility-design.md`](../../docs/plans/2026-08-19-dameng-external-database-compatibility-design.md)

This specification replaces the unapproved Operator migration-job path for the current test delivery. It supports an explicitly initialized external DM8 instance while preserving all default MySQL paths.

Commit groups:

1. `test: cover Dameng database compatibility boundaries` — regression tests and governed manifests.
2. `fix: initialize and verify Dameng region schemas` — Core schema ownership, idempotence and DAO capability fixes.
3. `fix: support Dameng console database workflows` — Console initialization, query result normalization and audited SQL fixes.
4. `test: verify Dameng and MySQL platform workflows` — clean-schema matrix and final release evidence.

Safeguards:

- No Operator, CRD, deployment controller or UI source changes.
- Tests are written before the corresponding behavior change and registered in each repository test manifest.
- MySQL default values, DSNs, startup flow and result keys are regression-tested unchanged.
- No credentials appear in source, test fixtures, logs, commands or documentation.
- The final image build is triggered only after source tests and clean-schema acceptance pass.
