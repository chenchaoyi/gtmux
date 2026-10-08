#!/usr/bin/env ruby
# frozen_string_literal: true
#
# The App Asset Library, from the command line: what App Store Connect API 4.5.1 added and
# what replaces the screenshot and preview resources fastlane deliver uses, which that
# release deprecated (appScreenshotSets, appScreenshots, appPreviewSets, appPreviews).
#
# An asset is uploaded once into the app's library and placed on surfaces (a version
# localization, a custom product page, an experiment treatment). An asset has no language;
# the localization it is placed on does. Placements travel through review with their
# parent surface, so they need one that is still editable; an asset can also be submitted
# on its own from the Asset Library page in App Store Connect.
#
# Usage (from mobileapp/, ASC key in the env, e.g. `. ../ops/asc-env.sh`):
#   bundle exec ruby scripts/asc-asset-library.rb list [--category CREATIVE_ASSETS]
#   bundle exec ruby scripts/asc-asset-library.rb upload FILE --category CREATIVE_ASSETS --name "…"
#   bundle exec ruby scripts/asc-asset-library.rb creative-status --version 1.0.98
#   bundle exec ruby scripts/asc-asset-library.rb place-creative --version 1.0.98
#
# `upload` is idempotent by name: an asset with the same reference name that has not
# failed is reported and left alone. It waits for processing and prints the spec App
# Store Connect matched the file to, which decides where the asset may be placed.
#
# The creative asset (fastlane/creative/<locale>/creative-16x9.png, from frame-creative.mjs)
# is part of every submission (the user, 2026-10-08), so it is named by its content:
# "gtmux creative 16:9 <locale> <sha256[0,8]>". `creative-status` answers, read-only and per
# locale, whether the current file is in the library and whether the version shows it as
# both its product page header and its search results asset. `place-creative` uploads what
# is missing and replaces any placement that shows another asset; it needs the version to
# be editable, because placements are reviewed with it. See docs/appstore/submit.md.

require 'spaceship'
require 'net/http'
require 'json'
require 'digest'

APP_ID = 'com.gtmux.app'
API = 'https://api.appstoreconnect.apple.com'
CATEGORIES = %w[CREATIVE_ASSETS APP_SCREENSHOTS_AND_PREVIEWS].freeze

TOKEN = Spaceship::ConnectAPI::Token.create(
  key_id: ENV.fetch('ASC_KEY_ID'),
  issuer_id: ENV['ASC_ISSUER_ID'],
  filepath: File.expand_path(ENV.fetch('ASC_KEY_PATH'))
)

def call(method, path, body = nil)
  uri = URI(path.start_with?('http') ? path : "#{API}#{path}")
  req = Net::HTTP.const_get(method.capitalize).new(uri)
  req['Authorization'] = "Bearer #{TOKEN.text}"
  if body
    req['Content-Type'] = 'application/json'
    req.body = JSON.generate(body)
  end
  res = Net::HTTP.start(uri.host, uri.port, use_ssl: true) { |h| h.request(req) }
  data = res.body.to_s.empty? ? {} : JSON.parse(res.body)
  return data if res.code.to_i < 300

  detail = (data['errors'] || []).map { |e| "#{e['code']}: #{e['detail'] || e['title']}" }.join('; ')
  abort "#{method} #{uri.path} -> HTTP #{res.code} #{detail}"
end

def opt(name)
  i = ARGV.index("--#{name}")
  i && ARGV[i + 1]
end

def library_id
  Spaceship::ConnectAPI.token = TOKEN
  app = Spaceship::ConnectAPI::App.find(APP_ID) or abort "no app #{APP_ID}"
  call('get', "/v1/apps/#{app.id}/assetLibrary").dig('data', 'id')
end

def images(lib, category = nil)
  path = "/v1/appAssetLibraries/#{lib}/images?limit=200&sort=-createdDate"
  path += "&filter[category]=#{category}" if category
  out = []
  while path
    page = call('get', path)
    out.concat(page['data'] || [])
    path = page.dig('links', 'next')
  end
  out
end

def show(img)
  a = img['attributes']
  dims = a['imageAsset'] ? "#{a['imageAsset']['width']}x#{a['imageAsset']['height']}" : '-'
  puts "#{img['id']}  #{a['state']}  #{a['category']}  #{dims}  #{a['referenceName'] || a['fileName']}"
end

def upload(lib, file, category, name)
  abort "no such file: #{file}" unless File.file?(file)
  abort "--category must be one of #{CATEGORIES.join(', ')}" unless CATEGORIES.include?(category)
  abort '--name is required (it is how a rerun finds the asset)' if name.to_s.empty?

  same = images(lib, category).find { |i| i.dig('attributes', 'referenceName') == name && i.dig('attributes', 'state') != 'FAILED' }
  if same
    puts 'already in the library, left alone:'
    return show(same)
  end

  bytes = File.binread(file)
  res = call('post', '/v1/appAssetLibraryImages', {data: {
    type: 'appAssetLibraryImages',
    attributes: {fileName: File.basename(file), fileSize: bytes.bytesize, category: category, referenceName: name},
    relationships: {assetLibrary: {data: {type: 'appAssetLibraries', id: lib}}}
  }})
  id = res.dig('data', 'id')
  res.dig('data', 'attributes', 'uploadOperations').each do |op|
    uri = URI(op['url'])
    req = Net::HTTP.const_get(op['method'].capitalize).new(uri)
    (op['requestHeaders'] || []).each { |h| req[h['name']] = h['value'] }
    req.body = bytes.byteslice(op['offset'], op['length'])
    put = Net::HTTP.start(uri.host, uri.port, use_ssl: true) { |h| h.request(req) }
    abort "upload part at #{op['offset']} -> HTTP #{put.code}" unless put.code.to_i < 300
  end
  call('patch', "/v1/appAssetLibraryImages/#{id}", {data: {type: 'appAssetLibraryImages', id: id, attributes: {uploaded: true}}})

  60.times do
    img = call('get', "/v1/appAssetLibraryImages/#{id}")['data']
    state = img.dig('attributes', 'state')
    if state == 'PREPARE_FOR_SUBMISSION'
      show(img)
      return puts "  spec #{img.dig('attributes', 'specId')}"
    end
    abort "processing failed: #{img.dig('attributes', 'stateDetails').inspect}" if state == 'FAILED'
    sleep 5
  end
  abort "still processing after 5 minutes: #{id}"
end

CREATIVE_LOCALES = {'en-US' => 'fastlane/creative/en-US/creative-16x9.png', 'zh-Hans' => 'fastlane/creative/zh-Hans/creative-16x9.png'}.freeze
CREATIVE_TYPES = %w[PRODUCT_PAGE_HEADER_ASSET APP_STORE_SEARCH_RESULTS_ASSET].freeze

def creative_name(locale, file)
  "gtmux creative 16:9 #{locale} #{Digest::SHA256.file(file).hexdigest[0, 8]}"
end

def version_localizations(version)
  Spaceship::ConnectAPI.token = TOKEN
  app = Spaceship::ConnectAPI::App.find(APP_ID)
  v = app.get_app_store_versions(filter: {versionString: version}).first or abort "no version #{version} in App Store Connect"
  [v, v.get_app_store_version_localizations.to_h { |l| [l.locale, l.id] }]
end

def placement_on(loc_id, type)
  path = "/v1/appStoreVersionLocalizations/#{loc_id}/placements?filter[placementType]=#{type}&include=image"
  (call('get', path)['data'] || []).first
end

# One row per locale and placement type: the asset the file should be, and what is placed.
def creative_rows(lib, version)
  v, locs = version_localizations(version)
  have = images(lib, 'CREATIVE_ASSETS').to_h { |i| [i.dig('attributes', 'referenceName'), i] }
  rows = CREATIVE_LOCALES.flat_map do |locale, file|
    abort "missing #{file}: render it with scripts/frame-creative.mjs" unless File.file?(file)
    name = creative_name(locale, file)
    asset = have[name]
    CREATIVE_TYPES.map do |type|
      placed = locs[locale] && placement_on(locs[locale], type)
      {locale: locale, file: file, name: name, asset: asset, type: type, loc: locs[locale], placed: placed,
       ok: asset && placed && placed.dig('relationships', 'image', 'data', 'id') == asset['id']}
    end
  end
  [v, rows]
end

def creative_status(lib, version)
  v, rows = creative_rows(lib, version)
  puts "version #{version} (#{v.app_store_state})"
  rows.each do |r|
    asset = r[:asset] ? "#{r[:asset]['id']} #{r[:asset].dig('attributes', 'state')}" : 'NOT IN LIBRARY'
    placed = r[:placed] ? "placed #{r[:placed].dig('attributes', 'state')}" : 'not placed'
    puts "#{r[:ok] ? 'ok  ' : 'TODO'}  #{r[:locale]}  #{r[:type]}  #{r[:name]}  asset: #{asset}  #{placed}"
  end
  exit(rows.all? { |r| r[:ok] } ? 0 : 1)
end

def place_creative(lib, version)
  v, rows = creative_rows(lib, version)
  abort "version #{version} is #{v.app_store_state}; placements need an editable version" unless v.app_store_state == 'PREPARE_FOR_SUBMISSION'
  rows.each do |r|
    next puts("ok    #{r[:locale]} #{r[:type]}") if r[:ok]
    abort "no #{r[:locale]} localization on #{version}" unless r[:loc]
    asset = r[:asset]
    unless asset
      upload(lib, r[:file], 'CREATIVE_ASSETS', r[:name])
      asset = images(lib, 'CREATIVE_ASSETS').find { |i| i.dig('attributes', 'referenceName') == r[:name] } or abort "uploaded #{r[:name]} but cannot find it"
    end
    rows.each { |o| o[:asset] = asset if o[:name] == r[:name] }
    call('delete', "/v1/appAssetLibraryPlacements/#{r[:placed]['id']}") if r[:placed]
    call('post', '/v1/appAssetLibraryPlacements', {data: {
      type: 'appAssetLibraryPlacements',
      attributes: {placementType: r[:type], placementGroup: 'DEFAULT_PROFILE'},
      relationships: {image: {data: {type: 'appAssetLibraryImages', id: asset['id']}},
                      appStoreVersionLocalization: {data: {type: 'appStoreVersionLocalizations', id: r[:loc]}}}
    }})
    puts "placed #{r[:locale]} #{r[:type]} <- #{r[:name]}"
  end
  creative_status(lib, version)
end

lib = library_id
case ARGV.first
when 'list' then images(lib, opt('category')).each { |i| show(i) }
when 'upload' then upload(lib, ARGV[1], opt('category'), opt('name'))
when 'creative-status' then creative_status(lib, opt('version') || abort('--version is required'))
when 'place-creative' then place_creative(lib, opt('version') || abort('--version is required'))
else abort File.read(__FILE__)[/^# Usage.*?\n#\n/m]
end
