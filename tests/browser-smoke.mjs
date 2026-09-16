// Requires Playwright. Run ONLY against disposable localhost:18080 stack.
import assert from 'node:assert/strict';
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'/tmp/connectme-browser-tools/node_modules/playwright/index.mjs');
const browser=await chromium.launch({headless:true});
const page=await browser.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.message));
let frames=0;const socketErrors=[];
page.on('websocket',socket=>{socket.on('framereceived',()=>frames++);socket.on('socketerror',e=>socketErrors.push(String(e)))});
await page.goto('http://127.0.0.1:18080/');
await page.locator('[name=email]').fill('admin');await page.locator('[name=password]').fill('Isolated-test-only-2026!');
await page.getByRole('button',{name:'Entrar',exact:true}).click();await page.waitForURL('**/app');
const api=async(path,method='GET',body)=>page.evaluate(async({path,method,body})=>{
 const csrf=document.cookie.split('; ').find(x=>x.startsWith('connectme_csrf='))?.split('=')[1];
 const r=await fetch('/api/v1/'+path,{method,headers:{'Content-Type':'application/json','X-CSRF-Token':csrf},body:body===undefined?undefined:JSON.stringify(body)});
 if(!r.ok)throw Error('Fixture API failed '+r.status);return r.status===204?{}:r.json();
},{path,method,body});
let loc,host,cred,conn,conn2,n1,n2;
try {
 loc=await api('locations','POST',{name:'browser-'+Date.now()});
 n1=await api('locations/'+loc.id+'/networks','POST',{name:'lan',cidr:'192.0.2.0/24'});
 n2=await api('locations/'+loc.id+'/networks','POST',{name:'documentation-only',cidr:'198.51.100.0/24'});
 host=await api('hosts','POST',{name:'browser-host',location_id:loc.id,address:'192.0.2.246',operating_system:'windows'});
 await page.getByRole('button',{name:'Hosts',exact:true}).click();
 const row=page.locator('tr').filter({hasText:'browser-host'});await row.getByRole('button',{name:'Editar',exact:true}).click();
 await page.locator('[name=address]').fill('192.0.2.200');await page.getByRole('button',{name:'Salvar alterações',exact:true}).click();
 await page.waitForFunction(()=>document.querySelector('#rows')?.textContent.includes('192.0.2.200'));
 assert.equal((await api('hosts')).items.find(x=>x.id===host.id).address,'192.0.2.200');
 // Never attempt dummy credentials against the user's Windows machine.
 await api('hosts/'+host.id,'PUT',{...host,address:'192.0.2.200'});
 cred=await api('credentials','POST',{name:'browser-credential',type:'password',username:'dummy-user',value:'Dummy-browser-test'});
 conn=await api('connections','POST',{name:'browser-rdp',host_id:host.id,credential_ref_id:cred.id,protocol:'rdp',port:3389});
 await page.getByRole('button',{name:'Conexões',exact:true}).click();
 await page.evaluate(()=>localStorage.setItem('GUAC_AUTH',JSON.stringify({authToken:'stale-token-from-old-ui'})));
 const auth=page.waitForResponse(r=>r.url().endsWith('/open')&&r.request().method()==='POST',{timeout:20000});
 await page.locator('tr').filter({hasText:'browser-rdp'}).getByRole('button',{name:'Abrir',exact:true}).click();
 assert.equal((await auth).status(),200);
 await page.waitForTimeout(3000);
 assert.equal(errors.length,0,errors.join('; '));
 assert.equal(socketErrors.length,0,'WebSocket transport failed');
 assert.ok(frames>0,'No Guacamole WebSocket frames received');
 
 assert.ok(page.url().endsWith('/app'));
 const pane=page.locator('.desktop-window');
 await pane.getByRole('button',{name:'Minimizar',exact:true}).click();assert.equal(await pane.isVisible(),false);
 await page.locator('#session-tabs').getByRole('button',{name:'browser-rdp',exact:true}).click();assert.equal(await pane.isVisible(),true);
 await pane.getByRole('button',{name:'Reconectar',exact:true}).click();
 await page.waitForTimeout(2000);
 await page.locator('.desktop-window').getByRole('button',{name:'Encerrar',exact:true}).click();
 await page.waitForFunction(()=>document.querySelectorAll('.desktop-window').length===0);
 await page.locator('tr').filter({hasText:'browser-rdp'}).getByRole('button',{name:'Abrir',exact:true}).click();
 await page.waitForFunction(()=>document.querySelectorAll('.desktop-window').length===1);
 await page.locator('.desktop-window').getByRole('button',{name:'Minimizar',exact:true}).click();
 conn2=await api('connections','POST',{name:'browser-secondary',host_id:host.id,credential_ref_id:cred.id,protocol:'rdp',port:3389});
 await page.getByRole('button',{name:'Atualizar',exact:true}).click();
 await page.locator('tr').filter({hasText:'browser-secondary'}).getByRole('button',{name:'Abrir',exact:true}).click();
 await page.waitForFunction(()=>document.querySelectorAll('.desktop-window').length===2);
 await page.locator('.desktop-window:not([hidden])').getByRole('button',{name:'Encerrar',exact:true}).click();
 await page.waitForFunction(()=>document.querySelectorAll('.desktop-window').length===1);
 await page.goto('http://127.0.0.1:18080/livez');await page.goBack();
 await page.getByRole('button',{name:'Conexões',exact:true}).click();
 await page.locator('tr').filter({hasText:'browser-rdp'}).getByRole('button',{name:'Abrir',exact:true}).click();
 await page.waitForFunction(()=>document.querySelectorAll('.desktop-window').length===1);
 assert.equal(errors.length,0,errors.join('; '));
 console.log('PASS: Chromium stale storage ignored; dashboard retained; minimize/restore/reconnect/close/reopen; WebSocket frames received');

} finally {
 await page.goto('http://127.0.0.1:18080/app');
 for(const [kind,obj] of [['connections',conn2],['connections',conn],['credentials',cred],['hosts',host],['networks',n1],['networks',n2],['locations',loc]])if(obj)await api(kind+'/'+obj.id,'DELETE');
 await browser.close();
}
