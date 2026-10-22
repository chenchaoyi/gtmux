"""Exercise real Ruby helper branches without Apple credentials or network calls."""
from pathlib import Path
import subprocess
import unittest


class RubyPreparationTests(unittest.TestCase):
    def test_asc_progress_readback_and_upload_resume(self):
        result = subprocess.run(['ruby', str(Path(__file__).with_suffix('.rb'))], text=True, capture_output=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_fastlane_resolves_package_paths_before_upload_and_stops_when_gate_fails(self):
        script = r'''
require 'json'
$lanes = {}
$uploads = 0
$fail_gate = true
$verified_ipa = nil
$expected_baseline = nil
module UI
  def self.message(*) = nil
  def self.user_error!(message) = raise(message)
end
def default_platform(*) = nil
def platform(*) = yield
def desc(*) = nil
def lane(name, &block) = $lanes[name] = block
def app_store_connect_api_key(*) = {}
def latest_testflight_build_number(*) = 25
def build_app(*) = nil
def get_version_number(*) = '1.2.3'
def upload_to_testflight(**options)
  raise 'upload differs from verified IPA' unless options[:ipa] == $verified_ipa
  $uploads += 1
end
def sh(*args)
  script = args.fetch(1)
  raise 'gate script not found from fastlane working directory' unless File.file?(script)
  root = File.expand_path('../..', script)
  values = %w[--ipa --baseline --receipt].to_h { |flag| [flag, args.fetch(args.index(flag) + 1)] }
  raise 'package path depends on working directory' unless values.values.all? { |path| path.start_with?('/') }
  raise 'receipt not under mobile build directory' unless values['--receipt'] == File.join(root, 'build/store-package.json')
  raise 'baseline not resolved from mobile root' unless values['--baseline'] == File.join(root, $expected_baseline)
  $verified_ipa = values['--ipa']
  raise 'gate failed' if $fail_gate
end
ENV['ASC_KEY_ID'] = 'synthetic'
ENV['ASC_ISSUER_ID'] = 'synthetic'
ENV['ASC_KEY_PATH'] = 'synthetic'
load ARGV.fetch(0)
[:release, :upload].each do |name|
  $expected_baseline = name == :release ? 'build/GtmuxMobile.xcarchive' : 'original.ipa'
  begin
    $lanes[name].call({baseline: 'original.ipa', version: '1.2.3', build: '26'})
    raise 'gate was not reached'
  rescue => error
    raise unless error.message == 'gate failed'
  end
end
raise 'uploaded despite gate failure' unless $uploads.zero?
$fail_gate = false
$lanes[:upload].call({ipa: 'candidate.ipa', baseline: 'original.ipa', version: '1.2.3', build: '26'})
raise 'verified package not uploaded exactly once' unless $uploads == 1
raise 'relative IPA not resolved from mobile root' unless $verified_ipa == File.expand_path('../candidate.ipa', File.dirname(ARGV.fetch(0)))
'''
        fastfile = Path(__file__).resolve().parents[1] / 'fastlane/Fastfile'
        result = subprocess.run(['ruby', '-e', script, str(fastfile)], cwd=fastfile.parent, text=True, capture_output=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)


if __name__ == '__main__':
    unittest.main()
