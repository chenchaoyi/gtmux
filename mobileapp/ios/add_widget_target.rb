# Adds the GtmuxWidget Live Activity extension target to the Xcode project,
# programmatically (the reliable way — same lib CocoaPods uses). Idempotent.
require 'xcodeproj'

proj_path = 'GtmuxMobile.xcodeproj'
project = Xcodeproj::Project.open(proj_path)

app = project.targets.find { |t| t.name == 'GtmuxMobile' }
raise 'app target GtmuxMobile not found' unless app

if project.targets.any? { |t| t.name == 'GtmuxWidget' }
  puts 'GtmuxWidget target already exists — nothing to do'
  exit 0
end

# --- the widget extension target (iOS 16.1, Live Activities) ---
widget = project.new_target(:app_extension, 'GtmuxWidget', :ios, '16.1')

widget.build_configurations.each do |c|
  s = c.build_settings
  s['PRODUCT_BUNDLE_IDENTIFIER'] = 'com.gtmux.app.widget'
  s['PRODUCT_NAME'] = 'GtmuxWidget'
  s['INFOPLIST_FILE'] = 'GtmuxWidget/Info.plist'
  s['IPHONEOS_DEPLOYMENT_TARGET'] = '16.1'
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
widget_group = project.main_group.find_subpath('GtmuxWidget', true)
widget_group.set_source_tree('SOURCE_ROOT')
widget_group.set_path('GtmuxWidget')
widget_swift = widget_group.new_reference('GtmuxWidget.swift')
attrs_swift  = widget_group.new_reference('GtmuxActivityAttributes.swift')
widget_group.new_reference('Info.plist')

# widget compiles its UI + the shared attributes
widget.add_file_references([widget_swift, attrs_swift])

# the APP target compiles the shared attributes (so the native module sees the
# type) + the native module (Swift + ObjC bridge)
app_del = project.files.find { |f| f.path && f.path.end_with?('AppDelegate.swift') }
raise 'AppDelegate.swift ref not found' unless app_del
mobile_group = app_del.parent
prefix = File.dirname(app_del.path) # e.g. "GtmuxMobile"
mod_swift = mobile_group.new_reference(File.join(prefix, 'LiveActivityModule.swift'))
mod_m     = mobile_group.new_reference(File.join(prefix, 'LiveActivityModule.m'))
app.add_file_references([attrs_swift, mod_swift, mod_m])

# --- embed the widget into the app (+ build dependency) ---
app.add_dependency(widget)
embed = app.new_copy_files_build_phase('Embed Foundation Extensions')
embed.symbol_dst_subfolder_spec = :plug_ins
bf = embed.add_file_reference(widget.product_reference, true)
bf.settings = { 'ATTRIBUTES' => ['RemoveHeadersOnCopy'] }

project.save
puts "OK: added GtmuxWidget target (#{project.targets.map(&:name).join(', ')})"
