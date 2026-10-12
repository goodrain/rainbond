#!/usr/bin/env python3
"""Create platform-owned Registry keys without printing or overwriting them.

Only touches two Secrets; never changes workloads, Service ports or storage.
Requires an explicit kubectl context. Existing credentials are validated before
any write. A partial run can be resumed without rotating the surviving key.
"""
import argparse
import base64
import json
import re
import secrets
import subprocess
import sys

CONTROL = 'rainbond-registry-control'
PERMIT = 'rainbond-registry-permit'
NAMESPACE = 'rbd-system'


class InvalidIdentity(Exception):
    pass


def new_credentials(enterprise, region):
    result = []
    for name in (CONTROL, PERMIT):
        key = base64.b64encode(secrets.token_hex(32).encode('ascii')).decode('ascii')
        data = {'key': key}
        if name == CONTROL:
            document = {'enterprise': enterprise, 'region': region, 'key': key}
            data['console.json'] = base64.b64encode(json.dumps(document).encode('utf-8')).decode('ascii')
        result.append({'apiVersion': 'v1', 'kind': 'Secret', 'type': 'Opaque', 'immutable': True,
                       'metadata': {'name': name, 'namespace': NAMESPACE, 'annotations': {
                           'cleanup.rainbond.io/enterprise': enterprise, 'cleanup.rainbond.io/region': region,
                           'cleanup.rainbond.io/purpose': 'platform-registry-coordination'}}, 'data': data})
    return result


def validate_secret(secret, name, enterprise, region):
    try:
        metadata = secret['metadata']
        annotations = metadata['annotations']
        data = secret['data']
        if (secret.get('apiVersion') != 'v1' or secret.get('kind') != 'Secret' or secret.get('type') != 'Opaque'
                or secret.get('immutable') is not True or metadata.get('name') != name
                or metadata.get('namespace') != NAMESPACE or metadata.get('ownerReferences')
                or annotations.get('cleanup.rainbond.io/enterprise') != enterprise
                or annotations.get('cleanup.rainbond.io/region') != region
                or annotations.get('cleanup.rainbond.io/purpose') != 'platform-registry-coordination'
                or set(data) != ({'key', 'console.json'} if name == CONTROL else {'key'})):
            raise InvalidIdentity()
        key = base64.b64decode(data['key'], validate=True)
        if not re.fullmatch(rb'[a-f0-9]{64}', key):
            raise InvalidIdentity()
        if name == CONTROL:
            document = json.loads(base64.b64decode(data['console.json'], validate=True))
            if document != {'enterprise': enterprise, 'region': region, 'key': data['key']}:
                raise InvalidIdentity()
    except Exception:
        raise InvalidIdentity('existing identity is invalid; no overwrite allowed') from None


def provision(enterprise, region, read, create):
    if not all(re.fullmatch(r'[A-Za-z0-9_-]{1,128}', item) for item in (enterprise, region)):
        raise InvalidIdentity('invalid scope')
    existing = {name: read(name) for name in (CONTROL, PERMIT)}
    for name, secret in existing.items():
        if secret is not None:
            validate_secret(secret, name, enterprise, region)
    candidates = dict(zip((CONTROL, PERMIT), new_credentials(enterprise, region)))
    effective = {name: existing[name] if existing[name] is not None else candidates[name] for name in (CONTROL, PERMIT)}
    if effective[CONTROL]['data']['key'] == effective[PERMIT]['data']['key']:
        raise InvalidIdentity('control and permit identities must be independent')
    for name in (CONTROL, PERMIT):
        if existing[name] is None:
            create(effective[name])
    # Readback catches partial writes or conflicting concurrent installers.
    for name in (CONTROL, PERMIT):
        actual = read(name)
        validate_secret(actual, name, enterprise, region)
        if actual['data'] != effective[name]['data']:
            raise InvalidIdentity('identity changed during provisioning')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--context', required=True)
    parser.add_argument('--enterprise', required=True)
    parser.add_argument('--region', required=True)
    args = parser.parse_args()
    command = ['kubectl', '--context', args.context, '--request-timeout=20s', '-n', NAMESPACE]

    def invoke(arguments, payload=None):
        result = subprocess.run(command + arguments, input=payload, stdout=subprocess.PIPE,
                                stderr=subprocess.PIPE, timeout=30, check=False)
        if result.returncode:
            # kubectl errors can include submitted documents; never echo them.
            raise InvalidIdentity('Kubernetes operation failed; inspect resource metadata before retrying')
        return result.stdout

    def read(name):
        raw = invoke(['get', 'secret', name, '--ignore-not-found', '-o', 'json'])
        return json.loads(raw) if raw.strip() else None

    def create(secret):
        invoke(['create', '-f', '-', '-o', 'name'], json.dumps(secret).encode('utf-8'))

    try:
        provision(args.enterprise, args.region, read, create)
    except Exception:
        print('Identity provisioning failed; existing credentials were not overwritten.', file=sys.stderr)
        return 1
    print('Platform Registry identity Secrets verified. Workloads and ingress unchanged.')
    return 0


if __name__ == '__main__':
    sys.exit(main())
