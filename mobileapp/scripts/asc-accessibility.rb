#!/usr/bin/env ruby
# frozen_string_literal: true
require 'json'
require 'digest'
require 'time'
require 'uri'

# Review evidence stays private. This helper only GETs ASC; it cannot publish labels.
class StoreAccessibility
  BUNDLE = 'com.gtmux.app'
  FAMILIES = %w[IPHONE IPAD].freeze
  FEATURES = %w[supportsVoiceover supportsVoiceControl supportsLargerText supportsDarkInterface
                supportsDifferentiateWithoutColorAlone supportsSufficientContrast supportsReducedMotion
                supportsCaptions supportsAudioDescriptions].freeze
  TASKS = %w[first_launch_pairing radar conversation_terminal replies_choices hq
             servers_settings errors_sheets notifications_live_activity].freeze

  def initialize(api)
    @api = api
  end

  def rows(path)
    result, seen = [], []
    while path
      uri = URI(path)
      unless (uri.relative? && path.start_with?('/v1/')) ||
             (uri.scheme == 'https' && uri.host == 'api.appstoreconnect.apple.com' && uri.port == 443 && uri.userinfo.nil?)
        raise 'Unexpected accessibility pagination origin'
      end
      raise 'Accessibility pagination repeated or exceeded 20 pages' if seen.include?(path) || seen.length >= 20
      seen << path
      page = @api.call('get', path)
      raise 'Malformed accessibility API list' unless page['data'].is_a?(Array)
      result.concat(page['data'])
      path = page.dig('links', 'next')
      raise 'Malformed accessibility pagination link' unless path.nil? || (path.is_a?(String) && !path.empty?)
    end
    result
  end

  def snapshot
    apps = rows('/v1/apps?filter[bundleId]=com.gtmux.app&limit=2')
    raise 'Expected exactly one gtmux ASC app' unless apps.length == 1 && apps.first.dig('attributes', 'bundleId') == BUNDLE
    id = apps.first.fetch('id')
    versions = rows("/v1/apps/#{id}/appStoreVersions?filter[platform]=IOS&limit=200")
    live = versions.select do |v|
      state = v.dig('attributes', 'appVersionState') || v.dig('attributes', 'appStoreState')
      %w[READY_FOR_SALE READY_FOR_DISTRIBUTION].include?(state)
    end
    raise 'No released iOS Store version; review the first release manually' if live.empty?
    # ASC retains READY_* on historical releases too. Choose the numerically latest
    # released version, never the first API row or a newer unreleased candidate.
    live.each do |v|
      raise 'Unexpected Store version format; review availability manually' unless v.dig('attributes', 'versionString').to_s.match?(/\A\d+(?:\.\d+){0,2}\z/)
    end
    version_key = ->(v) { parts = v['attributes']['versionString'].split('.').map(&:to_i); parts + [0] * (3 - parts.length) }
    latest = live.max_by(&version_key)
    raise 'Multiple released builds share the latest Store version; review availability manually' unless live.count { |v| version_key.call(v) == version_key.call(latest) } == 1
    build = @api.call('get', "/v1/appStoreVersions/#{latest.fetch('id')}/build")['data']
    raise 'Live Store build is missing or not VALID' unless build&.dig('attributes', 'processingState') == 'VALID'
    published = FAMILIES.to_h { |family| [family, nil] }
    rows("/v1/apps/#{id}/accessibilityDeclarations?filter[state]=PUBLISHED&limit=200").each do |row|
      attributes = row.fetch('attributes')
      next unless attributes['state'] == 'PUBLISHED' && FAMILIES.include?(attributes['deviceFamily'])
      family = attributes['deviceFamily']
      raise "Multiple published accessibility declarations for #{family}" unless published[family].nil?
      unknown = attributes.keys.grep(/^supports/) - FEATURES
      raise 'Apple added accessibility fields; update the review helper before continuing' unless unknown.empty?
      values = FEATURES.to_h { |feature| [feature, attributes[feature]] }
      raise "Incomplete published capability values for #{family}" unless values.values.all? { |value| boolean?(value) }
      published[family] = values
    end
    {'app_id' => id, 'live' => {'version' => latest.dig('attributes', 'versionString'), 'build' => build.dig('attributes', 'version')},
     'published' => published}
  end

  def candidate(receipt, version, build)
    package = JSON.parse(File.read(receipt))
    unless package['schema'] == 1 && package['upload_allowed'] == true && package['version'] == version && package['build'] == build
      raise 'Package receipt does not match the candidate version/build or allow upload'
    end
    {'version' => version, 'build' => build, 'package_receipt_sha256' => Digest::SHA256.file(receipt).hexdigest}
  end

  def template(receipt:, version:, build:)
    identity = candidate(receipt, version, build)
    {'schema' => 1, 'bundle_id' => BUNDLE, 'candidate' => identity, 'baseline' => snapshot,
     'reviewed_by' => '', 'reviewed_at' => '', 'change_review' => '', 'url_review' => '',
     'families' => FAMILIES.to_h do |family|
       [family, {'environment' => '', 'limitations' => '', 'common_tasks' => TASKS.to_h { |task| [task, ''] },
                 'capabilities' => FEATURES.to_h { |feature| [feature, {'supported' => nil, 'evidence' => ''}] }}]
     end,
     'after_release' => {'owner' => '', 'plan' => ''}}
  end

  def boolean?(value) = value == true || value == false
  def text?(value) = value.is_a?(String) && !value.strip.empty?

  def review(path, receipt:, version:, build:)
    raise 'Accessibility review is missing; run asc-accessibility.rb init and complete it' unless path && File.file?(path)
    raise 'Accessibility review exceeds 1 MiB' if File.size(path) > 1024 * 1024
    data = JSON.parse(File.read(path))
    unless data.is_a?(Hash) && data['schema'] == 1 && data['bundle_id'] == BUNDLE && data['candidate'] == candidate(receipt, version, build)
      raise 'Accessibility review is for a different package/version/build; create a fresh review'
    end
    %w[reviewed_by reviewed_at change_review url_review].each do |key|
      raise "Complete accessibility #{key}" unless text?(data[key])
    end
    raise 'Accessibility reviewed_at is in the future' if Time.iso8601(data['reviewed_at']) > Time.now + 300
    raise 'Review both IPHONE and IPAD separately' unless data['families'].is_a?(Hash) && data['families'].keys.sort == FAMILIES.sort
    FAMILIES.each do |family|
      entry = data['families'][family]
      raise "Complete #{family} environment and limitations" unless entry.is_a?(Hash) && %w[environment limitations].all? { |key| text?(entry[key]) }
      raise "Complete #{family} common-task evidence" unless entry['common_tasks'].is_a?(Hash) && TASKS.all? { |task| text?(entry['common_tasks'][task]) }
      capabilities = entry['capabilities']
      raise "Review all #{family} capabilities" unless capabilities.is_a?(Hash) && capabilities.keys.sort == FEATURES.sort
      FEATURES.each do |feature|
        decision = capabilities[feature]
        unless decision.is_a?(Hash) && boolean?(decision['supported']) && text?(decision['evidence'])
          raise "Complete #{family} #{feature}: boolean supported plus evidence or reason for not claiming support"
        end
      end
    end
    baseline = data['baseline']
    unless baseline.is_a?(Hash) && text?(baseline['app_id']) && baseline['live'].is_a?(Hash) &&
           %w[version build].all? { |key| text?(baseline['live'][key]) } &&
           baseline['published'].is_a?(Hash) && baseline['published'].keys.sort == FAMILIES.sort &&
           baseline['published'].values.all? { |values| values.nil? || (values.is_a?(Hash) && values.keys.sort == FEATURES.sort && values.values.all? { |v| boolean?(v) }) }
      raise 'Accessibility baseline is missing or malformed; create a fresh review'
    end
    data
  end

  def check(path, receipt:, version:, build:, live: false)
    data = review(path, receipt: receipt, version: version, build: build)
    current = snapshot
    errors, additions = [], []
    if live
      errors << 'Candidate is not the exact live Store version/build yet' unless current['live'] == {'version' => version, 'build' => build}
      errors << 'ASC app identity changed' unless current['app_id'] == data.dig('baseline', 'app_id')
    else
      errors << 'Live version/build or published declarations changed; create and complete a fresh review' unless current == data['baseline']
    end
    FAMILIES.each do |family|
      actual = current['published'][family]
      expected = data['families'][family]['capabilities'].transform_values { |decision| decision['supported'] }
      if live
        errors << "#{family} published declarations differ; publish verified live support in ASC, then rerun" unless actual == expected
      else
        FEATURES.each do |feature|
          errors << "#{family} #{feature} regresses an existing claim; resolve and reassess before submission" if actual&.[](feature) == true && !expected[feature]
          additions << "#{family}.#{feature}" if expected[feature] && actual&.[](feature) != true
        end
      end
    end
    if !live && !additions.empty? && !%w[owner plan].all? { |key| text?(data.dig('after_release', key)) }
      errors << 'New claims require an after_release owner and plan; publish only after the supported version is live'
    end
    {ok: errors.empty?, errors: errors, post_live_claims: additions,
     review_sha256: Digest::SHA256.file(path).hexdigest, live: current['live'], phase: live ? 'live' : 'preparation'}
  rescue JSON::ParserError
    {ok: false, errors: ['Accessibility review, package receipt or ASC response is not valid JSON'], post_live_claims: [], phase: live ? 'live' : 'preparation'}
  rescue StandardError => e
    {ok: false, errors: [e.message], post_live_claims: [], phase: live ? 'live' : 'preparation'}
  end
end

if $PROGRAM_NAME == __FILE__
  require_relative 'asc-asset-library'
  require 'fileutils'
  if ARGV.empty? || ARGV.include?('--help')
    puts 'Usage: bundle exec ruby scripts/asc-accessibility.rb init|status|live-status --version X.Y.Z --build N --package-receipt FILE [--review build/store-accessibility-review.json]'
    puts 'Read-only ASC access. init creates an unapproved private review without overwriting it. status checks preparation; live-status checks exact live build and published claims. Never publishes or submits.'
    exit 0
  end
  abort 'Use init, status or live-status' unless %w[init status live-status].include?(ARGV.first)
  inputs = %w[version build package-receipt].to_h { |key| [key, opt(key) || abort("--#{key} is required")] }
  helper = StoreAccessibility.new(method(:call))
  path = opt('review') || 'build/store-accessibility-review.json'
  args = {receipt: inputs['package-receipt'], version: inputs['version'], build: inputs['build']}
  if ARGV.first == 'init'
    abort 'Review already exists; use a new path for a new assessment' if File.exist?(path) || File.symlink?(path)
    data = helper.template(**args)
    FileUtils.mkdir_p(File.dirname(File.expand_path(path)), mode: 0o700)
    File.open(path, File::WRONLY | File::CREAT | File::EXCL | File::NOFOLLOW, 0o600) { |stream| stream.puts(JSON.pretty_generate(data)) }
    puts 'Unapproved private accessibility review created. Complete both device families before checking readiness.'
  else
    result = helper.check(path, **args, live: ARGV.first == 'live-status')
    puts JSON.pretty_generate(result)
    exit(result[:ok] ? 0 : 1)
  end
end
