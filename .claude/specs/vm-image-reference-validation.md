# VM image reference validation implementation spec

Design: `docs/plans/2026-08-07-vm-image-reference-validation-design.md`

The implementation has three ordered commits: reject malformed registry DataVolume input in `rainbond`, generate and validate safe runtime references in `rainbond-console`, then clarify the display-name contract in `rainbond-ui`.

The source of truth is [vm-image-reference-validation.yaml](vm-image-reference-validation.yaml). Each task begins with a regression test, receives a spec review and code-quality review, and is committed only after its repository checks pass.
