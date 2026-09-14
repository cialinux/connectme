import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'/tmp/connectme-browser-tools/node_modules/playwright/index.mjs');
const browser=await chromium.launch({headless:true});const context=await browser.newContext();
await context.grantPermissions(['clipboard-read','clipboard-write'],{origin:'http://localhost:18999'});
await context.route('http://localhost:18999/**',r=>r.fulfill({contentType:'text/html',body:'<div id="pane"><div id="screen" tabindex="0">Remote screen</div></div><input id="local">'}));
const page=await context.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.message));
try{
 await page.goto('http://localhost:18999/');
 await page.addScriptTag({content:await readFile(new URL('../modules/identity/clipboard.js',import.meta.url),'utf8')});
 await page.evaluate(()=>{
  window.calls=[];window.releaseKeys=()=>{};
  window.entry={pane:document.querySelector('#pane'),screen:document.querySelector('#screen'),connected:true,connection:{protocol:'rdp'},client:{sendKeyEvent:(...args)=>calls.push(['key',...args])}};
  window.activeDesktop=entry;
  // Reproduce Guacamole's earlier document-capture keyboard listener.
  document.addEventListener('keydown',e=>{if(entry.screen.contains(document.activeElement)&&e.ctrlKey&&e.key.toLowerCase()==='v'){window.prematureCapture=true;e.preventDefault()}},true);
  window.direct=installDirectClipboard(entry,{copy:true,paste:true,upload:true},{text:async value=>{calls.push(['text',value])},files:async files=>calls.push(['files',...files.map(f=>({name:f.name,size:f.size,type:f.type}))])});
  entry.screen.focus();
 });
 // Real Chromium clipboard + native keyboard paste, not a synthetic paste event.
 await page.evaluate(()=>navigator.clipboard.writeText('Native clipboard test'));
 await page.keyboard.press('Control+v');
 await page.waitForFunction(()=>calls.some(x=>x[0]==='text'));
 assert.deepEqual(await page.evaluate(()=>calls[0]),['text','Native clipboard test']);
 assert.equal(await page.evaluate(()=>window.prematureCapture),undefined);
 await page.waitForFunction(()=>calls.filter(x=>x[0]==='key').length===4);
 assert.equal(await page.evaluate(()=>calls.filter(x=>x[0]==='key').length),4);
 await page.evaluate(()=>{entry.screen.focus();calls=[]});await page.keyboard.press('Control+c');
 await page.evaluate(()=>direct.received('Remote text'));
 assert.equal(await page.evaluate(()=>navigator.clipboard.readText()),'Remote text');
 // A background/unsolicited stream must not overwrite the OS clipboard.
 await page.evaluate(()=>direct.received('Unsolicited text'));
 assert.equal(await page.evaluate(()=>navigator.clipboard.readText()),'Remote text');
 await page.evaluate(()=>{const dt=new DataTransfer();dt.items.add(new File(['png'],'image.png',{type:'image/png'}));entry.screen.dispatchEvent(new ClipboardEvent('paste',{clipboardData:dt,bubbles:true,cancelable:true}))});
 await page.waitForFunction(()=>calls.some(x=>x[0]==='files'));
 assert.equal(await page.evaluate(()=>calls.find(x=>x[0]==='files')[1].type),'image/png');
 await page.locator('#local').focus();await page.evaluate(()=>calls=[]);await page.keyboard.press('Control+v');
 assert.equal(await page.locator('#local').inputValue(),'Remote text');assert.equal(await page.evaluate(()=>calls.length),0);
 // Closing removes listeners and prevents clipboard traffic.
 await page.evaluate(()=>{direct.close();entry.screen.focus()});await page.keyboard.press('Control+v');assert.equal(await page.evaluate(()=>calls.length),0);
 // Permission denial must not fall back to forwarding a remote paste shortcut.
 await page.evaluate(()=>{direct=installDirectClipboard(entry,{copy:false,paste:false,upload:false},{text:async v=>calls.push(['denied-text',v]),files:async()=>calls.push(['denied-file'])});entry.screen.focus()});
 await page.keyboard.press('Control+v');assert.equal(await page.evaluate(()=>calls.length),0);
 await page.evaluate(()=>direct.close());
 // A focus switch while the audit/request is pending must cancel remote keys.
 await page.evaluate(()=>{direct=installDirectClipboard(entry,{copy:true,paste:true,upload:true},{text:()=>new Promise(resolve=>window.finishPaste=resolve),files:async()=>{}});entry.screen.focus()});
 await page.keyboard.press('Control+v');await page.waitForFunction(()=>typeof finishPaste==='function');
 await page.locator('#local').focus();await page.evaluate(()=>finishPaste());
 await page.waitForTimeout(350);
 assert.equal(await page.evaluate(()=>calls.length),0);await page.evaluate(()=>direct.close());
 assert.deepEqual(errors,[]);console.log('PASS: native Ctrl+V, remote copy gesture, unsolicited/background isolation, image file routing, cleanup');
}finally{await browser.close()}
