import base64
import importlib.util
import json
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location('provision', Path(__file__).with_name('provision_identity.py'))
provision = importlib.util.module_from_spec(spec)
spec.loader.exec_module(provision)


class IdentityProvisionTests(unittest.TestCase):
    def test_independent_keys_and_console_scope_are_consistent(self):
        control, permit = provision.new_credentials('enterprise', 'rainbond')
        self.assertNotEqual(control['data']['key'], permit['data']['key'])
        raw = json.loads(base64.b64decode(control['data']['console.json']))
        self.assertEqual(raw, {'enterprise': 'enterprise', 'region': 'rainbond', 'key': control['data']['key']})
        for secret in (control, permit):
            self.assertTrue(secret['immutable'])
            self.assertNotIn('ownerReferences', secret['metadata'])
            provision.validate_secret(secret, secret['metadata']['name'], 'enterprise', 'rainbond')

    def test_reuse_does_not_rotate_existing_credentials(self):
        stored = {s['metadata']['name']: s for s in provision.new_credentials('e', 'r')}
        before = json.dumps(stored, sort_keys=True)
        writes = []
        provision.provision('e', 'r', stored.get, lambda secret: writes.append(secret))
        self.assertEqual(writes, [])
        self.assertEqual(json.dumps(stored, sort_keys=True), before)

    def test_partial_installation_preserves_existing_key(self):
        control, _ = provision.new_credentials('e', 'r')
        stored = {control['metadata']['name']: control}

        def create(secret):
            stored[secret['metadata']['name']] = secret
        provision.provision('e', 'r', stored.get, create)
        self.assertEqual(stored[provision.CONTROL], control)
        self.assertNotEqual(stored[provision.CONTROL]['data']['key'], stored[provision.PERMIT]['data']['key'])

    def test_foreign_scope_or_corruption_blocks_all_writes(self):
        for mutation in ('scope', 'namespace', 'owner', 'key', 'projection'):
            control, permit = provision.new_credentials('e', 'r')
            if mutation == 'scope': control['metadata']['annotations']['cleanup.rainbond.io/enterprise'] = 'other'
            if mutation == 'namespace': control['metadata']['namespace'] = 'plugins'
            if mutation == 'owner': control['metadata']['ownerReferences'] = [{'name': 'plugin'}]
            if mutation == 'key': control['data']['key'] = 'invalid'
            if mutation == 'projection': control['data']['console.json'] = base64.b64encode(b'{}').decode()
            writes = []
            with self.assertRaises(provision.InvalidIdentity):
                provision.provision('e', 'r', {provision.CONTROL: control}.get, writes.append)
            self.assertEqual(writes, [])

    def test_reused_shared_key_is_rejected(self):
        control, permit = provision.new_credentials('e', 'r')
        permit['data']['key'] = control['data']['key']
        with self.assertRaises(provision.InvalidIdentity):
            provision.provision('e', 'r', {provision.CONTROL: control, provision.PERMIT: permit}.get, self.fail)

    def test_create_race_does_not_overwrite_or_print_credentials(self):
        stored = {}

        def raced_create(secret):
            replacement = provision.new_credentials('e', 'r')[0]
            stored[secret['metadata']['name']] = replacement
        with self.assertRaises(provision.InvalidIdentity):
            provision.provision('e', 'r', stored.get, raced_create)

    def test_cli_requires_explicit_context_and_redacts_subprocess_errors(self):
        from unittest.mock import patch
        from types import SimpleNamespace
        from contextlib import redirect_stdout, redirect_stderr
        import io
        out, err = io.StringIO(), io.StringIO()
        with patch('sys.argv', ['provision', '--context', 'isolated', '--enterprise', 'e', '--region', 'r']), \
                patch.object(provision.subprocess, 'run', return_value=SimpleNamespace(
                    returncode=1, stdout=b'private-fixture', stderr=b'private-fixture')) as run, \
                redirect_stdout(out), redirect_stderr(err):
            self.assertEqual(provision.main(), 1)
        self.assertNotIn('private-fixture', out.getvalue() + err.getvalue())
        self.assertEqual(run.call_args.args[0][:3], ['kubectl', '--context', 'isolated'])
        self.assertEqual(run.call_count, 1)
        with patch('sys.argv', ['provision', '--enterprise', 'e', '--region', 'r']), \
                patch.object(provision.subprocess, 'run') as run, redirect_stderr(io.StringIO()):
            with self.assertRaises(SystemExit):
                provision.main()
            run.assert_not_called()
