import datetime
import hashlib
from pathlib import Path
import plistlib
import tempfile
import unittest
import uuid
import zipfile

import store_package as store


def tlv(tag, value):
    length = len(value)
    count = (length.bit_length() + 7) // 8
    encoded = bytes([length]) if length < 128 else bytes([128 | count]) + length.to_bytes(count, 'big')
    return bytes([tag]) + encoded + value


def der(fields):
    pairs = []
    for key, value in fields.items():
        encoded = tlv(1, b'\xff' if value else b'\0') if isinstance(value, bool) else tlv(12, value.encode())
        pairs.append(tlv(48, tlv(12, key.encode()) + encoded))
    return tlv(112, tlv(2, b'\1') + tlv(176, b''.join(pairs)))


class StorePackageTests(unittest.TestCase):
    team = 'ABCDEFGHIJ'

    def ent(self, bundle):
        return {'application-identifier': self.team + '.' + bundle, 'com.apple.developer.team-identifier': self.team,
                'get-task-allow': False, **({'aps-environment': 'production'} if bundle == 'com.gtmux.app' else {})}

    def profile(self, bundle):
        return {'TeamIdentifier': [self.team], 'ExpirationDate': datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(days=10),
                'Entitlements': {**self.ent(bundle), 'beta-reports-active': True}}

    def test_entitlement_comparison_blocks_unwanted_keychain_groups_and_all_other_drift(self):
        bundle = 'com.gtmux.app'
        original = self.ent(bundle)
        good = {**original, 'beta-reports-active': True}
        store.check_entitlements(original, good, bundle, self.team)
        for change in ({'keychain-access-groups': 'unexpected'}, {'get-task-allow': True}, {'get-task-allow': 'false'}, {'aps-environment': 'development'}, {'com.apple.security.application-groups': 'new-group'}):
            with self.subTest(change=change), self.assertRaises(ValueError):
                store.check_entitlements(original, {**good, **change}, bundle, self.team)

    def test_store_profiles_reject_adhoc_enterprise_expired_wrong_identity(self):
        bundle = 'com.gtmux.app'
        good = self.profile(bundle)
        store.check_profile(good, bundle, self.team)
        for change in ({'ProvisionedDevices': ['a-device']}, {'ProvisionedDevices': []}, {'ProvisionsAllDevices': True}, {'TeamIdentifier': ['OTHERTEAM1']}, {'ExpirationDate': datetime.datetime(2000, 1, 1)}):
            with self.subTest(change=change), self.assertRaises(ValueError):
                store.check_profile({**good, **change}, bundle, self.team)

    def ipa(self, root, name, build='26', resource='same JS'):
        path = root / name
        with zipfile.ZipFile(path, 'w') as output:
            for index, bundle in enumerate(sorted(store.BUNDLES)):
                base = 'Payload/gtmux.app' + ('' if bundle == 'com.gtmux.app' else '/PlugIns/' + bundle + '.appex')
                info = {'CFBundleIdentifier': bundle, 'CFBundleExecutable': 'executable', 'CFBundleVersion': build, 'CFBundleShortVersionString': '1.2.3'}
                output.writestr(base + '/Info.plist', plistlib.dumps(info))
                output.writestr(base + '/executable', ('native-code:' + bundle).encode())
                output.writestr(base + '/embedded.mobileprovision', b'profile')
            output.writestr('Payload/gtmux.app/main.jsbundle', resource)
        return path

    def fake_command(self, change=None, development_archive=False):
        def run(args):
            if args[0] == 'codesign' and '--entitlements' in args:
                path = Path(args[-1])
                bundle = plistlib.loads((path / 'Info.plist').read_bytes())['CFBundleIdentifier']
                fields = self.ent(bundle)
                if development_archive and any(part.endswith('.xcarchive') for part in path.parts):
                    fields['get-task-allow'] = True
                    if bundle == 'com.gtmux.app':
                        fields['aps-environment'] = 'development'
                if 'candidate' in path.parts:
                    fields['beta-reports-active'] = True
                    fields.update(change or {})
                return der(fields)
            if args[0] == 'security':
                path = Path(args[-1]).parent
                bundle = plistlib.loads((path / 'Info.plist').read_bytes())['CFBundleIdentifier']
                return plistlib.dumps(self.profile(bundle))
            if args[0] == 'dwarfdump':
                native = uuid.UUID(hashlib.md5(Path(args[-1]).read_bytes()).hexdigest())
                return f'UUID: {native} (arm64) binary'.encode()
            return b''
        return run

    def test_full_original_ipa_verification_checks_all_extensions_and_resources(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            original = self.ipa(root, 'original.ipa', build='24')
            candidate = self.ipa(root, 'candidate.ipa')
            receipt = store.verify(candidate, original, '1.2.3', '26', self.team, self.fake_command())
            self.assertEqual(len(receipt['packages']), 3)
            self.assertTrue(receipt['upload_allowed'])
            self.assertFalse(receipt['matching_symbols'])
            with self.assertRaisesRegex(ValueError, 'entitlement drift'):
                store.verify(candidate, original, '1.2.3', '26', self.team, self.fake_command({'keychain-access-groups': 'unexpected'}))
            with self.assertRaisesRegex(ValueError, 'version or build'):
                store.verify(candidate, original, '1.2.3', '27', self.team, self.fake_command())
            altered = self.ipa(root, 'altered.ipa', resource='changed JS')
            with self.assertRaisesRegex(ValueError, 'resources'):
                store.verify(altered, original, '1.2.3', '26', self.team, self.fake_command())

    def test_archive_requires_matching_symbols_and_preserves_original(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            original = self.ipa(root, 'original.ipa')
            candidate = self.ipa(root, 'candidate.ipa')
            archive = root / 'canonical.xcarchive'
            store.unpack(original, archive)
            # IPA layout -> canonical Xcode archive layout.
            (archive / 'Products/Applications').mkdir(parents=True)
            (archive / 'Payload/gtmux.app').rename(archive / 'Products/Applications/gtmux.app')
            with self.assertRaisesRegex(ValueError, 'matching.*dSYM'):
                store.verify(candidate, archive, '1.2.3', '26', self.team, self.fake_command())
            for bundle in store.BUNDLES:
                symbol = archive / f'dSYMs/{bundle}.dSYM/Contents/Resources/DWARF/executable'
                symbol.parent.mkdir(parents=True)
                symbol.write_bytes(('native-code:' + bundle).encode())
            before = hashlib.sha256(candidate.read_bytes()).hexdigest()
            receipt = store.verify(candidate, archive, '1.2.3', '26', self.team, self.fake_command())
            self.assertTrue(receipt['matching_symbols'])
            development = store.verify(candidate, archive, '1.2.3', '26', self.team, self.fake_command(development_archive=True))
            self.assertTrue(development['upload_allowed'])
            with self.assertRaisesRegex(ValueError, 'entitlement drift'):
                store.verify(candidate, archive, '1.2.3', '26', self.team, self.fake_command({'keychain-access-groups': 'unexpected'}, development_archive=True))
            self.assertEqual(before, hashlib.sha256(candidate.read_bytes()).hexdigest())

    def test_unsafe_zip_fails_before_extraction(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            ipa = root / 'unsafe.ipa'
            with zipfile.ZipFile(ipa, 'w') as output:
                output.writestr('../escape', 'bad')
            with self.assertRaises(ValueError):
                store.unpack(ipa, root / 'out')
            self.assertFalse((root / 'escape').exists())


if __name__ == '__main__':
    unittest.main()
