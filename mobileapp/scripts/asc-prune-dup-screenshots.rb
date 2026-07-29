#!/usr/bin/env ruby
# frozen_string_literal: true
#
# Prune DUPLICATE App Store screenshots left by `fastlane metadata`/deliver.
#
# WHY: deliver uploads all screenshots, then verifies each is present — but ASC's
# media processing lags, so some come back "missing on App Store Connect", deliver
# re-uploads them, and ASC ends up with duplicates (e.g. 05.png twice). It happens on
# essentially every run, even with `overwrite_screenshots: true`. This keeps the FIRST
# occurrence of each file name (they're already in 01..NN order) and deletes the rest,
# so each locale ends up with exactly one of each — no re-upload, no re-dup.
#
# Usage (from mobileapp/, ASC key in the env — see docs/appstore/submit.md):
#   eval "$(grep -E '^export ASC_(KEY_ID|ISSUER_ID|KEY_PATH)=' ~/.zshrc)"
#   bundle exec ruby scripts/asc-prune-dup-screenshots.rb          # prune
#   bundle exec ruby scripts/asc-prune-dup-screenshots.rb --list   # list only, no delete
#
# Run it AFTER `fastlane metadata`, before you Submit in App Store Connect.

require 'spaceship'

APP_ID = '6791144062' # gtmux — com.gtmux.app
LOCALES = %w[en-US zh-Hans].freeze
list_only = ARGV.include?('--list')

Spaceship::ConnectAPI.token = Spaceship::ConnectAPI::Token.create(
  key_id: ENV.fetch('ASC_KEY_ID'),
  issuer_id: ENV.fetch('ASC_ISSUER_ID'),
  filepath: ENV.fetch('ASC_KEY_PATH'),
)

app = Spaceship::ConnectAPI::App.get(app_id: APP_ID)
version = app.get_edit_app_store_version(platform: Spaceship::ConnectAPI::Platform::IOS)
puts "editable version: #{version.version_string} (#{version.app_store_state})"

version.get_app_store_version_localizations.each do |loc|
  next unless LOCALES.include?(loc.locale)

  loc.get_app_screenshot_sets.each do |set|
    shots = set.app_screenshots
    names = shots.map(&:file_name)
    puts "#{loc.locale} / #{set.screenshot_display_type}: #{shots.size} -> #{names.join(', ')}"
    next if list_only

    seen = {}
    shots.each do |s|
      if seen[s.file_name]
        puts "  delete dup: #{s.file_name} (#{s.id})"
        s.delete!
      else
        seen[s.file_name] = true
      end
    end
  end
end

puts list_only ? '== list only ==' : '== pruning done =='
