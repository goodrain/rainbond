# cmd-args-yaml-attributes

Design doc: `/Users/zhangqihang/MyWork/workrc/rainbond/docs/plans/2026-07-22-cmd-args-yaml-design.md`

## Commit 1: `fix: parse cmd args attributes as yaml arrays`

Repository: `/Users/zhangqihang/MyWork/workrc/rainbond`

- Add parser tests with a `capability_id` marker.
- Implement YAML/JSON sequence parsing for `cmd` and `args`.
- Replace the old `strings.Split(value, " ")` fallback with whole-string compatibility.
- Verify focused Go tests and manifest validation.

## Commit 2: `fix: validate cmd args yaml attributes`

Repository: `/Users/zhangqihang/MyWork/workrc/rainbond-console`

- Add console service tests with a `capability_id` marker.
- Normalize user and internal `cmd/args` values to `save_type=yaml`.
- Validate YAML/JSON list semantics and reject scalar YAML.
- Verify focused pytest and manifest validation.

## Commit 3: `fix: edit cmd args as yaml attributes`

Repository: `/Users/zhangqihang/MyWork/workrc/rainbond-ui`

- Expose `args` in the Kubernetes attribute selector.
- Move `cmd` and `args` to YAML editing.
- Update hints so users understand that each YAML list item is one argv element and no space splitting occurs.
- Verify with `yarn build`.
