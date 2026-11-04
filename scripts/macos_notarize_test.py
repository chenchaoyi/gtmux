"""Accepted status, narrow retries and mandatory final validation, without Apple IO."""
import importlib.util
from pathlib import Path
import subprocess
import unittest
from unittest.mock import Mock

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('notarize', ROOT / 'macapp/notarize.py')
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


def response(code=0, text='', error=''):
    return subprocess.CompletedProcess([], code, text, error)


ACCEPTED = response(text='{"status":"Accepted"}')
LOST = response(68, error='Error Domain=NSURLErrorDomain Code=-1005 "The network connection was lost."')


class NotarizeTests(unittest.TestCase):
    def invoke(self, results, env=None):
        self.run = Mock(side_effect=results)
        self.pause = Mock()
        module.notarize(Path('signed.zip'), Path('Gtmux.app'), env or {'GTMUX_NOTARY_PROFILE':'synthetic profile'}, self.run, self.pause)

    def test_success_submits_once_and_validates(self):
        self.invoke([ACCEPTED, response(), response()])
        calls = [x.args[0] for x in self.run.call_args_list]
        self.assertEqual(calls[0], ['xcrun','notarytool','submit','signed.zip','--keychain-profile','synthetic profile','--wait','--output-format','json'])
        self.assertEqual(calls[-1], ['xcrun','stapler','validate','Gtmux.app'])
        self.pause.assert_not_called()

    def test_transport_loss_retries_only_ticket_not_submission(self):
        self.invoke([ACCEPTED, LOST, LOST, response(), response()])
        calls = [x.args[0] for x in self.run.call_args_list]
        self.assertEqual(sum('notarytool' in x for x in calls), 1)
        self.assertEqual(sum('staple' in x for x in calls), 3)
        self.assertEqual([x.args for x in self.pause.call_args_list], [(2,), (5,)])

    def test_api_key_arguments_preserve_paths_without_shell_splitting(self):
        self.invoke([ACCEPTED,response(),response()], {
            'GTMUX_NOTARY_KEY':'/synthetic signing/key.p8',
            'GTMUX_NOTARY_KEY_ID':'synthetic-key',
            'GTMUX_NOTARY_ISSUER':'synthetic-issuer',
        })
        self.assertEqual(self.run.call_args_list[0].args[0], [
            'xcrun','notarytool','submit','signed.zip','--key','/synthetic signing/key.p8',
            '--key-id','synthetic-key','--issuer','synthetic-issuer','--wait','--output-format','json',
        ])

    def test_exhausted_transport_stops_without_validation_or_publication(self):
        with self.assertRaises(ValueError):
            self.invoke([ACCEPTED, LOST, LOST, LOST])
        self.assertEqual(self.run.call_count, 4)

    def test_rejection_bad_response_or_submission_failure_never_retries(self):
        for result in (response(text='{"status":"Invalid"}'),response(text='{}'),response(text='bad JSON'),response(1)):
            with self.subTest(result=result), self.assertRaises(ValueError):
                self.invoke([result])
            self.assertEqual(self.run.call_count, 1)
            self.pause.assert_not_called()

    def test_permanent_staple_failure_and_other_codes_never_retry(self):
        for result in (response(68, error='No ticket for this signature'),response(65),response(1, error=LOST.stderr)):
            with self.subTest(result=result), self.assertRaises(ValueError):
                self.invoke([ACCEPTED,result])
            self.assertEqual(self.run.call_count, 2)
            self.pause.assert_not_called()

    def test_validation_failure_stays_failure(self):
        with self.assertRaises(ValueError):
            self.invoke([ACCEPTED,response(),response(65)])
        self.assertEqual(self.run.call_count, 3)

    def test_missing_credentials_fails_before_external_process(self):
        run = Mock()
        with self.assertRaises(ValueError):
            module.notarize(Path('signed.zip'),Path('Gtmux.app'),{},run)
        run.assert_not_called()

    def test_native_timeout_is_bounded_and_not_silently_retried(self):
        with self.assertRaises(subprocess.TimeoutExpired):
            self.invoke([ACCEPTED,subprocess.TimeoutExpired('stapler',90)])
        self.assertEqual(self.run.call_count,2)
        self.pause.assert_not_called()


if __name__ == '__main__':
    unittest.main()
