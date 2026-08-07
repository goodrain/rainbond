# VM image reference validation implementation spec

Design: `docs/plans/2026-08-07-vm-image-reference-validation-design.md`

The approved minimal implementation validates URL/upload image names in `rainbond-ui`, applies the same pre-persistence guard in `rainbond-console`, and retains a final full-reference guard in `rainbond` before DataVolume creation. Existing-image selection and API success contracts remain unchanged.

The source of truth is [vm-image-reference-validation.yaml](vm-image-reference-validation.yaml).
