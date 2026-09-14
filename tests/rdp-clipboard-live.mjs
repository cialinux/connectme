// Explicit lab-only test: opens its own blank Notepad and a unique redirected file.
import assert from 'node:assert/strict';
import {randomUUID} from 'node:crypto';
if(process.env.CONNECTME_LIVE_RDP_TEST!=='yes')throw Error('Explicit CONNECTME_LIVE_RDP_TEST=yes required');
let password='';for await(const chunk of process.stdin)password+=chunk;password=password.trimEnd();if(!password)throw Error('Supply lab Windows password via stdin');
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'/tmp/connectme-browser-tools/node_modules/playwright/index.mjs');
const browser=await chromium.launch({headless:true});const context=await browser.newContext({permissions:['clipboard-read','clipboard-write']});const page=await context.newPage();
await page.goto('http://127.0.0.1:18080/');await page.locator('[name=email]').fill('admin');await page.locator('[name=password]').fill('Isolated-test-only-2026!');await page.getByRole('button',{name:'Entrar',exact:true}).click();await page.waitForURL('**/app');
const api=(path,method='GET',body)=>page.evaluate(async({path,method,body})=>{const csrf=document.cookie.split('; ').find(x=>x.startsWith('connectme_csrf='))?.split('=')[1];const r=await fetch('/api/v1/'+path,{method,headers:{'Content-Type':'application/json','X-CSRF-Token':csrf},body:body===undefined?undefined:JSON.stringify(body)});if(!r.ok)throw Error('Fixture API '+r.status);return r.status===204?{}:r.json()},{path,method,body});
const marker='ConnectMe-'+randomUUID(),filename=marker+'.txt';let loc,net,host,cred,conn;
const chord=keys=>page.evaluate(({id,keys})=>{const c=desktops.get(id).client;for(const k of keys)c.sendKeyEvent(1,k);for(const k of keys.toReversed())c.sendKeyEvent(0,k)},{id:conn.id,keys});
try{
 loc=await api('locations','POST',{name:marker});net=await api('locations/'+loc.id+'/networks','POST',{name:'lab',cidr:'192.168.1.200/32'});
 host=await api('hosts','POST',{name:marker,location_id:loc.id,address:'192.168.1.200',operating_system:'windows'});
 cred=await api('credentials','POST',{name:marker,type:'password',username:'admin',value:password});password='';
 conn=await api('connections','POST',{name:marker,host_id:host.id,credential_ref_id:cred.id,protocol:'rdp',port:3389,clipboard_enabled:true,clipboard_copy_enabled:true,clipboard_paste_enabled:true,file_transfer_enabled:true,file_upload_enabled:true,file_download_enabled:true});
 await page.evaluate(c=>openDesktop(c),conn);await page.waitForFunction(id=>{const d=desktops.get(id);return d?.connected&&d.client.getDisplay().getWidth()>0},conn.id);
 // Guacamole CONNECTED precedes the completed Windows desktop/logon sequence.
 await page.waitForTimeout(3000);
 await page.evaluate(async({id,filename,marker})=>{const d=desktops.get(id),csrf=document.cookie.split('; ').find(x=>x.startsWith('connectme_csrf='))?.split('=')[1];const r=await fetch('/api/v1/remote-sessions/'+d.id+'/files/'+filename,{method:'POST',headers:{'X-CSRF-Token':csrf},body:marker});if(r.status!==201)throw Error('Temporary file upload failed')},{id:conn.id,filename,marker});
 await chord([0xffeb,114]);await page.waitForTimeout(500);
 await chord([0xffe3,97]);
 await page.evaluate(({id,command})=>{const c=desktops.get(id).client;for(const ch of command){c.sendKeyEvent(1,ch.charCodeAt(0));c.sendKeyEvent(0,ch.charCodeAt(0))}c.sendKeyEvent(1,0xff0d);c.sendKeyEvent(0,0xff0d)},{id:conn.id,command:'notepad'});
 await page.waitForTimeout(2500);
 const replacement=marker+'-pasted';await page.evaluate(async({id,replacement})=>{await navigator.clipboard.writeText(replacement);desktops.get(id).screen.focus()},{id:conn.id,replacement});
 await page.keyboard.press('Control+v');await page.waitForFunction(()=>document.querySelector('.clipboard-notice')?.textContent.includes('atalho de colagem'));
 await page.waitForTimeout(1000);await chord([0xffe3,97]);await page.keyboard.press('Control+c');
 await page.waitForFunction(expected=>document.querySelector('[aria-label="Texto recebido do remoto"]').value===expected,replacement);
 assert.equal(await page.evaluate(()=>navigator.clipboard.readText()),replacement);
 // Save only our new document into the redirected drive, using clipboard for
 // the path to avoid keyboard-layout-dependent slash/backslash translation.
 const savedName=marker+'-saved.txt';
 await chord([0xffe3,0xffe1,83]);await page.waitForTimeout(1000);await chord([0xffe3,97]);
 await page.evaluate(async({id,path})=>{await navigator.clipboard.writeText(path);desktops.get(id).screen.focus()},{id:conn.id,path:'\\\\tsclient\\ConnectMe\\'+savedName});
 await page.keyboard.press('Control+v');await page.waitForTimeout(1000);await chord([0xff0d]);
 await page.waitForFunction(async({id,name,expected})=>{const r=await fetch('/api/v1/remote-sessions/'+desktops.get(id).id+'/files/'+name);return r.ok&&(await r.text()).replace(/^\uFEFF/,'')===expected},{id:conn.id,name:savedName,expected:replacement},{timeout:15000});
 // File bytes can be visible before Windows completes Close/Save As.
 await page.waitForTimeout(3000);
 await chord([0xffeb,114]);await page.waitForTimeout(1500);await chord([0xffe3,97]);
 await page.evaluate(async({id,path})=>{await navigator.clipboard.writeText(path);desktops.get(id).screen.focus()},{id:conn.id,path:'notepad "\\\\tsclient\\ConnectMe\\'+filename+'"'});
 await page.keyboard.press('Control+v');await page.waitForTimeout(1000);await chord([0xff0d]);await page.waitForTimeout(1000);
 await chord([0xffe3,97]);await page.keyboard.press('Control+c');
 await page.waitForFunction(expected=>document.querySelector('[aria-label="Texto recebido do remoto"]').value===expected,marker);
 const downloaded=await page.evaluate(async({id,filename})=>{const r=await fetch('/api/v1/remote-sessions/'+desktops.get(id).id+'/files/'+filename);if(!r.ok)throw Error('Download failed');return r.text()},{id:conn.id,filename});assert.equal(downloaded.replace(/^\uFEFF/,''),marker);
 await chord([0xffe3,119]);console.log('PASS: real Windows native text paste/copy, Windows read/write redirected files and exact file API round-trip verified');
}catch(error){
 if(process.env.CONNECTME_LIVE_SCREENSHOT==='yes')await page.screenshot({path:'/tmp/connectme-rdp-test-failure.png'});
 throw error;
}finally{
 await page.goto('http://127.0.0.1:18080/app');for(const [kind,obj] of [['connections',conn],['credentials',cred],['hosts',host],['networks',net],['locations',loc]])if(obj)await api(kind+'/'+obj.id,'DELETE');await browser.close();
}
