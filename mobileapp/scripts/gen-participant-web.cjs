// Browser identity copy and fact ordering share the native App's source of truth.
const fs = require('node:fs'), path = require('node:path');
const root = path.resolve(__dirname, '../..');
const source = JSON.parse(fs.readFileSync(path.join(root, 'mobileapp/src/ui/participantPresentation.json'), 'utf8'));
const content = '// Generated from mobileapp/src/ui/participantPresentation.json; do not edit.\nwindow.GtmuxParticipantPresentation = ' + JSON.stringify(source) + ';\n';
const target = path.join(root, 'internal/server/web/participant-presentation.js');
if (process.argv.includes('--check')) { if (fs.readFileSync(target, 'utf8') !== content) throw Error('Run node mobileapp/scripts/gen-participant-web.cjs'); }
else fs.writeFileSync(target, content);
