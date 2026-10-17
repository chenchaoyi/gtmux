import json
from pathlib import Path
import plistlib
import struct
import tempfile
import unittest
from unittest.mock import patch

import store_artifacts as store


def png(path, size, marker):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(b'\x89PNG\r\n\x1a\n' + b'\0' * 8 + struct.pack('>II', *size) + marker.encode())


class ArtifactTests(unittest.TestCase):
    def app(self, root):
        app = root / 'gtmux.app'
        app.mkdir()
        (app / 'Info.plist').write_bytes(plistlib.dumps({'CFBundleIdentifier': 'com.gtmux.app', 'CFBundleShortVersionString': '1.2.3', 'CFBundleSupportedPlatforms': ['iPhoneSimulator']}))
        (app / 'main.jsbundle').write_text('released code')
        return app

    def test_build_reuse_requires_exact_source_toolchain_and_content(self):
        with tempfile.TemporaryDirectory() as tmp:
            app = self.app(Path(tmp))
            args = ['v1.2.3', 'a' * 40, 'Xcode example', 'arm64']
            manifest = store.build_manifest(app, *args)
            store.verify_build(app, manifest, *args)
            for index, changed in [(0, 'v1.2.4'), (1, 'b' * 40), (2, 'Xcode other'), (3, 'x86_64')]:
                bad = args[:]
                bad[index] = changed
                with self.subTest(index=index), self.assertRaises(ValueError):
                    store.verify_build(app, manifest, *bad)
            (app / 'main.jsbundle').write_text('different code')
            with self.assertRaises(ValueError):
                store.verify_build(app, manifest, *args)

    def test_rejects_device_or_signed_app_and_external_link(self):
        with tempfile.TemporaryDirectory() as tmp:
            app = self.app(Path(tmp))
            for name in ('embedded.mobileprovision', '_CodeSignature'):
                p = app / name
                p.touch()
                with self.assertRaises(ValueError):
                    store.build_manifest(app, 'v1.2.3', 'a' * 40, 'xcode', 'arm64')
                p.unlink()
            (app / 'outside').symlink_to(Path(tmp) / 'private-key')
            with self.assertRaises(ValueError):
                store.tree_hash(app)

    def test_reuse_run_rejects_forks_prs_running_or_other_workflow(self):
        good = {'repository': {'full_name': 'owner/repo'}, 'head_repository': {'full_name': 'owner/repo'}, 'workflow_id': 42,
                'event': 'workflow_dispatch', 'head_branch': 'main', 'status': 'completed'}
        store.verify_run(good, 'owner/repo', 42)
        for change in ({'head_repository': {'full_name': 'fork/repo'}}, {'event': 'pull_request'}, {'head_branch': 'feature'}, {'status': 'in_progress'}, {'workflow_id': 43}):
            with self.subTest(change=change), self.assertRaises(ValueError):
                store.verify_run({**good, **change}, 'owner/repo', 42)

    def test_fetch_rejects_expired_and_fork_artifacts_before_download(self):
        valid = {'repository': {'full_name': 'owner/repo'}, 'head_repository': {'full_name': 'owner/repo'},
                 'workflow_id': 42, 'event': 'workflow_dispatch', 'head_branch': 'main', 'status': 'completed'}
        with tempfile.TemporaryDirectory() as tmp:
            for run, artifacts in [(valid, {'artifacts': [{'id': 1, 'name': 'store-simulator-1-1', 'expired': True}]}),
                                   ({**valid, 'head_branch': 'feature'}, {'artifacts': []})]:
                replies = [json.dumps({'id': 42}), json.dumps(run), json.dumps(artifacts)]
                with patch.object(store.subprocess, 'check_output', side_effect=replies), patch.object(store.subprocess, 'run') as download:
                    with self.assertRaises(ValueError):
                        store.fetch_artifact(Path(tmp), '123', 'simulator', 'owner/repo')
                    download.assert_not_called()

    def group(self, root, key):
        raw = root / 'input' / key
        for name in store.GROUPS[key]:
            png(raw / (name + '.png'), [2752, 2064] if key.startswith('ipad-') else [1320, 2868], key + name)
        return raw

    def test_group_checkpoint_survives_later_failure_and_changed_harness_invalidates_it(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            out = root / 'capture'
            context = {'release_tag': 'v1.2.3', 'source_commit': 'a' * 40, 'harness': 'first'}
            store.checkpoint_group(out, context, 'phone-en', self.group(root, 'phone-en'))
            with self.assertRaises(FileNotFoundError):
                store.checkpoint_group(out, context, 'phone-zh', root / 'not-captured')
            self.assertEqual(set(store.load_checkpoint(out, context)['groups']), {'phone-en'})
            with self.assertRaises(ValueError):
                store.complete_checkpoint(out, 'v1.2.3', 'a' * 40)
            for changed in ({'harness': 'changed'}, {'runtime_identity': {'buildversion': 'new'}}, {'devices': {'phone': 'new-model'}}, {'capture_tool_files': {'helper': 'new'}}):
                self.assertEqual(store.load_checkpoint(out, {**context, **changed})['groups'], {})
            (out / 'raw/phone-en/01-radar.png').write_text('corrupt')
            with self.assertRaises(ValueError):
                store.load_checkpoint(out, context)

    def test_render_requires_all_groups_and_rejects_wrong_release_even_with_valid_hashes(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            out = root / 'capture'
            context = {'release_tag': 'v1.2.3', 'source_commit': 'a' * 40}
            for key in store.GROUPS:
                store.checkpoint_group(out, context, key, self.group(root, key))
            store.complete_checkpoint(out, 'v1.2.3', 'a' * 40)
            with self.assertRaises(ValueError):
                store.complete_checkpoint(out, 'v1.2.4', 'b' * 40)
            # A manifest cannot authorize traversal even if a target is an existing PNG.
            data = json.loads((out / 'checkpoint.json').read_text())
            data['groups']['phone-en'][0]['file'] = '../outside.png'
            store.save(out / 'checkpoint.json', data)
            with self.assertRaises(ValueError):
                store.load_checkpoint(out)

    def test_duplicate_or_wrong_orientation_does_not_complete_a_group(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            raw = self.group(root, 'phone-en')
            (raw / '01-radar.png').write_bytes((raw / '00-demo-entry.png').read_bytes())
            with self.assertRaises(ValueError):
                store.checkpoint_group(root / 'out', {}, 'phone-en', raw)
            self.assertFalse((root / 'out/checkpoint.json').exists())

    def test_render_uses_current_renderer_and_released_widget_without_capture_or_build(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            captures, source, renderer, output = [root / x for x in ('captures', 'released', 'renderer', 'rendered')]
            renderer.mkdir()
            for name in store.RENDER_FILES:
                (renderer / name).write_text('current renderer')
            captions = {key.replace('phone-', ''): [{'file': n} for n in names[:4 if key.startswith('ipad') else 7]] for key, names in store.GROUPS.items()}
            (renderer / 'shot-captions.json').write_text(json.dumps(captions))
            context = {'release_tag': 'v1.2.3', 'source_commit': 'a' * 40}
            for key in store.GROUPS:
                store.checkpoint_group(captures, context, key, self.group(root, key))
            calls = []
            def render(args, **kwargs):
                calls.append((args, kwargs))
                if 'frame-shots.mjs' in args[1]:
                    folder = Path(args[args.index('--out') + 1])
                    ipad = args[args.index('--slot') + 1] == 'ipad'
                    for i in range(4 if ipad else 7):
                        png(folder / (('ipad-' if ipad else '') + f'{i+1:02}.png'), [2752, 2064] if ipad else [1320, 2868], str(i))
                elif 'frame-creative.mjs' in args[1]:
                    png(Path(args[args.index('--out') + 1]) / 'creative-16x9.png', [5244, 2950], 'creative')
            with patch.object(store.subprocess, 'run', side_effect=render):
                store.render(source, renderer, captures, output, 'v1.2.3', 'a' * 40, 'b' * 40)
            manifest = json.loads((output / 'manifest.json').read_text())
            self.assertEqual(manifest['render_commit'], 'b' * 40)
            self.assertEqual(manifest['source_commit'], 'a' * 40)
            self.assertEqual(manifest['render_files']['shot-captions.json'], store.digest(renderer / 'shot-captions.json'))
            self.assertTrue(all(args[0] == 'node' for args, _ in calls))
            self.assertEqual(calls[0][1]['cwd'], source)
            self.assertEqual(calls[1][0][1], str(renderer / 'frame-shots.mjs'))


if __name__ == '__main__':
    unittest.main()
