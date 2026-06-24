# Adds DebugSettings.swift + DebugSettings.m to the GtmuxMobile app target so the
# launch-arg debug native module compiles in. Idempotent — safe to re-run.
require 'xcodeproj'

project = Xcodeproj::Project.open('GtmuxMobile.xcodeproj')
app = project.targets.find { |t| t.name == 'GtmuxMobile' }
raise 'GtmuxMobile target not found' unless app

# Find the group that already holds AppDelegate.swift (the app source group).
app_del = project.files.find { |f| f.path && f.path.end_with?('AppDelegate.swift') }
raise 'AppDelegate.swift ref not found' unless app_del
group = app_del.parent
prefix = File.dirname(app_del.path) # e.g. "GtmuxMobile"

%w[DebugSettings.swift DebugSettings.m].each do |name|
  rel = File.join(prefix, name)
  ref = project.files.find { |f| f.path == rel }
  ref ||= group.new_reference(rel)
  already = app.source_build_phase.files.any? { |bf| bf.file_ref == ref }
  if already
    puts "#{name} already in GtmuxMobile sources"
  else
    app.add_file_references([ref])
    puts "added #{name} to GtmuxMobile sources"
  end
end

project.save
puts 'OK'
