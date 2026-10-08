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
#   bundle exec ruby scripts/asc-asset-library.rb delete IMAGE_ID
#   bundle exec ruby scripts/asc-asset-library.rb creative-status --version 1.0.98
#   bundle exec ruby scripts/asc-asset-library.rb place-creative --version 1.0.98
#   bundle exec ruby scripts/asc-asset-library.rb screenshot-status --version 1.0.98
#   bundle exec ruby scripts/asc-asset-library.rb place-screenshots --version 1.0.98 [--dry-run]
#
# `upload` is idempotent by name: a usable asset with the same reference name (processed,
# not rejected or archived; an approved one included) is reported and left alone. Before
# that, it settles the others of that name: a reservation an interrupted run left without
# its bytes (AWAITING_UPLOAD) is deleted, one still processing is waited for, and failed,
# rejected or archived ones are passed over. A reservation this run makes and cannot finish
# is deleted on the way out. It waits for processing and prints the spec App Store Connect
# matched the file to. Status commands only read; only upload, delete and place-* change
# anything.
#
# Placement groups come from the reference data (GET /v1/appAssetLibraryRefData), found by
# the file's exact size, never from a list in this file, and are resolved before any
# placement is removed, so an unknown size stops the run with nothing changed.
#
# The creative asset (fastlane/creative/<locale>/creative-16x9.png, from frame-creative.mjs)
# is part of every submission (the user, 2026-10-08), so it is named by its content:
# "gtmux creative 16:9 <locale> <sha256[0,8]>". `creative-status` answers, read-only and per
# locale, whether the current file is in the library and whether the version shows it as
# both its product page header and its search results asset. `place-creative` uploads what
# is missing and replaces any placement that shows another asset; it needs the version to
# be editable, because placements are reviewed with it. See docs/appstore/submit.md.
#
# Screenshots go the same way since API 4.5.1 deprecated the set-based resources deliver
# uploads through (and deliver's double uploads needed a dedupe script after every run).
# fastlane/screenshots/<locale>/01.png… are the iPhone 6.9" group, ipad-01.png… the iPad
# 13" group. `place-screenshots` reuses an image already in the library (named
# "gtmux screenshot <file> <sha256[0,8]>"), uploads the rest, and replaces a group's
# placements only when they are not already these files in this order. `screenshot-status`
# compares, read-only; an image deliver uploaded before this script counts as the same
# file when its file name and byte size match. `--dir` points at another screenshot tree.

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

# Processed and placeable. An approved image is what a later version reuses.
USABLE = %w[PREPARE_FOR_SUBMISSION READY_FOR_REVIEW WAITING_FOR_REVIEW IN_REVIEW ACCEPTED APPROVED].freeze

def named(pool, name)
  pool.select { |i| i.dig('attributes', 'referenceName') == name }
end

# The newest usable image of this name in the pool (images() lists newest first). Reads only.
def usable(pool, name)
  named(pool, name).find { |i| USABLE.include?(i.dig('attributes', 'state')) }
end

def wait_processed(id)
  60.times do
    img = call('get', "/v1/appAssetLibraryImages/#{id}")['data']
    return img unless %w[AWAITING_UPLOAD UPLOAD_COMPLETE].include?(img.dig('attributes', 'state'))

    sleep 5
  end
  abort "still processing after 5 minutes: #{id}"
end

# Settle the images of this name before deciding to upload: delete a reservation that never
# got its bytes (nothing can be placed on it, so nothing uses it) and wait for one still
# processing. %7's review of #1535: a run interrupted between reserve and commit left such
# a reservation, and the next run took it as "already in the library".
def settle(pool, name)
  named(pool, name).each do |i|
    case i.dig('attributes', 'state')
    when 'AWAITING_UPLOAD'
      call('delete', "/v1/appAssetLibraryImages/#{i['id']}")
      pool.delete(i)
      puts "deleted an unfinished upload of #{name} (#{i['id']})"
    when 'UPLOAD_COMPLETE'
      i.replace(wait_processed(i['id']))
    end
  end
end

# Returns the usable image for this file and name, uploading it when the library has none.
def upload(lib, file, category, name, pool = nil)
  abort "no such file: #{file}" unless File.file?(file)
  abort "--category must be one of #{CATEGORIES.join(', ')}" unless CATEGORIES.include?(category)
  abort '--name is required (it is how a rerun finds the asset)' if name.to_s.empty?

  pool ||= images(lib, category)
  settle(pool, name)
  if (have = usable(pool, name))
    puts 'already in the library, left alone:'
    show(have)
    return have
  end

  bytes = File.binread(file)
  res = call('post', '/v1/appAssetLibraryImages', {data: {
    type: 'appAssetLibraryImages',
    attributes: {fileName: File.basename(file), fileSize: bytes.bytesize, category: category, referenceName: name},
    relationships: {assetLibrary: {data: {type: 'appAssetLibraries', id: lib}}}
  }})
  id = res.dig('data', 'id')
  committed = false
  begin
    res.dig('data', 'attributes', 'uploadOperations').each do |op|
      uri = URI(op['url'])
      req = Net::HTTP.const_get(op['method'].capitalize).new(uri)
      (op['requestHeaders'] || []).each { |h| req[h['name']] = h['value'] }
      req.body = bytes.byteslice(op['offset'], op['length'])
      put = Net::HTTP.start(uri.host, uri.port, use_ssl: true) { |h| h.request(req) }
      abort "upload part at #{op['offset']} -> HTTP #{put.code}" unless put.code.to_i < 300
    end
    call('patch', "/v1/appAssetLibraryImages/#{id}", {data: {type: 'appAssetLibraryImages', id: id, attributes: {uploaded: true}}})
    committed = true
  ensure
    unless committed
      begin
        call('delete', "/v1/appAssetLibraryImages/#{id}")
        warn "removed the unfinished reservation #{id}"
      rescue SystemExit, StandardError
        warn "could not remove the unfinished reservation #{id}; the next upload of #{name} deletes it"
      end
    end
  end

  img = wait_processed(id)
  abort "processing failed: #{img.dig('attributes', 'stateDetails').inspect}" unless img.dig('attributes', 'state') == 'PREPARE_FOR_SUBMISSION'
  show(img)
  puts "  spec #{img.dig('attributes', 'specId')}"
  pool.unshift(img)
  img
end

# Deletes an image the library holds, refusing while a placement still shows it.
def delete_image(id)
  used = call('get', "/v1/appAssetLibraryImages/#{id}/relationships/placements")['data'] || []
  abort "#{id} is still placed #{used.length} time(s); delete those placements first" unless used.empty?
  call('delete', "/v1/appAssetLibraryImages/#{id}")
  puts "deleted #{id}"
end

def ref_data
  @ref_data ||= Array(call('get', '/v1/appAssetLibraryRefData')['data']).first['attributes']
end

# The one placement group the reference data gives this placement type for an image of
# exactly this size. Nothing is placed when there is none, or more than one.
def group_for(type, width, height)
  pt = ref_data['placementTypes'].find { |t| t['placementTypeId'] == type } or abort "no placement type #{type} in the reference data"
  specs = ref_data['imageSpecs'].select do |sp|
    d = sp['dimensions']
    (sp['compatiblePlacementTypes'] || []).include?(type) &&
      [d['minWidth'], d['maxWidth']].uniq == [width] && [d['minHeight'], d['maxHeight']].uniq == [height]
  end.map { |sp| sp['specId'] }
  groups = pt['specMappings'].select { |m| (m['specs'] & specs).any? }.map { |m| m['placementGroupId'] }.uniq
  return groups.first if groups.length == 1

  abort "#{width}x#{height} #{type}: the reference data gives #{groups.empty? ? 'no placement group' : groups.join(', ')}; expected exactly one"
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
  pool = images(lib, 'CREATIVE_ASSETS')
  rows = CREATIVE_LOCALES.flat_map do |locale, file|
    abort "missing #{file}: render it with scripts/frame-creative.mjs" unless File.file?(file)
    name = creative_name(locale, file)
    asset = usable(pool, name)
    others = named(pool, name).reject { |i| i.equal?(asset) }.map { |i| i.dig('attributes', 'state') }
    CREATIVE_TYPES.map do |type|
      placed = locs[locale] && placement_on(locs[locale], type)
      {locale: locale, file: file, name: name, asset: asset, others: others, type: type, loc: locs[locale], placed: placed,
       group: group_for(type, *png_size(file)),
       ok: asset && placed && placed.dig('relationships', 'image', 'data', 'id') == asset['id']}
    end
  end
  [v, rows, pool]
end

def creative_status(lib, version)
  v, rows, = creative_rows(lib, version)
  puts "version #{version} (#{v.app_store_state})"
  rows.each do |r|
    asset = r[:asset] ? "#{r[:asset]['id']} #{r[:asset].dig('attributes', 'state')}" : 'NOT USABLE IN LIBRARY'
    asset += " (also #{r[:others].join(', ')})" unless r[:others].empty?
    placed = r[:placed] ? "placed #{r[:placed].dig('attributes', 'state')}" : 'not placed'
    puts "#{r[:ok] ? 'ok  ' : 'TODO'}  #{r[:locale]}  #{r[:type]}  #{r[:name]}  asset: #{asset}  #{placed}"
  end
  exit(rows.all? { |r| r[:ok] } ? 0 : 1)
end

def place_creative(lib, version)
  v, rows, pool = creative_rows(lib, version)
  abort "version #{version} is #{v.app_store_state}; placements need an editable version" unless v.app_store_state == 'PREPARE_FOR_SUBMISSION'
  missing = rows.reject { |r| r[:loc] }.map { |r| r[:locale] }.uniq
  abort "no #{missing.join(', ')} localization on #{version}" unless missing.empty?
  rows.each do |r|
    next puts("ok    #{r[:locale]} #{r[:type]}") if r[:ok]

    asset = r[:asset] || upload(lib, r[:file], 'CREATIVE_ASSETS', r[:name], pool)
    rows.each { |o| o[:asset] = asset if o[:name] == r[:name] }
    call('delete', "/v1/appAssetLibraryPlacements/#{r[:placed]['id']}") if r[:placed]
    call('post', '/v1/appAssetLibraryPlacements', {data: {
      type: 'appAssetLibraryPlacements',
      attributes: {placementType: r[:type], placementGroup: r[:group]},
      relationships: {image: {data: {type: 'appAssetLibraryImages', id: asset['id']}},
                      appStoreVersionLocalization: {data: {type: 'appStoreVersionLocalizations', id: r[:loc]}}}
    }})
    puts "placed #{r[:locale]} #{r[:type]} <- #{r[:name]}"
  end
  creative_status(lib, version)
end

SHOT_DIR = 'fastlane/screenshots'
# Which files form a group, and the size each must be. The placement group itself comes
# from the reference data for that size (group_for).
SHOT_GROUPS = [
  {slot: 'iPhone 6.9"', match: /\A\d\d\.png\z/, size: [1320, 2868]},
  {slot: 'iPad 13"', match: /\Aipad-\d\d\.png\z/, size: [2752, 2064]}
].freeze

def png_size(file)
  head = File.binread(file, 24)
  abort "#{file} is not a PNG" unless head[0, 8] == "\x89PNG\r\n\x1a\n".b
  head[16, 8].unpack('NN')
end

def shot_name(file)
  "gtmux screenshot #{File.basename(file)} #{Digest::SHA256.file(file).hexdigest[0, 8]}"
end

# Per locale directory and group, the files in display order, each checked for its size.
def shot_plan(dir)
  abort "no screenshot directory #{dir}" unless File.directory?(dir)
  Dir.children(dir).select { |l| File.directory?(File.join(dir, l)) }.sort.flat_map do |locale|
    files = Dir.children(File.join(dir, locale)).sort
    SHOT_GROUPS.filter_map do |g|
      list = files.grep(g[:match]).map { |f| File.join(dir, locale, f) }
      next if list.empty?

      list.each { |f| abort "#{f} is #{png_size(f).join('x')}, want #{g[:size].join('x')} for #{g[:slot]}" unless png_size(f) == g[:size] }
      {locale: locale, group: group_for('APP_SCREENSHOT', *g[:size]), files: list}
    end
  end
end

def shot_placements(loc_id, group)
  r = call('get', "/v1/appStoreVersionLocalizations/#{loc_id}/placements?filter[placementType]=APP_SCREENSHOT" \
                  "&filter[placementGroup]=#{group}&sort=placementGroupPosition&include=image&limit=50")
  imgs = (r['included'] || []).to_h { |i| [i['id'], i] }
  (r['data'] || []).map { |pl| {placement: pl, image: imgs[pl.dig('relationships', 'image', 'data', 'id')]} }
end

# How a placed image is known to be the local file: :hash by its content name, :name_size by
# file name and byte size, or nil. The weak match is only for images this script did not
# name: deliver's uploads, which the library migration named like "ipad-04.png (87)".
def same_file(image, file)
  return nil unless image

  a = image['attributes']
  return :hash if a['referenceName'] == shot_name(file)
  return nil if a['referenceName'].to_s.start_with?('gtmux screenshot ')
  return :name_size if a['fileName'] == File.basename(file) && a['fileSize'] == File.size(file)

  nil
end

def shot_rows(version, dir)
  v, locs = version_localizations(version)
  rows = shot_plan(dir).map do |sh|
    loc = locs[sh[:locale]] or abort "no #{sh[:locale]} localization on #{version}"
    placed = shot_placements(loc, sh[:group])
    how = placed.zip(sh[:files]).map { |pl, f| same_file(pl[:image], f) }
    ok = placed.length == sh[:files].length && how.all?
    sh.merge(loc: loc, placed: placed, ok: ok, weak: how.count(:name_size))
  end
  [v, rows]
end

def screenshot_status(version, dir)
  v, rows = shot_rows(version, dir)
  puts "version #{version} (#{v.app_store_state})"
  rows.each do |r|
    weak = r[:weak].positive? ? "  (#{r[:weak]} same by name+size)" : ''
    puts "#{r[:ok] ? 'ok  ' : 'TODO'}  #{r[:locale]}  #{r[:group]}  local #{r[:files].length}, placed #{r[:placed].length}#{weak}"
    next if r[:ok]

    r[:files].each_with_index do |f, i|
      pl = r[:placed][i]
      seen = if pl.nil? then 'nothing placed'
             elsif (how = same_file(pl[:image], f)) then how == :hash ? 'same' : 'same (name+size)'
             else "placed #{pl[:image]&.dig('attributes', 'fileName')} (#{pl[:image]&.dig('attributes', 'fileSize')} bytes)"
             end
      puts "      #{i + 1}. #{File.basename(f)}  #{seen}"
    end
  end
  exit(rows.all? { |r| r[:ok] } ? 0 : 1)
end

def place_screenshots(lib, version, dir, dry)
  v, rows = shot_rows(version, dir)
  unless dry || v.app_store_state == 'PREPARE_FOR_SUBMISSION'
    abort "version #{version} is #{v.app_store_state}; placements need an editable version"
  end
  pool = images(lib, 'APP_SCREENSHOTS_AND_PREVIEWS')
  rows.each do |r|
    next puts("ok    #{r[:locale]} #{r[:group]} (#{r[:files].length})") if r[:ok]

    puts "#{dry ? 'would replace' : 'replace'}  #{r[:locale]} #{r[:group]}: #{r[:placed].length} placed -> #{r[:files].length} files"
    next if dry

    ids = r[:files].map { |f| upload(lib, f, 'APP_SCREENSHOTS_AND_PREVIEWS', shot_name(f), pool)['id'] }
    r[:placed].each { |pl| call('delete', "/v1/appAssetLibraryPlacements/#{pl[:placement]['id']}") }
    created = ids.map do |id|
      call('post', '/v1/appAssetLibraryPlacements', {data: {
        type: 'appAssetLibraryPlacements',
        attributes: {placementType: 'APP_SCREENSHOT', placementGroup: r[:group]},
        relationships: {image: {data: {type: 'appAssetLibraryImages', id: id}},
                        appStoreVersionLocalization: {data: {type: 'appStoreVersionLocalizations', id: r[:loc]}}}
      }}).dig('data', 'id')
    end
    call('post', '/v1/appAssetLibraryPlacementOrderingRequests', {data: {
      type: 'appAssetLibraryPlacementOrderingRequests',
      attributes: {placementGroup: r[:group]},
      relationships: {orderedPlacements: {data: created.map { |id| {type: 'appAssetLibraryPlacements', id: id} }},
                      appStoreVersionLocalization: {data: {type: 'appStoreVersionLocalizations', id: r[:loc]}}}
    }})
  end
  screenshot_status(version, dir) unless dry
end

lib = library_id
case ARGV.first
when 'list' then images(lib, opt('category')).each { |i| show(i) }
when 'upload' then upload(lib, ARGV[1], opt('category'), opt('name'))
when 'delete' then delete_image(ARGV[1] || abort('delete needs an image id'))
when 'creative-status' then creative_status(lib, opt('version') || abort('--version is required'))
when 'place-creative' then place_creative(lib, opt('version') || abort('--version is required'))
when 'screenshot-status' then screenshot_status(opt('version') || abort('--version is required'), opt('dir') || SHOT_DIR)
when 'place-screenshots'
  place_screenshots(lib, opt('version') || abort('--version is required'), opt('dir') || SHOT_DIR, ARGV.include?('--dry-run'))
else abort File.read(__FILE__)[/^# Usage.*?\n#\n/m]
end
