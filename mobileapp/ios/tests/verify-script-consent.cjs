// eslint-env node
const {createRequire}=require('module'),fs=require('fs'),path=require('path');
const [root, simulator, appiumPort, wdaPort, wdaDerived] = process.argv.slice(2);
if (!root || !simulator || !appiumPort || !wdaPort || !wdaDerived) throw new Error('Usage: node verify-script-consent.cjs OUTPUT SIMULATOR_ID APPIUM_PORT WDA_PORT WDA_BUILD_DIR');
const {remote}=createRequire(path.resolve(__dirname, '../../package.json'))('webdriverio');
const packets=()=>JSON.parse(fs.readFileSync(root+'/packets.json')).length;
const assert=(value,message)=>{if(!value)throw new Error(message)};
(async()=>{let d;try{
 d=await remote({hostname:'127.0.0.1',port:Number(appiumPort),logLevel:'error',connectionRetryCount:0,connectionRetryTimeout:180000,capabilities:{platformName:'iOS','appium:platformVersion':process.env.GTMUX_E2E_OS || '26.5','appium:automationName':'XCUITest','appium:udid':simulator,'appium:bundleId':'com.gtmux.conversation-fixture','appium:noReset':true,'appium:wdaLocalPort':Number(wdaPort),'appium:derivedDataPath':wdaDerived,'appium:usePrebuiltWDA':true,'appium:newCommandTimeout':120}});
 let source=await d.getPageSource();fs.writeFileSync(root+'/consent-static.xml',source);await d.saveScreenshot(root+'/consent-static.png');
 assert(source.includes('静态预览')&&await d.$('~LOADING').isDisplayed(),'File script ran before consent');assert(packets()===0,'UDP emitted before consent');
 await d.$('~document-preview-interaction').click();fs.writeFileSync(root+'/consent-alert.xml',await d.getPageSource());await d.saveScreenshot(root+'/consent-alert.png');await d.$('~取消').click();
 assert(await d.$('~LOADING').isDisplayed(),'Cancel enabled scripts');assert(packets()===0,'Cancel emitted UDP');
 await d.$('~document-preview-interaction').click();await d.getPageSource();await d.$('~document-preview-enable-scripts').click();
 await d.waitUntil(async()=> await d.$('~INLINE_OK').isDisplayed().catch(()=>false),{timeout:10000});await d.waitUntil(()=>packets()>0,{timeout:10000});
 const allowed=packets();await d.saveScreenshot(root+'/consent-interactive.png');
 await d.$('~document-preview-interaction').click();await d.waitUntil(async()=> await d.$('~LOADING').isDisplayed().catch(()=>false),{timeout:10000});await d.pause(1500);const stopped=packets();await d.pause(4500);assert(packets()===stopped,'UDP continued after disabling interaction');
 await d.$('~document-preview-close').click();await d.$('~Open report / 打开报告').click();await d.waitUntil(async()=> (await d.getPageSource()).includes('静态预览'),{timeout:10000});await d.waitUntil(async()=>await d.$('~LOADING').isDisplayed().catch(()=>false),{timeout:10000});await d.pause(2000);assert(packets()===stopped,'New viewer emitted UDP');
 fs.writeFileSync(root+'/consent-result.json',JSON.stringify({default:'static',beforeConsent:0,cancel:0,afterConsent:allowed,afterDisable:stopped,newViewer:'static',ignoredRequestFlag:true}));console.log('Consent boundary passed');
 }finally{if(d)await d.deleteSession()}})().catch(e=>{process.stderr.write(e.message + '\n');process.exitCode=1});
