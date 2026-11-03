import copy
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import Mock
import urllib.error
import zipfile

from delivery_source import DeliveryError

spec = importlib.util.spec_from_file_location("fetch_delivery", Path(__file__).with_name("fetch-delivery.py"))
fetcher = importlib.util.module_from_spec(spec)
spec.loader.exec_module(fetcher)
SOURCE = "a" * 40
MEMBER = "ios-delivery.tar.gz.age"
ENCRYPTED = b"age-encryption.org/v1\nsynthetic encrypted bytes, never decrypted"


class FakeTransport:
    def __init__(self, members=None):
        data = io.BytesIO()
        with zipfile.ZipFile(data, "w") as package:
            for name, content in (members or [(MEMBER, ENCRYPTED)]):
                package.writestr(name, content)
        self.archive = data.getvalue()
        self.metadata = {"id": 100, "run_attempt": 2, "head_sha": SOURCE, "head_branch": "main", "event": "workflow_dispatch",
                         "path": ".github/workflows/ios-test-delivery.yml", "status": "completed", "conclusion": "success",
                         "repository": {"full_name": fetcher.REPO}}
        self.jobs = {"total_count": 2, "jobs": [{"name": name, "head_sha": SOURCE, "run_id": 100,
                      "status": "completed", "conclusion": "success"} for name in ("trusted-source", "delivery")]}
        self.artifact = {"id": 200, "name": "ios-delivery-encrypted-100-2", "expired": False, "expires_at": "2099-01-01T00:00:00Z",
                         "workflow_run": {"id": 100, "head_sha": SOURCE}, "size_in_bytes": len(self.archive),
                         "digest": "sha256:" + hashlib.sha256(self.archive).hexdigest()}
        self.ci = {"workflow_runs": [{"head_sha": SOURCE, "head_branch": "main", "event": "push", "run_number": 5,
                   "run_attempt": 1, "status": "completed", "conclusion": "success"}]}
        self.downloads = 0

    def json(self, path):
        if "workflows/ci.yml" in path: return self.ci
        if "/jobs?" in path: return self.jobs
        if "actions/artifacts/" in path: return self.artifact
        return self.metadata

    def get(self, url, consume, **kwargs):
        if kwargs.get("redirect"):
            return "https://synthetic.blob.core.windows.net/encrypted?private=do-not-print"
        assert kwargs.get("authenticated") is False
        self.downloads += 1
        return consume(io.BytesIO(self.archive))


class FetchTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.folder = Path(self.temp.name)
        self.transport = FakeTransport()

    def fetch(self):
        return fetcher.fetch("ios", SOURCE, 100, 2, 200, self.folder, self.transport)

    def test_verified_cache_reuse_and_no_plaintext(self):
        result = self.fetch()
        self.assertFalse(result["cache_reused"])
        self.assertEqual((self.folder / MEMBER).read_bytes(), ENCRYPTED)
        self.assertTrue(self.fetch()["cache_reused"])
        self.assertEqual(self.transport.downloads, 1)
        self.assertEqual({p.name for p in self.folder.iterdir()}, {"artifact.zip", MEMBER, "VERIFIED.json"})
        self.assertNotIn("private=", (self.folder / "VERIFIED.json").read_text())
        self.assertTrue(all(p.stat().st_mode & 0o077 == 0 for p in self.folder.iterdir()))

    def test_wrong_identity_missing_jobs_expiry_and_latest_ci_fail_before_download(self):
        original = copy.deepcopy(self.transport)
        for target, key, value in [("metadata", "head_sha", "b" * 40), ("metadata", "run_attempt", 3),
                                   ("metadata", "path", ".github/workflows/untrusted.yml"),
                                   ("jobs", "jobs", []), ("artifact", "expired", True),
                                   ("artifact", "expires_at", "2000-01-01T00:00:00Z"),
                                   ("artifact", "digest", ""), ("artifact", "name", "ios-delivery-encrypted-100-1")]:
            self.transport = copy.deepcopy(original)
            getattr(self.transport, target)[key] = value
            with self.subTest(target=target, key=key), self.assertRaises(DeliveryError): self.fetch()
            self.assertEqual(self.transport.downloads, 0)
        self.transport = original
        self.transport.ci["workflow_runs"].append({**self.transport.ci["workflow_runs"][0], "run_attempt": 2, "conclusion": "failure"})
        with self.assertRaises(DeliveryError): self.fetch()

    def test_corrupted_transfer_and_cache_are_not_accepted(self):
        self.transport.archive += b"unexpected bytes"
        with self.assertRaises(DeliveryError): self.fetch()
        self.assertEqual(list(self.folder.iterdir()), [])
        self.transport = FakeTransport(); self.fetch()
        (self.folder / "artifact.zip").write_bytes(b"corruption")
        with self.assertRaises(DeliveryError): self.fetch()
        self.assertEqual(self.transport.downloads, 1)

    def test_unsafe_or_plaintext_archive_never_emits_a_verified_result(self):
        for members in [[("../" + MEMBER, ENCRYPTED)], [(MEMBER, ENCRYPTED), ("extra", b"x")],
                        [(MEMBER, b"not an encrypted archive" * 3)]]:
            with tempfile.TemporaryDirectory() as temp:
                with self.subTest(members=[name for name, _ in members]), self.assertRaises(DeliveryError):
                    fetcher.fetch("ios", SOURCE, 100, 2, 200, Path(temp), FakeTransport(members))
                self.assertFalse((Path(temp) / "VERIFIED.json").exists())
                self.assertFalse((Path(temp) / MEMBER).exists())

    def test_shared_directory_and_symlink_outputs_rejected(self):
        self.folder.chmod(0o755)
        with self.assertRaises(DeliveryError): self.fetch()
        self.folder.chmod(0o700)
        (self.folder / "artifact.zip").symlink_to("outside")
        with self.assertRaises(DeliveryError): self.fetch()


class TransportTests(unittest.TestCase):
    def test_get_retry_bound_and_safe_diagnostics(self):
        transport = fetcher.Transport("synthetic-token", pause=lambda seconds: None)
        transport.opener = Mock()
        transport.opener.open.side_effect = urllib.error.URLError("https://private.invalid/?credential=never-print")
        with self.assertRaises(DeliveryError) as error:
            transport.json("actions/runs/100")
        self.assertEqual(transport.opener.open.call_count, 3)
        self.assertNotIn("credential", str(error.exception))
        transport.opener.open.reset_mock()
        transport.opener.open.side_effect = urllib.error.HTTPError("private", 403, "secret", {}, None)
        with self.assertRaises(DeliveryError): transport.json("actions/runs/100")
        self.assertEqual(transport.opener.open.call_count, 1)

    def test_authentication_is_never_forwarded_and_redirects_are_not_followed(self):
        transport = fetcher.Transport("synthetic-token")
        transport.opener = Mock()
        response = Mock(status=200)
        response.__enter__ = Mock(return_value=response); response.__exit__ = Mock(return_value=False)
        transport.opener.open.return_value = response
        transport.get("https://artifact.blob.core.windows.net/bytes?private=secret", lambda result: None, authenticated=False)
        request = transport.opener.open.call_args.args[0]
        self.assertFalse(request.has_header("Authorization"))
        for url in ["https://evil.invalid/", "http://artifact.blob.core.windows.net/", "https://user@artifact.blob.core.windows.net/",
                    "https://artifact.blob.core.windows.net.evil.invalid/", "https://artifact.blob.core.windows.net:444/"]:
            with self.subTest(url=url), self.assertRaises(DeliveryError):
                transport.get(url, lambda result: None, authenticated=False)
        self.assertIsNone(fetcher.NoRedirect().redirect_request(None, None, 302, "", {}, "https://evil.invalid"))
