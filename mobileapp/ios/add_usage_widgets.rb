# Register display-only shared sources and parent-only refresh/bridge sources.
# Idempotent. No Keychain code is compiled into the widget extension.
require 'xcodeproj'
project = Xcodeproj::Project.open('GtmuxMobile.xcodeproj')
app = project.targets.find { |t| t.name == 'GtmuxMobile' }
widget = project.targets.find { |t| t.name == 'GtmuxWidget' }
raise 'App and widget targets required' unless app && widget
def register(project, directory, names, targets)
  group = project.main_group[directory] || project.main_group.new_group(directory, directory)
  names.each do |name|
    ref = group.files.find { |f| f.path == name } || group.new_reference(name)
    targets.each do |target|
      target.add_file_references([ref]) unless target.source_build_phase.files.any? { |b| b.file_ref == ref }
    end
  end
end
register(project, '../../shared/UsageWidgets/Sources/GtmuxUsageWidgets', %w[UsageSnapshot.swift UsageSnapshotStore.swift], [app, widget])
register(project, '../../shared/UsageWidgets/Sources/GtmuxUsageWidgets', %w[UsageWidgetPresentation.swift UsageWidgetViews.swift UsageWidgetSelection.swift], [widget])
register(project, '../../shared/UsageWidgets/Sources/GtmuxUsageWidgetHost', %w[WidgetOwnerConnection.swift WidgetUsageRequest.swift], [app])
register(project, 'UsageWidgets', %w[UsageWidgetRefresh.swift UsageWidgetsModule.swift UsageWidgetsModule.m], [app])
register(project, 'UsageWidgets', %w[UsageWidgetDefinition.swift], [widget])
widget.build_configurations.each { |c| c.build_settings['CODE_SIGN_ENTITLEMENTS'] = 'GtmuxWidget/GtmuxWidget.entitlements' }
group = project.main_group['GtmuxWidget']
resource = group.files.find { |f| f.path == 'Localizable.xcstrings' } || group.new_reference('Localizable.xcstrings')
widget.resources_build_phase.add_file_reference(resource) unless widget.resources_build_phase.files.any? { |b| b.file_ref == resource }
project.save
