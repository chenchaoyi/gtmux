# Adds the TermSelection native module (TermSelectionView.swift +
# TermSelectionManager.swift + TermSelection.m) to the GtmuxMobile app target —
# the Stage 2 iOS-native terminal text selection layer
# (openspec/changes/mobile-native-term-selection). Idempotent — safe to re-run.
require 'xcodeproj'

project = Xcodeproj::Project.open('GtmuxMobile.xcodeproj')
app = project.targets.find { |t| t.name == 'GtmuxMobile' }
raise 'GtmuxMobile target not found' unless app

# A dedicated group mirroring the ios/TermSelection/ directory.
group = project.main_group['TermSelection']
group ||= project.main_group.new_group('TermSelection', 'TermSelection')

%w[TermSelectionView.swift TermSelectionManager.swift TermSelection.m].each do |name|
  rel = File.join('TermSelection', name)
  ref = project.files.find { |f| f.path == rel || (f.parent == group && f.path == name) }
  ref ||= group.new_reference(name)
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
