#!/usr/bin/env ruby
# Regenerate the committed project using the repository's xcodeproj gem.
require 'xcodeproj'
root = File.expand_path('..', __dir__)
path = File.join(root, 'GtmuxWidgets.xcodeproj')
project = Xcodeproj::Project.new(path)
project.root_object.attributes['LastUpgradeCheck'] = '2600'
widget = project.new_target(:app_extension, 'GtmuxUsageWidget', :osx, '14.0')
host = project.new_target(:application, 'Gtmux', :osx, '14.0')
widget_sources = project.main_group.new_group('Widget', 'Widget')
shared_intent = widget_sources.new_file('MacUsageConfiguration.swift')
widget.source_build_phase.add_file_reference(widget_sources.new_file('UsageWidget.swift'))
widget.source_build_phase.add_file_reference(shared_intent)
widget_sources.new_file('Info.plist')
localization = widget_sources.new_file('Localizable.xcstrings')
[host, widget].each { |target| target.resources_build_phase.add_file_reference(localization) }
host_sources = project.main_group.new_group('WidgetHost', 'WidgetHost')
host.source_build_phase.add_file_reference(host_sources.new_file('main.swift'))
host.source_build_phase.add_file_reference(shared_intent)
[['.', ['MacUsageWidgetSupport', 'GtmuxBarCore']], ['../shared/UsageWidgets', ['GtmuxUsageWidgets']]].each do |relative, products|
  reference = project.new(Xcodeproj::Project::Object::XCLocalSwiftPackageReference)
  reference.relative_path = relative
  project.root_object.package_references << reference
  products.each do |product|
    targets = product == 'GtmuxBarCore' ? [host] : [host, widget]
    targets.each do |target|
      dependency = project.new(Xcodeproj::Project::Object::XCSwiftPackageProductDependency)
      dependency.package = reference
      dependency.product_name = product
      target.package_product_dependencies << dependency
      build_file = project.new(Xcodeproj::Project::Object::PBXBuildFile)
      build_file.product_ref = dependency
      target.frameworks_build_phase.files << build_file
    end
  end
end
host.add_dependency(widget)
embed = host.new_copy_files_build_phase('Embed App Extensions')
embed.dst_subfolder_spec = '13'
embedded = embed.add_file_reference(widget.product_reference)
embedded.settings = { 'ATTRIBUTES' => ['RemoveHeadersOnCopy'] }
[host, widget].each do |target|
  target.build_configurations.each do |config|
    config.build_settings.merge!({
      'GENERATE_INFOPLIST_FILE' => 'NO',
      'SWIFT_VERSION' => '5.0',
      'CODE_SIGNING_ALLOWED' => 'NO',
      'MARKETING_VERSION' => '0.0.0',
      'CURRENT_PROJECT_VERSION' => '1',
    })
    config.build_settings.merge!(target == widget ? {
      'GTMUX_WIDGET_BUNDLE_IDENTIFIER' => 'com.gtmux.menubar.usage',
      'PRODUCT_BUNDLE_IDENTIFIER' => '$(GTMUX_WIDGET_BUNDLE_IDENTIFIER)',
      'INFOPLIST_FILE' => 'Widget/Info.plist',
      'APPLICATION_EXTENSION_API_ONLY' => 'YES',
      'SKIP_INSTALL' => 'YES',
      'ENABLE_APP_SANDBOX' => 'YES',
    } : {
      'GTMUX_HOST_BUNDLE_IDENTIFIER' => 'com.gtmux.menubar',
      'PRODUCT_BUNDLE_IDENTIFIER' => '$(GTMUX_HOST_BUNDLE_IDENTIFIER)',
      'GTMUX_HOST_INFO_FILE' => 'build/Parent-Info.plist',
      'PRODUCT_MODULE_NAME' => 'GtmuxWidgetHost',
      'GTMUX_HOST_PREVIEW_CONDITIONS' => '',
      'SWIFT_ACTIVE_COMPILATION_CONDITIONS' => '$(inherited) $(GTMUX_HOST_PREVIEW_CONDITIONS)',
      'EXECUTABLE_NAME' => 'GtmuxBar',
      'INFOPLIST_FILE' => '$(GTMUX_HOST_INFO_FILE)',
      'SKIP_INSTALL' => 'NO',
      'ENABLE_APP_SANDBOX' => 'NO',
    })
  end
end
project.predictabilize_uuids
project.save
[host, widget].each do |target|
  scheme = Xcodeproj::XCScheme.new
  scheme.add_build_target(target)
  scheme.save_as(path, target.name, true)
end
