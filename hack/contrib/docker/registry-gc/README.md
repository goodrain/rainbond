# Registry coordination image

This image contains the coordinator, the one-shot GC executor, and the native
Registry binary from the explicitly supplied image. Build it from the same Core
revision as `rbd-api` and publish it outside the Registry it will coordinate.

```sh
docker build \
  --build-arg REGISTRY_IMAGE="$REGISTRY_NATIVE_IMAGE" \
  -f hack/contrib/docker/registry-gc/Dockerfile \
  -t "$GC_COORDINATOR_IMAGE" .
```

`REGISTRY_NATIVE_IMAGE` must include the digest observed on the target cluster.
The build rejects mutable tags. Use `/registry-coordinator` as the sidecar command;
the same image supplies `/registry-gc` for server-generated GC Jobs.

## Installation-scoped control authentication

Set these coordinator arguments when using the Console bridge:

```text
--coordination-api=<console-origin>
--console-enterprise=<enterprise-id>
--console-region=<region-name>
--credential-file=/control/key
--permit-key-file=/permit/key
```

The control key is the installation-scoped gateway credential. The independent
permit key is shared only by `rbd-api` and the coordinator. Configure the API with
`CLEANUP_REGISTRY_PERMIT_KEY_FILE=/var/run/cleanup-registry-permit/key` and project
that Secret read-only into both components. Do not reuse the control key as the
permit key. No Region administrator token or private client certificate is
needed in signed Console mode. Plain HTTP requires explicit
`--allow-internal-http=true` for the trusted internal Console origin.

Use separate Secrets and volumes for the two keys. GC Jobs inherit the Console
scope and control credential from the verified coordinator Pod, but never receive
the permit signing Secret. The launcher rejects shared Secret projections and
incomplete scopes. Invalid or missing explicit permit files fail closed, including
during rotation. Legacy direct-token mode remains supported when Console scope
arguments are absent.

Storage IDs, generations and physical volume identities must come from Core's
verified registry preparation response. Enrollment in `collecting` mode does not
authorize deletion. Storage, ingress, producer coverage, references, and original
execution identity still have to pass the existing readiness checks.

## Deployment boundary

These binaries and authentication settings are prerequisites. Persistent Operator
support for the coordinator container and coordinated Service/probe configuration
must be installed before switching a managed `rbd-hub`. A manual Deployment edit
is not a durable installation method. Live ingress changes require the separate
review agreed for this rollout. GC remains an explicitly confirmed, one-shot job.
