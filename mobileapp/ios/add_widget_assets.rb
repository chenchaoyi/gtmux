# Adds GtmuxWidget/Assets.xcassets (holding the real app-icon BrandIcon) to the
# GtmuxWidget extension target's resources. Idempotent — safe to re-run. Separate
# from add_widget_target.rb because the widget target already exists in-repo.
require 'xcodeproj'

project = Xcodeproj::Project.open('GtmuxMobile.xcodeproj')
widget = project.targets.find { |t| t.name == 'GtmuxWidget' }
raise 'GtmuxWidget target not found — run add_widget_target.rb first' unless widget

widget_group = project.main_group.find_subpath('GtmuxWidget', true)
existing = widget_group.files.find { |f| f.path == 'Assets.xcassets' }
ref = existing || widget_group.new_reference('Assets.xcassets')

already = widget.resources_build_phase.files.any? { |bf| bf.file_ref == ref }
if already
  puts 'Assets.xcassets already in GtmuxWidget resources — nothing to do'
else
  widget.resources_build_phase.add_file_reference(ref, true)
  project.save
  puts 'OK: added Assets.xcassets to GtmuxWidget resources'
end
