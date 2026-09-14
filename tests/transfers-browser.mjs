// Browser regression with dummy data, no real RDP host.
import assert from 'node:assert/strict';
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'/tmp/connectme-browser-tools/node_modules/playwright/index.mjs');
const browser=await chromium.launch({headless:true});const page=await browser.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.message));page.on('dialog',dialog=>dialog.accept());
await page.goto('http://127.0.0.1:18080/');await page.locator('[name=email]').fill('admin');await page.locator('[name=password]').fill('Isolated-test-only-2026!');await page.getByRole('button',{name:'Entrar',exact:true}).click();await page.waitForURL('**/app');
const api=async(path,method='GET',body)=>page.evaluate(async({path,method,body})=>{const csrf=document.cookie.split('; ').find(x=>x.startsWith('connectme_csrf='))?.split('=')[1];const r=await fetch('/api/v1/'+path,{method,headers:{'Content-Type':'application/json','X-CSRF-Token':csrf},body:body===undefined?undefined:JSON.stringify(body)});if(!r.ok)throw Error('Fixture API '+r.status);return r.status===204?{}:r.json()},{path,method,body});
let loc,net,host,cred,conn;
try{
 loc=await api('locations','POST',{name:'browser-transfer-'+Date.now()});net=await api('locations/'+loc.id+'/networks','POST',{name:'doc',cidr:'192.0.2.0/24'});
 host=await api('hosts','POST',{name:'transfer-ui',location_id:loc.id,address:'192.0.2.20',operating_system:'windows'});
 cred=await api('credentials','POST',{name:'transfer-ui',type:'password',username:'dummy',value:'Dummy-test-only'});
 conn=await api('connections','POST',{name:'transfer-ui',host_id:host.id,credential_ref_id:cred.id,protocol:'rdp',port:3389,clipboard_enabled:true,clipboard_copy_enabled:true,clipboard_paste_enabled:true,file_transfer_enabled:true,file_upload_enabled:true,file_download_enabled:true});
 await page.getByRole('button',{name:'Conexões',exact:true}).click();await page.locator('tr').filter({hasText:'transfer-ui'}).getByRole('button',{name:'Editar',exact:true}).click();
 assert.equal(await page.locator('[name=file_upload_enabled]').inputValue(),'true');assert.equal(await page.locator('[name=clipboard_copy_enabled]').inputValue(),'true');
 await page.locator('tr').filter({hasText:'transfer-ui'}).getByRole('button',{name:'Abrir',exact:true}).click();
 await page.getByRole('button',{name:'Texto e arquivos',exact:true}).click();
 const bytes=Buffer.from([0,255,1,2,3,10,128]);
 await page.getByLabel('Selecionar arquivo para enviar').setInputFiles({name:'browser-test.bin',mimeType:'application/octet-stream',buffer:bytes});
 await page.getByRole('button',{name:'Enviar arquivo',exact:true}).click();
 await page.getByRole('button',{name:'Baixar browser-test.bin',exact:true}).waitFor();
 const pending=page.waitForEvent('download');await page.getByRole('button',{name:'Baixar browser-test.bin',exact:true}).click();
 const download=await pending;const stream=await download.createReadStream();const chunks=[];for await(const chunk of stream)chunks.push(chunk);assert.deepEqual(Buffer.concat(chunks),bytes);
 // Exercise the real Guacamole StringReader and UI without changing a user's clipboard.
 await page.evaluate(id=>{const entry=desktops.get(id);const stream={sendAck(){}};entry.client.onclipboard(stream,'text/plain');stream.onblob(btoa('Clipboard smoke text'));stream.onend()},conn.id);
 await page.waitForFunction(()=>document.querySelector('[aria-label="Texto recebido do remoto"]').value==='Clipboard smoke text');
 await page.getByRole('button',{name:'Selecionar texto recebido',exact:true}).click();
 assert.equal(await page.getByLabel('Texto recebido do remoto').evaluate(el=>el.selectionEnd-el.selectionStart),20);
 await page.getByRole('button',{name:'Encerrar',exact:true}).click();await page.waitForFunction(()=>document.querySelectorAll('.desktop-window').length===0);
 assert.equal(errors.length,0,errors.join('; '));
 console.log('PASS: permission editor, file chooser/upload/progress/download bytes, clipboard reader/manual selection, cleanup confirmation');
}finally{
 await page.goto('http://127.0.0.1:18080/app');
 for(const [kind,obj] of [['connections',conn],['credentials',cred],['hosts',host],['networks',net],['locations',loc]])if(obj)await api(kind+'/'+obj.id,'DELETE');await browser.close();
}
