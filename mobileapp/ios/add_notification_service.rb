# Adds the GtmuxNotificationService UNNotificationServiceExtension target to the
# Xcode project (the same programmatic way as add_widget_target.rb). Idempotent.
# The extension attaches a per-kind status badge to each `mutable-content` push.
require 'xcodeproj'

proj_path = 'GtmuxMobile.xcodeproj'
project = Xcodeproj::Project.open(proj_path)

app = project.targets.find { |t| t.name == 'GtmuxMobile' }
raise 'app target GtmuxMobile not found' unless app

if project.targets.any? { |t| t.name == 'GtmuxNotificationService' }
  puts 'GtmuxNotificationService target already exists — nothing to do'
  exit 0
end

nse = project.new_target(:app_extension, 'GtmuxNotificationService', :ios, '15.1')

nse.build_configurations.each do |c|
  s = c.build_settings
  s['PRODUCT_BUNDLE_IDENTIFIER'] = 'com.gtmux.app.notificationservice'
  s['PRODUCT_NAME'] = 'GtmuxNotificationService'
  s['INFOPLIST_FILE'] = 'GtmuxNotificationService/Info.plist'
  s['IPHONEOS_DEPLOYMENT_TARGET'] = '15.1'
  s['SWIFT_VERSION'] = '5.0'
  s['TARGETED_DEVICE_FAMILY'] = '1,2'
  s['CODE_SIGN_STYLE'] = 'Automatic'
  s['GENERATE_INFOPLIST_FILE'] = 'NO'
  s['CURRENT_PROJECT_VERSION'] = '1'
  s['MARKETING_VERSION'] = '1.0'
  s['CLANG_ENABLE_MODULES'] = 'YES'
  s['LD_RUNPATH_SEARCH_PATHS'] =
    ['$(inherited)', '@executable_path/Frameworks', '@executable_path/../../Frameworks']
end

# --- file references ---
grp = project.main_group.find_subpath('GtmuxNotificationService', true)
grp.set_source_tree('SOURCE_ROOT')
grp.set_path('GtmuxNotificationService')
svc_swift = grp.new_reference('NotificationService.swift')
grp.new_reference('Info.plist')
nse.add_file_references([svc_swift])

# --- embed into the app (reuse the widget's Embed Foundation Extensions phase so
#     there's a single one), + build dependency ---
app.add_dependency(nse)
embed = app.copy_files_build_phases.find { |ph| ph.symbol_dst_subfolder_spec == :plug_ins }
unless embed
  embed = app.new_copy_files_build_phase('Embed Foundation Extensions')
  embed.symbol_dst_subfolder_spec = :plug_ins
end
bf = embed.add_file_reference(nse.product_reference, true)
bf.settings = { 'ATTRIBUTES' => ['RemoveHeadersOnCopy'] }

project.save
puts "OK: added GtmuxNotificationService target (#{project.targets.map(&:name).join(', ')})"
