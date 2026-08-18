# Dameng standard images specification

Source of truth: [dameng-standard-images.yaml](dameng-standard-images.yaml).

This supersedes the optional `ENABLE_DM`/`Dockerfile.dm` image route. The normal
Action-owned Dockerfiles are the only release route and contain both MySQL and
DM dependencies. Runtime selection remains `DB_TYPE=mysql` by default and
`DB_TYPE=dm` when configured.

The official driver is not vendored. Every formal Action receives it through a
private, immutable, multi-architecture OCI image configured once as the
`DAMENG_DRIVER_BUNDLE_IMAGE` repository variable. The actions must fail before
building if that variable is unavailable, rather than publishing an image that
pretends to support DM. No credential or artifact URL belongs in this spec.
