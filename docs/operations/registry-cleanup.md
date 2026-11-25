# Registry cleanup schedule

Chaos schedules one cleanup at startup and then once daily. The defaults are
03:00 in `Asia/Shanghai`, independent of the host timezone. All Chaos replicas
share a startup operation for the same DaemonSet UID/generation, and share the
same daily operation for a scheduled timestamp. Restarts of an unchanged rollout
do not repeat a completed startup cleanup.

## Configuration

Use these Chaos arguments for the coordinated cleanup path:

```text
--clean-up=false
--scheduled-clean-up=true
--clean-time=03:00
--clean-timezone=Asia/Shanghai
--keep-count=5
```

`--scheduled-clean-up=false` disables the new scheduler. The legacy `--clean-up`
loop is removed; leave its explicit false argument in the deployed Pod so the
existing GC admission checks can verify that no old cleaner is active.
`--clean-interval` is deprecated and ignored. `--clean-time` accepts strict
24-hour `HH:MM`; the timezone must be an explicit IANA name or UTC. IANA data is
embedded, so the image does not depend on the host tzdata package.

## Execution requirements

The scheduler uses the existing Core coordination mechanism. The system Registry
must already have its storage identity, coordinated ingress and participants
registered and independently verified. Its native listener must be accessible
only through the coordinated ingress, and its upload purger must be disabled.
The existing GC executor must be installed and pinned by digest. Chaos needs the
same existing deletion-permit key reference used by Core, mounted from the
installation Secret through `CLEANUP_REGISTRY_PERMIT_KEY_FILE`; no key belongs in
command arguments or documentation.

This path relies on the complete producer admission used by Rainbond builds,
imports and image publishing. An external registry client must hold the existing
producer admission for its entire push, including the interval between blob and
manifest uploads. HTTP request admission alone is not a complete external push
session. An installation with uncoordinated writers must not be certified ready
for automatic cleanup.

Missing or changed prerequisites skip cleanup and log a controlled warning.
There is no fallback to native online `registry garbage-collect` from Chaos.

## Concurrency and recovery

A busy build or push postpones the same cleanup operation by one minute. Under
the storage-row lock, scheduled GC enters maintenance only when no producer or
registry request remains unfinished. The winning maintenance closes new writer
admission before the single original GC Job is started. A concurrent writer
therefore either enters first and delays GC, or receives the existing busy
response and retries after maintenance.

Old successful build versions are considered beyond the configured retention
count. Current versions, active service operations, retained aliases, system
images and references covered by the existing Core audit remain protected.
Records for one manifest are retired atomically only after the remaining
reference audit succeeds. A durable pending receipt preserves deletion candidates
across a subsequent read failure or process restart. A destructive HTTP request
with an unknown outcome is not automatically replayed.

The original Job, actual terminated executor and durable measurements must all
be verified before writer admission is restored. Cancellation before execution
can cancel a verified unstarted operation. Once execution has been granted, an
unknown result or changed source remains protected and requires the existing
coordination recovery workflow; it is not unlocked by a timer or lease expiry.

## Verification

```sh
go test ./builder/clean ./builder/sources/registry ./config/configs/rbdcomponent ./pkg/cleanup ./pkg/cleanup/kubeidentity ./api/controller
go build ./...
go vet ./...
python3 scripts/validate_test_manifest.py --repo-root . --manifest test-manifest.json
```

Unit and integration tests use isolated databases, fake Kubernetes objects and
owned HTTP endpoints. They do not run GC against the live platform Registry.
