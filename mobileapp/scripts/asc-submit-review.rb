# Submit the App Store version for review, after the runbook's read-back checks.
#
#   bundle exec ruby scripts/asc-submit-review.rb <version> <build>
#
# Refuses unless that version carries that build. Adds the version to an open review
# submission (creating one if none is open) and submits it, retrying the App Store
# Connect 500s that both calls return intermittently: on 2026-09-17 adding the version
# failed with a bare "Server error got 500" once and succeeded on the next try a minute
# later. Once added, the version's state is READY_FOR_REVIEW and it no longer counts as
# the "editable" version, so it is looked up by version string, not by that.
require 'spaceship'

version_string, build_number = ARGV
abort 'usage: asc-submit-review.rb <version> <build>' unless version_string && build_number

Spaceship::ConnectAPI.token = Spaceship::ConnectAPI::Token.create(
  key_id: ENV.fetch('ASC_KEY_ID'),
  issuer_id: ENV['ASC_ISSUER_ID'],
  filepath: File.expand_path(ENV.fetch('ASC_KEY_PATH'))
)
app = Spaceship::ConnectAPI::App.find('com.gtmux.app')
platform = Spaceship::ConnectAPI::Platform::IOS

def with_retries(what, tries: 4, pause: 45)
  tries.times do |i|
    begin
      return yield
    rescue Spaceship::InternalServerError
      puts "#{what}: App Store Connect answered 500 (attempt #{i + 1} of #{tries})"
      raise if i == tries - 1
      sleep pause
    end
  end
end

find_version = lambda do
  app.get_app_store_versions(filter: {versionString: version_string, platform: platform}, includes: 'build').first
end
v = find_version.call
abort "no version #{version_string}" unless v
abort "refusing: #{version_string} carries build #{v.build&.version.inspect}, not #{build_number}" unless v.build&.version == build_number
puts "version #{version_string} build #{build_number} state=#{v.app_store_state}"

sub = app.get_review_submissions(filter: {state: 'READY_FOR_REVIEW', platform: platform}).first ||
      app.create_review_submission(platform: platform)
if v.app_store_state == 'PREPARE_FOR_SUBMISSION'
  with_retries('adding the version') { sub.add_app_store_version_to_review_items(app_store_version_id: v.id) }
end
with_retries('submitting') { sub.submit_for_review }
sleep 5
puts "version #{version_string} state=#{find_version.call.app_store_state}"
