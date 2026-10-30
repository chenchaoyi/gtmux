#!/usr/bin/env node
// The browser and native clients must compare exactly the same attachment forms.
const fs = require('node:fs');
const path = require('node:path');
const root = path.resolve(__dirname, '../..');
const source = fs.readFileSync(path.join(root, 'mobileapp/src/screens/promptIdentity.js'), 'utf8');
const target = path.join(root, 'internal/server/web/prompt-identity.js');
if (process.argv.includes('--check')) {
  if (!fs.existsSync(target) || fs.readFileSync(target, 'utf8') !== source) {
    console.error('Run node mobileapp/scripts/gen-prompt-identity-web.cjs'); process.exit(1);
  }
} else { fs.writeFileSync(target, source); }
