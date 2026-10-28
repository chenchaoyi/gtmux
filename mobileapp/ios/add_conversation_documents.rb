# Register the document adapter and TextKit conversation renderer. Idempotent.
require 'xcodeproj'
project = Xcodeproj::Project.open('GtmuxMobile.xcodeproj')
app = project.targets.find { |target| target.name == 'GtmuxMobile' }
raise 'GtmuxMobile target not found' unless app
{
  'ConversationText' => %w[ConversationTextView.swift ConversationTextManager.swift ConversationText.m],
  'Documents' => %w[DocumentTransport.swift DocumentMarkdown.swift DocumentResources.swift DocumentStatusView.swift DocumentPreviewController.swift GtmuxDocuments.swift GtmuxDocuments.m]
}.each do |directory, names|
  group = project.main_group[directory] || project.main_group.new_group(directory, directory)
  names.each do |name|
    ref = project.files.find { |file| file.parent == group && file.path == name } || group.new_reference(name)
    app.add_file_references([ref]) unless app.source_build_phase.files.any? { |build| build.file_ref == ref }
  end
end
project.save
