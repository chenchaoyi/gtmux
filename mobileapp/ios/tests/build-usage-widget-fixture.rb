require 'xcodeproj'
require 'fileutils'
require 'json'
# Generates a standalone simulator-only App and real Widget extension.
# Pairing data and credentials are never loaded by the synthetic UI/checks.
root=File.expand_path('../../..', __dir__)
dir=File.expand_path(ARGV.fetch(0))
raise 'Keep fixture output outside the source checkout' if dir == root || dir.start_with?(root + '/')
FileUtils.mkdir_p(dir+'/Sources')
%w[UsageSnapshot.swift UsageSnapshotStore.swift UsageWidgetPresentation.swift UsageWidgetViews.swift UsageWidgetSelection.swift].each do |name|
  text=File.read(root+'/shared/UsageWidgets/Sources/GtmuxUsageWidgets/'+name)
  text=text.gsub('group.com.gtmux.app.usage','group.com.gtmux.widget-fixture.usage')
  File.write(dir+'/Sources/'+name,text)
end
FileUtils.cp(root+'/mobileapp/ios/tests/UsageWidgetFixture.swift',dir+'/Sources/UsageWidgetFixture.swift')
definition = File.read(root+'/mobileapp/ios/UsageWidgets/UsageWidgetDefinition.swift')
File.write(dir+'/Sources/UsageWidgetDefinition.swift', definition)
File.write(dir+'/Sources/FixtureWidgets.swift', "import WidgetKit\nimport SwiftUI\n@main struct FixtureWidgets: WidgetBundle { var body: some Widget { GtmuxUsageWidget() } }\n")
%w[WidgetOwnerConnection.swift WidgetUsageRequest.swift].each { |n| FileUtils.cp(root+'/shared/UsageWidgets/Sources/GtmuxUsageWidgetHost/'+n,dir+'/Sources/'+n) }
%w[UsageWidgetRefresh.swift].each { |n| FileUtils.cp(root+'/mobileapp/ios/UsageWidgets/'+n,dir+'/Sources/'+n) }
FileUtils.cp(root+'/mobileapp/ios/tests/UsageWidgetRefreshFixtureChecks.swift',dir+'/Sources/UsageWidgetRefreshFixtureChecks.swift')
project=Xcodeproj::Project.new(dir+'/WidgetFixture.xcodeproj')
app=project.new_target(:application,'UsageFixture',:ios,'17.0')
ext=project.new_target(:app_extension,'UsageFixtureWidget',:ios,'17.0')
app.add_dependency(ext)
phase=app.new_copy_files_build_phase('Embed Widget');phase.dst_subfolder_spec='13';phase.add_file_reference(ext.product_reference)
group=project.main_group.new_group('Sources','Sources')
%w[UsageSnapshot.swift UsageSnapshotStore.swift UsageWidgetPresentation.swift UsageWidgetViews.swift UsageWidgetSelection.swift].each do |name|
  ref=group.new_reference(name);app.add_file_references([ref]);ext.add_file_references([ref])
end
app.add_file_references(%w[UsageWidgetFixture.swift UsageWidgetRefreshFixtureChecks.swift WidgetOwnerConnection.swift WidgetUsageRequest.swift UsageWidgetRefresh.swift].map { |n| group.new_reference(n) })
%w[UsageWidgetDefinition.swift FixtureWidgets.swift].each { |n| ext.add_file_references([group.new_reference(n)]) }
strings=project.main_group.new_reference(root+'/mobileapp/ios/GtmuxWidget/Localizable.xcstrings')
ext.resources_build_phase.add_file_reference(strings)
assets=project.main_group.new_reference(root+'/mobileapp/ios/GtmuxWidget/Assets.xcassets')
[app,ext].each do |target|
 target.resources_build_phase.add_file_reference(assets)
 target.build_configurations.each do |c|
  c.build_settings.merge!({'SWIFT_VERSION'=>'5.0','PRODUCT_BUNDLE_IDENTIFIER'=>'com.gtmux.widget-fixture'+(target==ext ? '.widget' : ''),'INFOPLIST_FILE'=>target.name+'.plist','CODE_SIGNING_ALLOWED'=>'NO','TARGETED_DEVICE_FAMILY'=>'1,2','SWIFT_STRICT_CONCURRENCY'=>'minimal','GENERATE_INFOPLIST_FILE'=>'NO','ASSETCATALOG_COMPILER_APPICON_NAME'=>'','ASSETCATALOG_COMPILER_GLOBAL_ACCENT_COLOR_NAME'=>''})
 end
end
ext.build_configurations.each { |c| c.build_settings['APPLICATION_EXTENSION_API_ONLY']='YES'; c.build_settings['SKIP_INSTALL']='YES' }
base={'CFBundleExecutable'=>'$(EXECUTABLE_NAME)','CFBundleIdentifier'=>'$(PRODUCT_BUNDLE_IDENTIFIER)','CFBundleName'=>'$(PRODUCT_NAME)','CFBundleDisplayName'=>'Usage QA','CFBundleVersion'=>'1','CFBundleShortVersionString'=>'1.0','CFBundlePackageType'=>'APPL','UILaunchScreen'=>{},'UISupportedInterfaceOrientations'=>%w[UIInterfaceOrientationPortrait UIInterfaceOrientationLandscapeLeft UIInterfaceOrientationLandscapeRight],'CFBundleURLTypes'=>[{'CFBundleURLSchemes'=>['gtmux']}],'NSAppTransportSecurity'=>{'NSAllowsLocalNetworking'=>true}}
Xcodeproj::Plist.write_to_path(base,dir+'/UsageFixture.plist')
base=base.merge('CFBundlePackageType'=>'XPC!','NSExtension'=>{'NSExtensionPointIdentifier'=>'com.apple.widgetkit-extension'})
Xcodeproj::Plist.write_to_path(base,dir+'/UsageFixtureWidget.plist')
Xcodeproj::Plist.write_to_path({'com.apple.security.application-groups'=>['group.com.gtmux.widget-fixture.usage']},dir+'/fixture.entitlements')
project.save
log=File.open(dir+'/build.log','w')
command=['xcodebuild','-project',project.path.to_s,'-scheme','UsageFixture','-configuration','Debug','-sdk','iphonesimulator','-destination','generic/platform=iOS Simulator','-derivedDataPath',dir+'/Build','-jobs','2','ARCHS=arm64','CODE_SIGNING_ALLOWED=NO','build']
raise "Fixture build failed; inspect #{dir}/build.log" unless system(*command, out:log, err:log)
app_path=dir+'/Build/Build/Products/Debug-iphonesimulator/UsageFixture.app'
[app_path+'/PlugIns/UsageFixtureWidget.appex',app_path].each do |path|
  raise 'Fixture signing failed' unless system('codesign','--force','--sign','-','--entitlements',dir+'/fixture.entitlements',path,out:log,err:log)
end
log.close
puts app_path
