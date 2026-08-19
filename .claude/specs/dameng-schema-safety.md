# Dameng Schema Safety — Execution Spec

Design: `docs/plans/2026-08-19-dameng-full-compatibility-design.md`

## Commit 1 — `fix: fail Dameng startup on schema initialization errors`

1. Add a failing paired regression test to `db/mysql/mysql_dameng_test.go`.
   - DM schema errors must include the table name and fail initialization.
   - MySQL keeps the existing log-and-continue behavior.
2. Register the managed capability with `scripts/manage_test_manifest.py`.
3. Change `Manager.CheckTable` to return an error only for the DM path; wire that error through `CreateManager` after closing the opened connection.
4. Run focused tests, the package tests, manifest validation, `go build`, and `go vet` before the commit.

This is intentionally the first vertical slice only. The next specs cover Core bootstrap/DAO SQL, Console raw SQL, and Operator CR reconciliation after their source audit is complete.
