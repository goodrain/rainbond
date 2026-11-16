"""Exercise the real Skopeo client against isolated local registry endpoints."""

import base64
import gzip
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import io
import json
import os
from pathlib import Path
import secrets
import subprocess
import tarfile
import tempfile
import threading
import unittest
from urllib.parse import parse_qs, urlsplit


SCRIPT = Path(__file__).with_name("sync-image.sh")


def digest(data):
    return "sha256:" + hashlib.sha256(data).hexdigest()


class Registry(BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def respond(self, status, body=b"", **headers):
        self.send_response(status)
        for name, value in headers.items():
            self.send_header(name.replace("_", "-"), str(value))
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        if self.command != "HEAD":
            self.wfile.write(body)

    def handle_request(self):
        server = self.server
        path = urlsplit(self.path).path
        size = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(size)
        if path == "/auth":
            server.auth_methods.append(self.command)
            if self.command != "GET":
                # Reproduce an ACR OAuth POST connection reset, without a network dependency.
                self.connection.close()
                return
            if self.headers.get("Authorization") != server.basic:
                self.respond(401)
                return
            self.respond(200, json.dumps({"token": server.token, "expires_in": 300}).encode(),
                         Content_Type="application/json")
            return
        if self.headers.get("Authorization") != "Bearer " + server.token:
            self.respond(401, WWW_Authenticate=f'Bearer realm="{server.url}/auth",service="fixture"')
            return
        if path == "/v2/":
            self.respond(200, b"{}", Content_Type="application/json")
        elif "/manifests/" in path:
            if self.command == "PUT":
                server.published = body
                self.respond(201, Docker_Content_Digest=digest(body))
            elif server.source:
                server.manifest_reads += 1
                self.respond(200, server.manifest, Content_Type=server.media_type,
                             Docker_Content_Digest=digest(server.manifest))
            else:
                self.respond(404)
        elif path.endswith("/blobs/uploads/") and self.command == "POST":
            server.uploads += 1
            if server.fail_uploads:
                server.fail_uploads -= 1
                self.respond(503)
                return
            key = str(server.uploads)
            server.pending[key] = b""
            self.respond(202, Location=f"{server.url}/v2/test/blobs/uploads/{key}")
        elif "/blobs/uploads/" in path:
            key = path.rsplit("/", 1)[-1]
            server.pending[key] += body
            if self.command == "PUT":
                blob_digest = parse_qs(urlsplit(self.path).query)["digest"][0]
                if digest(server.pending[key]) != blob_digest:
                    self.respond(400)
                    return
                server.blobs[blob_digest] = server.pending[key]
                self.respond(201, Docker_Content_Digest=blob_digest)
            else:
                self.respond(202, Location=server.url + path, Range=f"0-{len(server.pending[key])}")
        elif "/blobs/" in path:
            blob_digest = path.rsplit("/", 1)[-1]
            blob = server.blobs.get(blob_digest)
            if blob is None:
                self.respond(404)
            else:
                self.respond(200, blob, Content_Type="application/octet-stream",
                             Docker_Content_Digest=blob_digest)
        else:
            self.respond(404)

    do_GET = do_HEAD = do_POST = do_PATCH = do_PUT = handle_request


class SyncImageTest(unittest.TestCase):
    def run_sync(self, arch, oci=False, failures=0, index_only=False):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            servers = []
            try:
                for source in (True, False):
                    server = ThreadingHTTPServer(("127.0.0.1", 0), Registry)
                    server.daemon_threads = True
                    server.source = source
                    server.url = f"http://127.0.0.1:{server.server_port}"
                    server.token = secrets.token_hex(16)
                    server.basic = "Basic " + base64.b64encode(b"fixture:fixture").decode()
                    server.auth_methods, server.blobs, server.pending = [], {}, {}
                    server.published, server.manifest_reads, server.uploads = None, 0, 0
                    server.fail_uploads = failures if not source else 0
                    servers.append(server)
                    threading.Thread(target=server.serve_forever, daemon=True).start()
                source, target = servers
                tar = io.BytesIO()
                with tarfile.open(fileobj=tar, mode="w") as archive:
                    info = tarfile.TarInfo("fixture")
                    info.size = 4
                    archive.addfile(info, io.BytesIO(b"data"))
                layer = gzip.compress(tar.getvalue(), mtime=0)
                config = json.dumps({"architecture": arch, "os": "linux",
                                     "rootfs": {"type": "layers", "diff_ids": [digest(tar.getvalue())]}}).encode()
                prefix = "application/vnd.oci.image" if oci else "application/vnd.docker"
                source.media_type = prefix + (".manifest.v1+json" if oci else ".distribution.manifest.v2+json")
                source.manifest = json.dumps({
                    "schemaVersion": 2, "mediaType": source.media_type,
                    "config": {"mediaType": prefix + (".config.v1+json" if oci else ".container.image.v1+json"),
                               "digest": digest(config), "size": len(config)},
                    "layers": [{"mediaType": prefix + (".layer.v1.tar+gzip" if oci else ".image.rootfs.diff.tar.gzip"),
                                "digest": digest(layer), "size": len(layer),
                                "annotations": {"containerd.io/snapshot/stargz/toc.digest": digest(b"toc")}}],
                }).encode()
                source.blobs = {digest(layer): layer, digest(config): config}
                if index_only:
                    source.media_type = "application/vnd.oci.image.index.v1+json"
                    source.manifest = json.dumps({
                        "schemaVersion": 2, "mediaType": source.media_type,
                        "manifests": [{"digest": digest(source.manifest), "size": len(source.manifest),
                                       "mediaType": "application/vnd.oci.image.manifest.v1+json",
                                       "platform": {"os": "linux", "architecture": arch}}],
                    }).encode()
                auth_dir = root / "docker"
                auth_dir.mkdir()
                (auth_dir / "config.json").write_text(json.dumps({"auths": {
                    server.url.removeprefix("http://"): {"auth": server.basic.removeprefix("Basic ")}
                    for server in servers}}))
                # Only these isolated loopback fixtures use HTTP; production keeps TLS verification.
                (root / "skopeo").write_text(
                    '#!/bin/sh\nexec /usr/bin/skopeo "$@" --src-tls-verify=false --dest-tls-verify=false\n')
                (root / "skopeo").chmod(0o755)
                result = subprocess.run(
                    ["bash", str(SCRIPT)] + (["--index-only"] if index_only else []) +
                    [source.url.removeprefix("http://") + "/test@" + digest(source.manifest),
                     target.url.removeprefix("http://") + "/test:version-" + arch],
                    env={**os.environ, "PATH": directory + os.pathsep + os.environ["PATH"],
                         "DOCKER_CONFIG": str(auth_dir), "RUNNER_TEMP": directory, "RETRY_DELAY": "0"},
                    capture_output=True, text=True, timeout=30,
                )
                return result, source.manifest, target.published, source.blobs, target.blobs, \
                    source.manifest_reads, target.auth_methods
            finally:
                for server in servers:
                    server.shutdown()
                    server.server_close()

    def test_copy_preserves_manifest_layers_and_uses_get_auth(self):
        for arch, oci in (("amd64", False), ("arm64", True)):
            with self.subTest(arch=arch, oci=oci):
                result, manifest, published, blobs, copied, _, methods = self.run_sync(arch, oci)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(published, manifest)
                self.assertEqual(copied, blobs)
                self.assertTrue(methods)
                self.assertEqual(set(methods), {"GET"})

    def test_upload_retries_reuse_downloaded_image(self):
        result, manifest, published, _, _, reads, _ = self.run_sync("amd64", failures=1)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(published, manifest)
        self.assertEqual(reads, 1)

    def test_index_publication_does_not_retransfer_layers(self):
        result, manifest, published, _, copied, _, methods = self.run_sync("arm64", oci=True, index_only=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(published, manifest)
        self.assertEqual(copied, {})
        self.assertEqual(set(methods), {"GET"})

    def test_persistent_upload_failure_never_publishes_manifest(self):
        result, _, published, _, _, reads, _ = self.run_sync("amd64", failures=100)
        self.assertNotEqual(result.returncode, 0)
        self.assertIsNone(published)
        self.assertEqual(reads, 1)
