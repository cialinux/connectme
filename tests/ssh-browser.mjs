import assert from 'node:assert/strict';
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'/tmp/connectme-browser-tools/node_modules/playwright/index.mjs');
const address=process.env.SSH_FIXTURE_IP;
if(!address)throw Error('Set SSH_FIXTURE_IP and provision its trusted public key in the isolated config/ssh_known_hosts');
const browser=await chromium.launch({headless:true});const page=await browser.newPage();
const errors=[];page.on('pageerror',e=>errors.push(e.message));
await page.goto('http://127.0.0.1:18080/');await page.locator('[name=email]').fill('admin');await page.locator('[name=password]').fill('Isolated-test-only-2026!');await page.getByRole('button',{name:'Entrar',exact:true}).click();await page.waitForURL('**/app');
const api=(path,method='GET',body)=>page.evaluate(async({path,method,body})=>{const csrf=document.cookie.split('; ').find(x=>x.startsWith('connectme_csrf='))?.split('=')[1];const r=await fetch('/api/v1/'+path,{method,headers:{'Content-Type':'application/json','X-CSRF-Token':csrf},body:body===undefined?undefined:JSON.stringify(body)});if(!r.ok)throw Error('API '+r.status);return r.status===204?{}:r.json()},{path,method,body});
let loc,net,host,cred,conn;
try{
 loc=await api('locations','POST',{name:'ssh-fixture-'+Date.now()});net=await api('locations/'+loc.id+'/networks','POST',{name:'isolated',cidr:address+'/32'});
 host=await api('hosts','POST',{name:'ssh-fixture',location_id:loc.id,address,operating_system:'linux'});
 cred=await api('credentials','POST',{name:'ssh-fixture',type:'password',username:'fixture',value:'Fixture-only-2026!'});
 conn=await api('connections','POST',{name:'ssh-fixture',host_id:host.id,credential_ref_id:cred.id,protocol:'ssh',port:2222,clipboard_enabled:true,clipboard_copy_enabled:true,clipboard_paste_enabled:true,file_transfer_enabled:true,file_upload_enabled:true,file_download_enabled:true});
 await page.getByRole('button',{name:'Conexões',exact:true}).click();await page.locator('tr').filter({hasText:'ssh-fixture'}).getByRole('button',{name:'Abrir',exact:true}).click();
 if(process.env.SSH_EXPECT_REJECT==='true'){
 await page.waitForFunction(id=>desktops.get(id)?.disconnected,conn.id);
 console.log('PASS: SSH session rejected with incorrect host identity');
 }else{
 await page.waitForFunction(id=>{const d=desktops.get(id);return d?.connected&&d.client.getDisplay().getWidth()>0},conn.id);
 const before=await page.evaluate(id=>desktops.get(id).client.getDisplay().flatten().toDataURL(),conn.id);
 // The SSH fixture echoes bytes; it never launches a shell or OS commands.
 await page.context().grantPermissions(['clipboard-read','clipboard-write']);
 await page.evaluate(async id=>{await navigator.clipboard.writeText('SSH-paste-test');desktops.get(id).screen.focus()},conn.id);
 await page.keyboard.press('Control+v');
 await page.waitForFunction(()=>document.querySelector('.clipboard-notice')?.textContent.includes('atalho de colagem'));
 await page.waitForFunction(({id,before})=>desktops.get(id).client.getDisplay().flatten().toDataURL()!==before,{id:conn.id,before});
 // SSH overlay is editable and must leave native contextmenu unprevented.
 assert.equal(await page.evaluate(id=>{const sink=desktops.get(id).screen.querySelector('[aria-label="Entrada de colagem da sessão"]');const e=new MouseEvent('contextmenu',{bubbles:true,cancelable:true,button:2});sink.dispatchEvent(e);return e.defaultPrevented},conn.id),false);
 await page.getByRole('button',{name:'Minimizar',exact:true}).click();await page.locator('#session-tabs button').click();
 await page.getByRole('button',{name:'Reconectar',exact:true}).click();await page.waitForFunction(id=>desktops.get(id)?.connected,conn.id);
 await page.getByRole('button',{name:'Encerrar',exact:true}).click();
 assert.equal(errors.length,0,errors.join('; '));console.log('PASS: verified SSH host, password login, terminal render/input echo, minimize/restore/reconnect/close');
 }
}finally{
 await page.goto('http://127.0.0.1:18080/app');for(const [kind,obj] of [['connections',conn],['credentials',cred],['hosts',host],['networks',net],['locations',loc]])if(obj)await api(kind+'/'+obj.id,'DELETE');await browser.close();
}
