/* eslint-env node */
// Own emulator + QA package only. Reuses the same synthetic server as iOS tests.
const fs = require('node:fs');
const {createHash} = require('node:crypto');
const path = require('node:path');
const assert = require('node:assert/strict');
const {execFileSync, spawn} = require('node:child_process');
const ts = require('typescript');
const {remote} = require('webdriverio');
require.extensions['.ts'] = (module, file) => module._compile(ts.transpileModule(fs.readFileSync(file, 'utf8'), {
  compilerOptions: {module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022, esModuleInterop: true},
}).outputText, file);
const {startFake} = require('../fake-serve/server.ts');
const root = path.resolve(__dirname, '../..');
const serial = process.env.ANDROID_SERIAL || '';
const sdk = process.env.ANDROID_HOME || process.env.ANDROID_SDK_ROOT;
const apk = path.resolve(process.env.GTMUX_ANDROID_APK || path.join(root, 'android/app/build/outputs/apk/qa/app-qa.apk'));
const output = path.resolve(process.env.GTMUX_ANDROID_E2E_OUTPUT || path.join(root, '.e2e-artifacts', 'android-' + Date.now()));
const app = 'com.gtmux.app.qa';
const port = Number(process.env.GTMUX_ANDROID_APPIUM_PORT || 14823);
const results = [];
const adb = (...args) => execFileSync(path.join(sdk, 'platform-tools/adb'), ['-s', serial, ...args], {encoding:'utf8', timeout:30000});
const wait = ms => new Promise(resolve => setTimeout(resolve, ms));
let driver, appium, owner, guest;
const id = value => driver.$(`//*[@resource-id="${value}"]`);
const label = value => driver.$(`~${value}`);
async function shown(element) { return element.isDisplayed().catch(() => false); }
async function tap(element) { await element.waitForDisplayed({timeout:15000}); await element.click(); await wait(300); }
// Android exposes a whole paragraph as a text node; its center can lie beyond a short link span.
async function tapLink(text) {
  const element=driver.$('//*[contains(@text,"'+text+'") ]');await element.waitForDisplayed({timeout:15000});
  const [position,size]=await Promise.all([element.getLocation(),element.getSize()]);const rect={...position,...size};await driver.execute('mobile: clickGesture',{x:Math.round(rect.x+Math.min(30,rect.width/4)),y:Math.round(rect.y+rect.height/2)});await wait(400);
}
async function source() { return driver.getPageSource(); }
async function capture(name) {
  await driver.saveScreenshot(path.join(output,name+'.png'));
  if(name.endsWith('-failed')) {const fatal=adb('logcat','-d','-t','1500','-v','brief','-s','AndroidRuntime');fs.writeFileSync(path.join(output,name+'-crash.log'),fatal);if(fatal.includes('FATAL EXCEPTION')) console.error(fatal.slice(-12000));}
  fs.writeFileSync(path.join(output,name+'.xml'), await source());
}
async function launch(extra = {}, fixture = owner) {
  await driver.terminateApp(app).catch(() => {});
  const flags = {GTMUX_DEBUG_PAIR_URL:`http://10.0.2.2:${fixture.port}`, GTMUX_DEBUG_PAIR_TOKEN:fixture.token,
    GTMUX_DEBUG_PAIR_NAME:'Test host', GTMUX_DEBUG_NO_PUSH:'1', GTMUX_DEBUG_SKIP_WHATS_NEW:'1', GTMUX_DEBUG_LANG:'en',
    GTMUX_DEBUG_RESET_SERVERS:'1', GTMUX_DEBUG_RESET_UI_STATE:String(Date.now()), ...extra};
  adb('shell','run-as',app,'mkdir','-p','files');
  execFileSync(path.join(sdk,'platform-tools/adb'),['-s',serial,'shell','run-as',app,'tee','files/gtmux-debug-flags.json'],{input:JSON.stringify(flags),timeout:10000});
  await driver.activateApp(app);
  if(extra.GTMUX_DEBUG_PAIR_URL === '') { await id('pairing-screen').waitForDisplayed({timeout:30000});return; }
  await driver.waitUntil(async () => shown(id('radar-screen')).then(async ok => ok || await shown(id('radar-split'))),{timeout:30000,timeoutMsg:'Radar did not load'});
}
async function seek(element, direction='down', scrollID) {
  const rect=await driver.getWindowRect();
  for(let i=0;i<24;i++) {if(await shown(element)) return;await driver.execute('mobile: scrollGesture',scrollID?{elementId:(await id(scrollID)).elementId,direction,percent:.9}:{left:Math.round(rect.width*.08),top:Math.round(rect.height*.25),width:Math.round(rect.width*.84),height:Math.round(rect.height*.58),direction,percent:.8});}
  await element.waitForDisplayed({timeout:5000});
}
async function open(pane = '%12') { if(!(pane==='%12' && await shown(id('detail-mode-chat')) && (await source()).includes('knowledge on the phone'))) await tap(id('agent-row-'+pane));await id('detail-mode-chat').waitForDisplayed({timeout:15000}); }
async function tools(name) { await tap(id('session-tools')); await tap(label(name)); }
async function check(name, action) {
  const only=process.env.GTMUX_ANDROID_E2E_ONLY?.split(',');
  if(only && !only.includes(name.slice(0,2))) {results.push({name,status:'not-run',reason:'Explicit local subset'});return;}
  const at = Date.now(); console.log('Start',name);
  try { await action(); await capture(name); results.push({name,status:'passed',seconds:(Date.now()-at)/1000}); }
  catch (error) { await capture(name+'-failed').catch(() => {}); await driver.hideKeyboard().catch(()=>{}); results.push({name,status:'failed',error:String(error.message),seconds:(Date.now()-at)/1000}); }
  fs.writeFileSync(path.join(output,'RESULTS.json'),JSON.stringify({serial,package:app,results},null,2)+'\n');
}
// Keep the process lifecycle in one finally: no orphan Appium or fixture service.
async function run() {
  if (!sdk || !/^emulator-\d+$/.test(serial)) throw new Error('Explicit owned emulator required');
  const avd = adb('emu','avd','name').split('\n')[0].trim();
  assert.match(avd,/^gtmux-(qa-|ci)/);
  assert.ok(fs.existsSync(apk),'Build the QA APK first');
  const badging = execFileSync(path.join(sdk,'build-tools/36.0.0/aapt'),['dump','badging',apk],{encoding:'utf8'});
  assert.ok(badging.includes("package: name='"+app+"'"),'Refuse any production or unrelated package');
  fs.mkdirSync(output,{recursive:true,mode:0o700});
  fs.writeFileSync(path.join(output,'APK.json'),JSON.stringify({file:path.basename(apk),bytes:fs.statSync(apk).size,sha256:createHash('sha256').update(fs.readFileSync(apk)).digest('hex')},null,2));
  adb('install','-r',apk);
  // Only this isolated QA app's synthetic data is reset.
  adb('shell','pm','clear',app);
  owner = await startFake(); guest = await startFake({guest:true});
  owner.world.agent('%12').session_id = 'synthetic-android-session';
  owner.world.agent('%12').home_id = 'a'.repeat(32);
  const nestedCall = 'await tools.exec_command({cmd:' + JSON.stringify("python3 - <<'PY'\nfrom pathlib import Path\na=Path(\"/example\");b={\"count\":2}\nprint(b)\nPY") + '});';
  const turns = Array.from({length:10},(_,index) => ({turn_ordinal:151+index, record_id:'synthetic-'+(151+index), session_id:'synthetic-android-session',
    prompt:'Android trace request '+(151+index),response:'Android trace response '+(151+index),time:new Date(Date.now()-(10-index)*60000).toISOString(),
    segments:[{text:'Recorded response '+(151+index)},{steps:[{kind:'tool',title:'Run code',operation:'code',input_kind:'code',input:nestedCall},{kind:'tool',title:'Wait',operation:'wait',input_kind:'json',input:'{"session_id":42,"yield_time_ms":1000}'}]}],
    ...(index === 0 ? {history_before:'synthetic-before-151'} : {}),
  }));
  const folder='/tmp/gtmux-android-reader';
  const doc=(name,kind,media,body)=>owner.world.documents.set(folder+'/'+name,{kind,media,body:Buffer.isBuffer(body)?body:Buffer.from(body),session:'synthetic-android-session',home:'a'.repeat(32),id:createHash('md5').update(name).digest('hex')});
  doc('report.html','html','text/html','<!doctype html><html><head><title>Android QA</title></head><body><h1>Android document preview</h1><p id="state">Static report is readable</p><img src="logo.png" alt="Synthetic local image"><script>document.getElementById("state").textContent="Interactive code ran";</script></body></html>');
  doc('report.md','markdown','text/markdown','# Android Markdown reader\n\n**Shared styling** with a local image.\n\n![Local image](logo.png)\n\n| Name | Value |\n| --- | --- |\n| Synthetic | 42 |\n\n```python\nprint(42)\n```');
  doc('logo.png','image','image/png',fs.readFileSync(path.join(root,'android/app/src/main/res/drawable-nodpi/brand.png')));
  const links=`\n\n[Android HTML report](${folder}/report.html)\n\n[Android Markdown report](${folder}/report.md)\n\n[Android image](${folder}/logo.png)`;
  turns[9].response+=links;turns[9].segments[0].text+=links;
  owner.world.transcripts.set('%12',turns);
  owner.world.diffs.set('%12','diff --git a/example.py b/example.py\n--- a/example.py\n+++ b/example.py\n@@ -1 +1,2 @@\n-print(1)\n+print(42)\n+print(\"Android synthetic diff\")\n');
  owner.world.historyPages.set('%12:synthetic-before-151',[{turn_ordinal:150,record_id:'synthetic-150',session_id:'synthetic-android-session',prompt:'Earlier request 150',response:'Earlier response 150'}]);
  const log = fs.openSync(path.join(output,'appium.log'),'w');
  appium = spawn(process.execPath,[path.join(root,'node_modules/appium/build/lib/main.js'),'--address','127.0.0.1','--port',String(port),'--log-level','info'],{cwd:root,env:process.env,stdio:['ignore',log,log],detached:true});
  fs.closeSync(log);
  await new Promise(async (resolve,reject) => {
    const until=Date.now()+60000;
    while(Date.now()<until) {try {if((await fetch(`http://127.0.0.1:${port}/status`)).ok){resolve();return;}}catch{} await wait(300);}
    reject(new Error('Appium did not start'));
  });
  driver = await remote({hostname:'127.0.0.1',port,path:'/',logLevel:'error',waitforTimeout:1000,waitforInterval:200,connectionRetryTimeout:180000,connectionRetryCount:0,capabilities:{
    platformName:'Android','appium:automationName':'UiAutomator2','appium:udid':serial,'appium:appPackage':app,'appium:appActivity':'com.gtmux.app.MainActivity',
    'appium:settings[waitForIdleTimeout]':0,'appium:noReset':true,'appium:autoGrantPermissions':false,'appium:newCommandTimeout':120,'appium:systemPort':Number(process.env.GTMUX_ANDROID_UIAUTOMATOR_PORT || 18201),
  }});
  fs.writeFileSync(path.join(output,'SESSION.json'),JSON.stringify({id:driver.sessionId,serial,package:app}));
  await driver.updateSettings({waitForIdleTimeout:0,waitForSelectorTimeout:1000});
  await driver.setOrientation(avd.includes('tablet')?'LANDSCAPE':'PORTRAIT');adb('shell','settings','put','system','font_scale','1.0');
  await check('01-radar-owner',async()=>{await launch();await id('agent-row-%12').waitForDisplayed({timeout:15000});assert.ok((await source()).includes('gtmux dev'));});
  await check('02-section-collapse',async()=>{const section=id('radar-section-working');await tap(section);assert.equal(await shown(id('agent-row-%12')),false);await tap(section);await id('agent-row-%12').waitForDisplayed({timeout:10000});});
  await check('03-chat-terminal',async()=>{await open();await tap(id('detail-mode-terminal'));try {await id('detail-term-rows').waitForExist({timeout:10000});await capture('03-terminal-readable');} finally {await tap(id('detail-mode-chat'));await id('detail-chat').waitForDisplayed({timeout:15000});}});
  await check('04-real-composer-send',async()=>{await tap(id('composer-kbd'));const input=id('composer-input');await tap(input);await input.setValue('Android synthetic message');await tap(id('composer-send'));await driver.waitUntil(async()=>owner.world.writesTo('/api/send').some(row=>row.id==='%12'&&row.text==='Android synthetic message'),{timeout:10000});await driver.hideKeyboard().catch(()=>{});});
  await check('05-display-size',async()=>{await tap(id('detail-chat-display'));await tap(id('detail-chat-bigger'));await tap(id('detail-chat-smaller'));await driver.back();});
  await check('06-trace-session-and-turns',async()=>{await tools('Trace');await id('trace-session-title').waitForDisplayed({timeout:15000});assert.ok((await source()).includes('knowledge on the phone'));assert.ok((await source()).includes('151'));});
  await check('07-trace-expand-collapse',async()=>{await tap(id('trace-fold-turns'));assert.ok((await source()).includes('Android trace'));await tap(id('trace-fold-calls'));await tap(id('trace-fold-calls'));});
  await check('08-trace-earlier',async()=>{await tap(id('trace-fold-turns'));const earlier=id('trace-history-earlier');await tap(earlier);await driver.waitUntil(async()=>owner.world.writesTo('/api/transcript/history').length===1,{timeout:10000});await seek(driver.$('//*[contains(@text,\"Earlier request 150\")]'),'up','trace-list');});
  await check('09-trace-search',async()=>{await tap(id('trace-search-toggle'));await id('trace-search').setValue('response 160');assert.ok((await source()).includes('160'));await driver.hideKeyboard().catch(()=>{});await id('trace-search').clearValue();await tap(id('trace-close'));});
  await check('10-diff-wrap-and-back',async()=>{await tools('Diff');await id('diff-wrap').waitForDisplayed({timeout:10000});await tap(id('diff-wrap'));await tap(id('diff-wrap'));await driver.back();assert.equal(await shown(id('diff-wrap')),false);});
  await check('11-fullscreen-tools',async()=>{await launch();await open();await tap(id('detail-mode-chat'));await id('detail-chat').waitForDisplayed({timeout:15000});for(let cycle=0;cycle<3;cycle++){await tap(id('detail-fullscreen'));await id('detail-fs-exit').waitForDisplayed({timeout:10000});assert.equal(await driver.getCurrentPackage(),app);await tap(id('session-tool-trace'));await id('trace-session-title').waitForDisplayed({timeout:10000});await tap(id('trace-close'));await tap(id('detail-fs-exit'));}});
  await check('12-offline-reconnect',async()=>{const streams=owner.dropStreams();assert.ok(streams>=1);await driver.waitUntil(async()=>owner.activeStreams()>0,{timeout:15000});owner.bumpAgents();assert.ok((await source()).includes('knowledge on the phone'));});
  await check('13-guest-scope',async()=>{await launch({},guest);assert.equal(await shown(id('agent-row-%11')),false);assert.equal(await shown(id('agent-row-%6')),false);await id('agent-row-%12').waitForDisplayed({timeout:15000});});
  await check('14-owner-reconnect-persisted',async()=>{await launch({GTMUX_DEBUG_PAIR_URL:'',GTMUX_DEBUG_PAIR_TOKEN:''});await id('pairing-host').setValue(`http://10.0.2.2:${owner.port}`);await id('pairing-token').setValue(owner.token);await tap(id('pairing-connect'));await id('agent-row-%12').waitForDisplayed({timeout:20000});await driver.hideKeyboard().catch(()=>{});adb('shell','run-as',app,'rm','files/gtmux-debug-flags.json');await driver.terminateApp(app);await driver.activateApp(app);await id('agent-row-%12').waitForDisplayed({timeout:25000});});
  await check('15-settings-and-honest-push',async()=>{await tap(id('radar-settings'));const status=driver.$('//*[contains(@text,\"Android push is not configured\")]');await seek(status);assert.ok(await shown(status));});
  await check('16-html-native-local-image-and-consent',async()=>{
    await launch();await open();await tap(id('detail-mode-chat'));await id('detail-chat').waitForDisplayed({timeout:15000});
    await tapLink('Android HTML report');
    await driver.waitUntil(async()=>(await source()).includes('Android document preview'),{timeout:15000});
    assert.ok((await source()).includes('Static report is readable'));assert.equal((await source()).includes('Interactive code ran'),false);
    await driver.waitUntil(async()=>owner.world.writesTo('/api/artifacts/resolve').some(row=>row.reference==='logo.png'&&row.parent_id===createHash('md5').update('report.html').digest('hex')),{timeout:15000});
    await tap(driver.$('//*[@text="Interact" or @text="INTERACT"]'));await tap(driver.$('//*[@text="Enable" or @text="ENABLE"]'));
    await driver.waitUntil(async()=>(await source()).includes('Interactive code ran'),{timeout:10000});
    await capture('16-html-interactive');await driver.back();await id('detail-mode-chat').waitForDisplayed({timeout:10000});
  });
  await check('17-markdown-native-share-and-back',async()=>{
    await tapLink('Android Markdown report');
    await driver.waitUntil(async()=>(await source()).includes('Android Markdown reader'),{timeout:15000});
    await tap(label('Share document'));assert.ok((await source()).length>0);await driver.back();await tap(label('Close document'));
    const exports=adb('shell','run-as',app,'find','cache/share-exports','-type','f');assert.ok(exports.includes('report.md'));
    await id('detail-mode-chat').waitForDisplayed({timeout:10000});
  });
  await check('18-image-native-zoom-and-back',async()=>{
    await launch();await open();await tap(id('detail-mode-chat'));await id('detail-chat').waitForDisplayed({timeout:15000});
    await tapLink('Android image');await label('Share document').waitForDisplayed({timeout:15000});
    const rect=await driver.getWindowRect();await driver.execute('mobile: pinchOpenGesture',{left:100,top:250,width:rect.width-200,height:rect.height-500,percent:.6});
    await capture('18-image-zoomed');await driver.back();await id('detail-mode-chat').waitForDisplayed({timeout:10000});
  });
  await check('19-trace-nested-code',async()=>{
    await launch();await open();await tools('Trace');await tap(id('trace-search-toggle'));await id('trace-search').setValue('python3');await driver.hideKeyboard().catch(()=>{});
    await tap(driver.$('//*[starts-with(@resource-id,"trace-record-")]'));
    await id('trace-code-panel').waitForDisplayed({timeout:10000});await tap(id('trace-code-part-cmd 1:0'));
    assert.ok((await source()).includes('Python'));await tap(id('trace-code-format'));await tap(id('trace-code-format'));
    await tap(id('trace-code-open'));await id('code-reader-close').waitForDisplayed({timeout:10000});await tap(id('code-reader-close'));
    await tap(id('trace-code-part-original'));if(await shown(id('trace-detail-close'))) await tap(id('trace-detail-close'));await tap(id('trace-close'));
  });
  await check('20-file-picker-and-cancel',async()=>{
    await launch();await open();await tap(id('composer-kbd'));await driver.hideKeyboard().catch(()=>{});await tap(id('composer-attach'));await tap(id('attach-2'));
    await driver.waitUntil(async()=>['com.google.android.documentsui','com.android.documentsui'].includes(await driver.getCurrentPackage()),{timeout:15000});
    await capture('20-system-file-picker');await wait(700);await driver.pressKeyCode(4);await wait(700);if(await driver.getCurrentPackage() !== app) {await driver.pressKeyCode(4);await wait(500);}await id('composer-input').waitForDisplayed({timeout:10000});
  });
  await check('21-camera-permission-denial',async()=>{
    await launch();await open();await tap(id('composer-kbd'));await driver.hideKeyboard().catch(()=>{});await tap(id('composer-attach'));await tap(id('attach-1'));const deny=driver.$('//*[@resource-id="com.android.permissioncontroller:id/permission_deny_button"]');await tap(deny);
    await driver.waitUntil(async()=>(await source()).includes('Camera access is off'),{timeout:10000});
  });
  await check('22-rotation-and-large-text',async()=>{
    const normal=avd.includes('tablet')?'LANDSCAPE':'PORTRAIT';
    try {
      await launch();await driver.setOrientation(normal==='PORTRAIT'?'LANDSCAPE':'PORTRAIT');await wait(900);await open();await tap(id('detail-mode-chat'));await id('detail-chat').waitForDisplayed({timeout:10000});await capture('22-rotated');
      adb('shell','settings','put','system','font_scale','1.3');await wait(1200);await id('detail-chat').waitForDisplayed({timeout:10000});await capture('22-large-text');
    } finally {adb('shell','settings','put','system','font_scale','1.0');await driver.setOrientation(normal);await wait(700);}
  });
  assert.ok(results.length>=22);
  if(results.some(row=>row.status==='failed')) throw new Error(`${results.filter(row=>row.status==='failed').length} Android flows failed; inspect ${output}`);
}
let closing = false;
async function cleanup() {
  if (closing) return; closing = true;
  await driver?.deleteSession().catch(()=>{});await owner?.close().catch(()=>{});await guest?.close().catch(()=>{});
  if(appium?.pid) {try{process.kill(appium.pid,'SIGTERM');}catch{}}
}
for (const signal of ['SIGINT','SIGTERM']) process.once(signal,()=>{void cleanup().finally(()=>process.exit(130));});
run().catch(error=>{console.error(error.message);process.exitCode=1;}).finally(cleanup);
