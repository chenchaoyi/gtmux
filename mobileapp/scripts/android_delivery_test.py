import importlib.util
from pathlib import Path
import struct
import unittest
import tempfile
import zipfile
import json
from types import SimpleNamespace
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("android_delivery", Path(__file__).with_name("android-delivery.py"))
delivery = importlib.util.module_from_spec(spec)
spec.loader.exec_module(delivery)


class AndroidDeliveryTests(unittest.TestCase):
    def test_versions_are_monotonic_and_unambiguous(self):
        self.assertEqual(delivery.version_code("1.0.168", 1), 100168001)
        self.assertLess(delivery.version_code("1.0.168", 999), delivery.version_code("1.0.169", 1))
        self.assertLess(delivery.version_code("1.0.999", 999), delivery.version_code("1.1.0", 1))
        for version, number in [("1.0.1000", 1), ("1.100.0", 1), ("21.0.0", 1), ("1.0.168", 0), ("1.0.168", 1000), ("unknown", 1)]:
            with self.assertRaises(delivery.DeliveryError):
                delivery.version_code(version, number)

    def elf(self, alignment, offset=0, address=0):
        data = bytearray(120)
        data[:6] = b"\x7fELF\x02\x01"
        struct.pack_into("<Q", data, 32, 64)
        struct.pack_into("<HH", data, 54, 56, 1)
        struct.pack_into("<IIQQQQQQ", data, 64, 1, 5, offset, address, 0, 64, 64, alignment)
        return data

    def test_elf_requires_real_16kb_load_alignment(self):
        self.assertEqual(delivery.elf_alignment(self.elf(16384)), 1)
        self.assertEqual(delivery.elf_alignment(self.elf(65536)), 1)
        for value in [self.elf(4096), self.elf(16385), self.elf(16384, offset=1), b"not an ELF"]:
            with self.assertRaises(delivery.DeliveryError):
                delivery.elf_alignment(value)

    def test_package_inventory_rejects_missing_abis_path_confusion_and_secrets(self):
        with tempfile.TemporaryDirectory() as directory:
            target=Path(directory)/"synthetic.apk"
            def package(extra=None, arm_only=False):
                with zipfile.ZipFile(target,"w") as z:
                    z.writestr("assets/index.android.bundle",b"synthetic-js"*10000)
                    z.writestr("lib/arm64-v8a/libsynthetic.so",self.elf(16384))
                    if not arm_only:z.writestr("lib/x86_64/libsynthetic.so",self.elf(16384))
                    if extra:z.writestr(extra,b"synthetic")
            package();self.assertEqual(len(delivery.inspect_zip(target,"")["native_libraries"]),2)
            package(arm_only=True)
            with self.assertRaises(delivery.DeliveryError):delivery.inspect_zip(target,"")
            for name in ["../outside","/absolute","assets/service-account.json","assets/signing.p12"]:
                package(name)
                with self.assertRaises(delivery.DeliveryError):delivery.inspect_zip(target,"")

    def test_local_signing_refuses_incomplete_or_altered_delivery_before_using_a_key(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory); input=root/"input";output=root/"output";input.mkdir();output.mkdir()
            args=SimpleNamespace(input=input,output=output,source="a"*40,sdk="unused")
            for value in [[],{}, {"signed":False,"source":"a"*40,"files":[]}]:
                (input/"DELIVERY.json").write_text(json.dumps(value))
                with patch.object(delivery,"run") as tool:
                    with self.assertRaises(delivery.DeliveryError):delivery.sign(args)
                    tool.assert_not_called()


if __name__ == "__main__":
    unittest.main()
