#!/usr/bin/env ruby
# frozen_string_literal: true
# Independent readback and idempotent asset placement. Never submits a version.
require_relative 'asc-asset-library'
require 'fileutils'

METADATA_FIELDS = {'description' => 'description', 'keywords' => 'keywords', 'marketing_url' => 'marketingUrl',
                   'promotional_text' => 'promotionalText', 'release_notes' => 'whatsNew', 'support_url' => 'supportUrl'}.freeze
LOCALES = %w[en-US zh-Hans].freeze

def metadata_plan(dir)
  LOCALES.to_h do |locale|
    expected = METADATA_FIELDS.to_h do |file, field|
      path = File.join(dir, locale, file + '.txt')
      raise "Missing #{locale}/#{file}.txt" unless File.file?(path)
      [field, File.read(path).strip]
    end
    [locale, expected]
  end
end

def package_receipt_ok(receipt, ipa, version, build)
  receipt['schema'] == 1 && receipt['upload_allowed'] == true && receipt['version'] == version && receipt['build'] == build &&
    receipt['ipa_sha256'] == Digest::SHA256.file(ipa).hexdigest &&
    receipt.fetch('packages', []).map { |p| p['bundle'] }.sort == %w[com.gtmux.app com.gtmux.app.notificationservice com.gtmux.app.widget]
end

def preparation_status(version:, build:, ipa:, receipt:, state:, metadata_dir:, screenshots:)
  # Read local inputs before contacting Apple. Authentication uses explicit current
  # environment settings; it never reads a previous task's temporary configuration.
  FileUtils.mkdir_p(File.dirname(File.expand_path(state)), mode: 0o700)
  StoreProgress.save(state, {schema: 1, version: version, build: build, status: 'checking', submitted: false})
  expected = metadata_plan(metadata_dir)
  package = JSON.parse(File.read(receipt))
  checks = {'package' => package_receipt_ok(package, ipa, version, build)}
  v, locs = version_localizations(version)
  actual_build = call('get', "/v1/appStoreVersions/#{v.id}/build")['data']
  checks['build'] = v.app_store_state == 'PREPARE_FOR_SUBMISSION' && actual_build&.dig('attributes', 'version') == build && actual_build&.dig('attributes', 'processingState') == 'VALID'
  checks['metadata'] = LOCALES.map do |locale|
    next false unless locs[locale]
    attributes = call('get', "/v1/appStoreVersionLocalizations/#{locs[locale]}").dig('data', 'attributes') || {}
    expected[locale].all? { |field, value| attributes[field].to_s.strip == value }
  end.all?
  _, shots = shot_rows(version, screenshots)
  checks['screenshots'] = shots.length == 4 && shots.all? { |r| LOCALES.include?(r[:locale]) && r[:ok] && r[:weak].zero? && r[:files].length == (File.basename(r[:files].first).start_with?('ipad-') ? 4 : 7) && r[:placed].all? { |p| p[:placement].dig('attributes', 'state') == 'ACTIVE' } }
  _, creative, = creative_rows(library_id, version)
  checks['creative'] = creative.length == 4 && creative.all? { |r| r[:ok] && r[:placed].dig('attributes', 'state') == 'ACTIVE' }
  ready = checks.values.all?
  result = {schema: 1, version: version, build: build, checked_at: Time.now.utc.iso8601,
            status: ready ? 'ready_for_user_review' : 'incomplete', checks: checks,
            inputs: {ipa_sha256: Digest::SHA256.file(ipa).hexdigest, package_receipt_sha256: Digest::SHA256.file(receipt).hexdigest,
                     metadata_sha256: Digest::SHA256.hexdigest(JSON.generate(expected))},
            symbols: package['matching_symbols'] == true ? 'verified_against_archive' : 'unavailable_original_ipa_fallback',
            user_preview_required: true, submitted: false}
  FileUtils.mkdir_p(File.dirname(File.expand_path(state)), mode: 0o700)
  StoreProgress.save(state, result)
  checks.each { |name, ok| store_progress.event('readback', check: name, result: ok ? 'passed' : 'incomplete') }
  puts ready ? 'Ready for user review in App Store Connect. Preview both locales, then Submit manually.' : 'Preparation incomplete; see the failed checks above. Existing uploads can be reused.'
  ready
rescue StandardError, SystemExit
  StoreProgress.save(state, {schema: 1, version: version, build: build, status: 'incomplete', checks: checks || {}, submitted: false})
  raise
end

if $PROGRAM_NAME == __FILE__
  if ARGV.empty? || ARGV.include?('--help')
    puts 'Usage: bundle exec ruby scripts/asc-preparation.rb status|assets --version X.Y.Z --build N --ipa FILE --package-receipt FILE [--state PRIVATE.json] [--dir screenshots] [--metadata-dir metadata] [--journal PRIVATE.jsonl]'
    puts 'status independently reads package/build/text/screenshots/creative. assets places images idempotently, then performs the same readback. Neither command uploads a binary or submits a version.'
    exit 0
  end
  abort 'Use status or assets' unless %w[status assets].include?(ARGV.first)
  options = %w[version build ipa package-receipt].to_h { |name| [name, opt(name) || abort("--#{name} is required")] }
  abort 'Use a semantic version and numeric build' unless options['version'].match?(/\A\d+\.\d+\.\d+\z/) && options['build'].match?(/\A\d+(?:\.\d+){0,2}\z/)
  begin
    state = opt('state') || 'build/store-state.json'
    FileUtils.mkdir_p(File.dirname(File.expand_path(state)), mode: 0o700)
    StoreProgress.save(state, {schema: 1, version: options['version'], build: options['build'], status: 'preparing', submitted: false})
    store_progress.event('start', command: ARGV.first)
    if ARGV.first == 'assets'
      # All local prerequisites must be valid before any write to ASC.
      metadata_plan(opt('metadata-dir') || 'fastlane/metadata')
      abort 'Package receipt differs from the selected IPA/version/build' unless package_receipt_ok(JSON.parse(File.read(options['package-receipt'])), options['ipa'], options['version'], options['build'])
      place_screenshots(library_id, options['version'], opt('dir') || SHOT_DIR, false)
      place_creative(library_id, options['version'])
    end
    ok = preparation_status(version: options['version'], build: options['build'], ipa: options['ipa'], receipt: options['package-receipt'],
                            state: opt('state') || 'build/store-state.json', metadata_dir: opt('metadata-dir') || 'fastlane/metadata', screenshots: opt('dir') || SHOT_DIR)
    exit(ok ? 0 : 1)
  rescue StoreProgress::Pending => e
    StoreProgress.save(state, {schema: 1, version: options['version'], build: options['build'], status: 'pending', submitted: false})
    warn e.message
    exit 75
  end
end
