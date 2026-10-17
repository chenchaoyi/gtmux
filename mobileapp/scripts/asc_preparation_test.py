"""Exercise real Ruby helper branches without Apple credentials or network calls."""
from pathlib import Path
import subprocess
import unittest


class RubyPreparationTests(unittest.TestCase):
    def test_asc_progress_readback_and_upload_resume(self):
        result = subprocess.run(['ruby', str(Path(__file__).with_suffix('.rb'))], text=True, capture_output=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_fastlane_upload_is_not_reached_when_package_gate_fails(self):
        script = r'''
require 'json'
$lanes = {}
$uploads = 0
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
def upload_to_testflight(*) = $uploads += 1
def sh(*args)
  raise 'gate failed' if args.include?('scripts/store_package.py')
end
ENV['ASC_KEY_ID'] = 'synthetic'
ENV['ASC_ISSUER_ID'] = 'synthetic'
ENV['ASC_KEY_PATH'] = 'synthetic'
load ARGV.fetch(0)
[:release, :upload].each do |name|
  begin
    $lanes[name].call({baseline: 'original.ipa', version: '1.2.3', build: '26'})
    raise 'gate was not reached'
  rescue => error
    raise unless error.message == 'gate failed'
  end
end
raise 'uploaded despite gate failure' unless $uploads.zero?
'''
        fastfile = Path(__file__).resolve().parents[1] / 'fastlane/Fastfile'
        result = subprocess.run(['ruby', '-e', script, str(fastfile)], text=True, capture_output=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)


if __name__ == '__main__':
    unittest.main()
