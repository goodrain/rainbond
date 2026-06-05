# VM Fixed IP Fallback Spec

Design doc: [2026-04-11-vm-fixed-ip-fallback-design.md](../docs/plans/2026-04-11-vm-fixed-ip-fallback-design.md)

## Commit Groups

1. `feat: support vm pod fixed ip fallback`
   - Go worker fallback path
   - Go capability response update
   - Go unit tests

2. `feat: allow vm fixed ip without business network`
   - Console validation change
   - Console regression tests

3. `feat: adapt vm fixed ip form to network capability`
   - UI form interaction update
   - Locale copy adjustments if needed

4. `chore: verify vm fixed ip fallback`
   - Cross-repo verification only

## Execution Notes

- Implementation order follows Rainbond cross-repo rules:
  1. `rainbond`
  2. `rainbond-console`
  3. `rainbond-ui`
- Pod fixed IP path is represented by:
  - `network_mode=fixed`
  - `network_name=""`
  - `fixed_ip="<ip-or-cidr>"`
- Business network fixed IP path keeps existing semantics.
- `fixed_ip` may contain CIDR; annotation logic must normalize to host IP.
