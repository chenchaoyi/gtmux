import {mkdir, writeFile, cp, rm} from 'node:fs/promises';
await rm(new URL('./dist', import.meta.url), {recursive: true, force: true});
await mkdir(new URL('./dist', import.meta.url), {recursive: true});
await cp(new URL('./src', import.meta.url), new URL('./dist/_worker.js', import.meta.url), {recursive: true});
await writeFile(new URL('./dist/_worker.js/index.js', import.meta.url), "export {default} from './service.mjs';\n");
await writeFile(new URL('./dist/_routes.json', import.meta.url), JSON.stringify({version: 1, include: ['/*'], exclude: []}));
