#!/usr/bin/env ruby
# frozen_string_literal: true
#
# Attach a processed build to the version being prepared in App Store Connect.
#
# WHY: `fastlane release` uploads the binary and `fastlane metadata` pushes the listing,
# but NEITHER points the version at the build you just uploaded — deliver leaves whatever
# was already selected. On 2026-09-10 that meant version 1.0.12 sat carrying build 14
# while build 15, the one with the release's actual work, was processed and idle beside
# it. Nothing in the pipeline says so; you find out by looking.
#
# Usage (from mobileapp/, ASC key in the env):
#   eval "$(grep -E '^export ASC_(KEY_ID|ISSUER_ID|KEY_PATH)=' ~/.zshrc)"
#   bundle exec ruby scripts/asc-attach-build.rb           # attach the newest VALID build
#   bundle exec ruby scripts/asc-attach-build.rb 15        # attach a specific one
#   bundle exec ruby scripts/asc-attach-build.rb --list    # show, change nothing
#
# Run it AFTER `fastlane release` has finished processing, before you Submit.

require 'spaceship'

APP_ID = 'com.gtmux.app'
list_only = ARGV.include?('--list')
want = ARGV.find { |a| a =~ /\A\d+\z/ }

Spaceship::ConnectAPI.token = Spaceship::ConnectAPI::Token.create(
  key_id: ENV.fetch('ASC_KEY_ID'),
  issuer_id: ENV['ASC_ISSUER_ID'],
  filepath: File.expand_path(ENV.fetch('ASC_KEY_PATH'))
)

app = Spaceship::ConnectAPI::App.find(APP_ID)
version = app.get_edit_app_store_version
abort 'no editable version — nothing is being prepared' unless version
puts "editable version: #{version.version_string} (#{version.app_store_state})"
puts "attached build:   #{version.build&.version || '(none)'}"

builds = app.get_builds(filter: {'preReleaseVersion.version' => version.version_string}, limit: 20)
builds.each { |b| puts "  build #{b.version}  processing=#{b.processing_state}" }

# How far behind the STORE is — which decides what "What's New" has to cover.
#
# Most device builds are never submitted, so the live version can sit many releases back:
# on 2026-09-11 the store showed 0.68.0 while 1.0.14 was being prepared, 36 archived
# versions later. The release notes had been written for the last three, because nothing
# in the pipeline ever put the span in front of anyone. Now it does.
live = app.get_live_app_store_version
if live
  puts "live on the store:  #{live.version_string}"
  notes = File.expand_path('../release-notes', __dir__)
  span = Dir.glob(File.join(notes, '*.en.txt')).map { |f| File.basename(f, '.en.txt') }
             .select { |v| Gem::Version.new(v) > Gem::Version.new(live.version_string) rescue false }
             .sort_by { |v| Gem::Version.new(v) }
  if span.length > 1
    puts
    puts "  !! the store is #{span.length} versions behind (#{span.first} … #{span.last})."
    puts "     \"What's New\" is read by someone deciding whether to update from #{live.version_string},"
    puts "     so it covers ALL of it — the capabilities they would notice, then one line for the rest."
    puts "     Write that into fastlane/metadata/*/release_notes.txt and do NOT re-run"
    puts "     set-version.sh: the archive keeps one entry per version for the in-app popup."
    puts "     See release-notes/README.md \"When a submission crosses several versions\"."
  end
end

if list_only
  puts '== list only =='
  exit 0
end

valid = builds.select { |b| b.processing_state == 'VALID' }
abort 'no processed build to attach yet — wait for ASC to finish processing' if valid.empty?
# Newest by build number, which is how the uploader numbers them.
target = want ? valid.find { |b| b.version == want } : valid.max_by { |b| b.version.to_i }
abort "build #{want} is not a processed build of #{version.version_string}" unless target

if version.build&.id == target.id
  puts "build #{target.version} is already attached — nothing to do"
  exit 0
end

version.select_build(build_id: target.id)
puts "attached build #{target.version} to #{version.version_string}"
