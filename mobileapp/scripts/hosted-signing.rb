# Runner-only manual signing for the three App targets. No source commit is made.
require 'json'
require 'xcodeproj'

profiles = JSON.parse(File.read(ARGV.fetch(0)))
project = Xcodeproj::Project.open(ARGV.fetch(1))
matched = []
project.targets.each do |target|
  target.build_configurations.each do |config|
    next unless config.name == 'Release'
    bundle = config.build_settings['PRODUCT_BUNDLE_IDENTIFIER']
    next unless profiles.key?(bundle)
    config.build_settings['CODE_SIGN_STYLE'] = 'Manual'
    config.build_settings['CODE_SIGN_IDENTITY'] = 'Apple Distribution'
    config.build_settings['PROVISIONING_PROFILE_SPECIFIER'] = profiles.fetch(bundle)
    config.build_settings['DEVELOPMENT_TEAM'] = ENV.fetch('IOS_TEAM_ID')
    matched << bundle
  end
end
raise 'App signing targets do not match supplied profiles' unless matched.sort == profiles.keys.sort
project.save
