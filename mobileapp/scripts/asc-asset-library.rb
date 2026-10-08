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
#
# `upload` is idempotent by name: an asset with the same reference name that has not
# failed is reported and left alone. It waits for processing and prints the spec App
# Store Connect matched the file to, which decides where the asset may be placed.

require 'spaceship'
require 'net/http'
require 'json'

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

lib = library_id
case ARGV.first
when 'list' then images(lib, opt('category')).each { |i| show(i) }
when 'upload' then upload(lib, ARGV[1], opt('category'), opt('name'))
else abort File.read(__FILE__)[/^# Usage.*?\n#\n/m]
end
