# Build-graph guard, not a claim about every future API a widget could call.
require 'xcodeproj'
require 'digest'
require 'set'

root = File.expand_path('../../..', __dir__)
ios = File.join(root, 'mobileapp/ios')
project = Xcodeproj::Project.open(ARGV.first || File.join(ios, 'GtmuxMobile.xcodeproj'))
app = project.targets.find { |t| t.name == 'GtmuxMobile' }
widget = project.targets.find { |t| t.name == 'GtmuxWidget' }
raise 'App and widget targets required' unless app && widget
def sources(target)
  target.source_build_phase.files.filter_map { |b| b.file_ref&.real_path&.to_s }.map { |p| File.expand_path(p) }.to_set
end
app_sources = sources(app)
widget_sources = sources(widget)
host = Dir.glob(File.join(root, 'shared/UsageWidgets/Sources/GtmuxUsageWidgetHost/*.swift'))
parent = Dir.glob(File.join(ios, 'UsageWidgets/*.{swift,m}')).reject { |p| File.basename(p) == 'UsageWidgetDefinition.swift' }
(host + parent).each do |file|
  raise "Missing parent source: #{File.basename(file)}" unless app_sources.include?(file)
  raise "Parent-only code leaked into widget: #{File.basename(file)}" if widget_sources.include?(file)
end
Dir.glob(File.join(root, 'shared/UsageWidgets/Sources/GtmuxUsageWidgets/*.swift')).each do |file|
  raise "Missing display source: #{File.basename(file)}" unless widget_sources.include?(file)
end
raise 'Missing WidgetKit configuration' unless widget_sources.include?(File.join(ios, 'UsageWidgets/UsageWidgetDefinition.swift'))

source = File.read(File.join(root, 'shared/UsageWidgets/Sources/GtmuxUsageWidgets/UsageSnapshotStore.swift'))
group = source.match(/groupIdentifier = "([^"]+)"/)&.captures&.first
raise 'Missing shared-container identity' unless group
[app, widget].each do |target|
  target.build_configurations.each do |configuration|
    setting = configuration.build_settings['CODE_SIGN_ENTITLEMENTS']
    raise "Missing entitlement file in #{target.name}/#{configuration.name}" unless setting.is_a?(String) && !setting.empty?
    file = File.expand_path(setting, ios)
    entitlements = Xcodeproj::Plist.read_from_path(file)
    raise "Wrong App Group in #{target.name}" unless entitlements['com.apple.security.application-groups'] == [group]
    if target == widget && entitlements.keys.any? { |key| key.include?('keychain') }
      raise 'The display widget must not receive a Keychain access group'
    end
  end
end
Dir.glob(File.join(ios, 'GtmuxWidget/Assets.xcassets/UsageAgent-*.imageset')).each do |directory|
  key = File.basename(directory).delete_prefix('UsageAgent-').delete_suffix('.imageset')
  canonical = File.join(root, "assets/agent-icons/#{key}.png")
  copies = [File.join(directory, 'mark.png'), File.join(root, "shared/UsageWidgets/Sources/GtmuxUsageWidgets/Resources/UsageAgent-#{key}.png")]
  copies.each { |copy| raise "Icon differs from its official source: #{copy}" unless Digest::SHA256.file(copy) == Digest::SHA256.file(canonical) }
end
puts 'Usage widget target, entitlement and canonical-icon boundaries passed'
