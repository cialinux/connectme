// Destructive only to the explicitly named disposable SSH fixture, never production.
import assert from 'node:assert/strict';
import {execFileSync} from 'node:child_process';
const address=process.env.SSH_FIXTURE_IP;
const fixture=process.env.SSH_FIXTURE_CONTAINER;
if(!address||fixture!=='connectme-cleanup-check-fixture-1')throw Error('Requires the isolated cleanup-check fixture');
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'/tmp/connectme-browser-tools/node_modules/playwright/index.mjs');
const browser=await chromium.launch({headless:true});
try {
 const page=await browser.newPage();
 await page.goto('http://127.0.0.1:18080/');
 await page.locator('[name=email]').fill('admin');
 await page.locator('[name=password]').fill('Isolated-test-only-2026!');
 await page.getByRole('button',{name:'Entrar',exact:true}).click();
 await page.waitForURL('**/app');
 const api=(path,body)=>page.evaluate(async({path,body})=>{
  const csrf=document.cookie.split('; ').find(x=>x.startsWith('connectme_csrf='))?.split('=')[1];
  const response=await fetch('/api/v1/'+path,{method:'POST',headers:{'Content-Type':'application/json','X-CSRF-Token':csrf},body:JSON.stringify(body)});
  if(!response.ok)throw Error('API '+response.status);
  return response.json();
 },{path,body});
 const loc=await api('locations',{name:'trust-fixture-'+Date.now()});
 await api('locations/'+loc.id+'/networks',{name:'fixture',cidr:address+'/32'});
 const host=await api('hosts',{name:'trust-fixture',location_id:loc.id,address,operating_system:'linux'});
 const cred=await api('credentials',{name:'trust-fixture',type:'password',username:'fixture',value:'Fixture-only-2026!'});
 const conn=await api('connections',{name:'trust-fixture',host_id:host.id,credential_ref_id:cred.id,protocol:'ssh',port:2222});
 const path='connections/'+conn.id+'/ssh-identity';
 const first=await api(path,{});
 assert.equal(first.registered,true);
 const again=await api(path,{});
 assert.equal(again.registered,false);
 assert.equal(again.fingerprint,first.fingerprint);
 execFileSync('docker',['restart',fixture],{stdio:'ignore'});
 let changed;
 for(let attempt=0;attempt<30;attempt++){
  try{changed=await api(path,{});break}catch{await new Promise(resolve=>setTimeout(resolve,500))}
 }
 assert.equal(changed?.changed,true);
 assert.equal(changed.previous,first.fingerprint);
 assert.notEqual(changed.fingerprint,first.fingerprint);
 const rejected=await api(path,{approve:changed.fingerprint,previous:'stale'});
 assert.equal(rejected.changed,true);
 const approved=await api(path,{approve:changed.fingerprint,previous:changed.previous});
 assert.equal(approved.changed,false);
 assert.equal(approved.fingerprint,changed.fingerprint);
 const persisted=await api(path,{});
 assert.equal(persisted.registered,false);
 assert.equal(persisted.fingerprint,changed.fingerprint);
 console.log('PASS: first-use trust, reuse, changed-key warning, stale approval rejected, explicit rotation persisted');
} finally {await browser.close()}
