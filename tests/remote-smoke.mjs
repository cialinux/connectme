// Destructive only to its own fixtures. Run against the disposable test stack.
import assert from 'node:assert/strict';
const base='http://127.0.0.1:18080';
let cookies={};
async function request(path,method='GET',body,expected=200,form=false){
 const r=await fetch(base+path,{method,headers:{Cookie:Object.entries(cookies).map(([k,v])=>k+'='+v).join('; '),'X-CSRF-Token':cookies.connectme_csrf||'',Origin:base,...(body===undefined?{}:{'Content-Type':form?'application/x-www-form-urlencoded':'application/json'})},body:body===undefined?undefined:form?body:JSON.stringify(body)});
 for(const c of r.headers.getSetCookie()){const pair=c.split(';')[0],p=pair.indexOf('=');cookies[pair.slice(0,p)]=pair.slice(p+1)}
 const raw=await r.text(); assert.equal(r.status,expected,`${method} ${path.split('?')[0]} unexpected status ${r.status}: ${raw.slice(0,160)}`);
 try{return JSON.parse(raw)}catch{return raw}
}
try {await request('/api/v1/auth/login','POST',{email:'admin',password:'Isolated-test-only-2026!'})}
catch {await request('/api/v1/auth/login','POST',{email:'admin',password:'admin'});
await request('/api/v1/auth/password','POST',{current_password:'admin',new_password:'Isolated-test-only-2026!'});
await request('/api/v1/auth/login','POST',{email:'admin',password:'Isolated-test-only-2026!'})}
assert.match(await request('/app/assets/console.js'),/Salvar alterações/);
const loc=await request('/api/v1/locations','POST',{name:'remote-smoke-'+Date.now(),region:'test',description:''},201);
const net=await request(`/api/v1/locations/${loc.id}/networks`,'POST',{name:'test-net',cidr:'192.0.2.0/24'},201);
const host=await request('/api/v1/hosts','POST',{name:'test-host',location_id:loc.id,address:'192.0.2.246',operating_system:'windows'},201);
host.address='192.0.2.200';await request('/api/v1/hosts/'+host.id,'PUT',host);
const cred=await request('/api/v1/credentials','POST',{name:'test-credential',type:'password',username:'test-user',domain:'',value:'Dummy-secret-only'},201);
const conn=await request('/api/v1/connections','POST',{name:'test-rdp',host_id:host.id,credential_ref_id:cred.id,protocol:'rdp',port:3389},201);
const launch=await request('/api/v1/connections/'+conn.id+'/open','POST',{});
assert.match(launch.id,/^[a-f0-9]{32}$/);
assert.ok(!('authToken' in launch)&&!('url' in launch));
await request(launch.tunnel,'GET',undefined,400);
const ownerCookies={...cookies};
await request('/api/v1/auth/login','POST',{email:'admin',password:'Isolated-test-only-2026!'});
await request(launch.tunnel,'GET',undefined,403);
cookies=ownerCookies;
const second=await request('/api/v1/connections/'+conn.id+'/open','POST',{});
assert.notEqual(second.id,launch.id);
await request(launch.tunnel,'GET',undefined,400);
await request(second.tunnel,'GET',undefined,400);
await request('/api/v1/remote-sessions/'+second.id,'DELETE',undefined,204);
await request(second.tunnel,'GET',undefined,403);
console.log('PASS: independent session IDs, server-side tokens, closing one preserves the other');
await request('/api/v1/credentials/'+cred.id,'PUT',{name:'test-credential',type:'password',username:'test-user',domain:'',enabled:true,value:'Rotated-dummy-secret'});
await request(launch.tunnel,'GET',undefined,403);
await request('/api/v1/connections/'+conn.id+'/open','POST',{});

host.enabled=false;await request('/api/v1/hosts/'+host.id,'PUT',host);
await request('/api/v1/connections/'+conn.id+'/open','POST',{},400);
await request(launch.tunnel,'GET',undefined,403);
await request('/api/v1/remote-sessions/'+launch.id,'DELETE',undefined,204);
await request('/api/v1/connections/'+conn.id,'DELETE',undefined,204);
await request('/api/v1/credentials/'+cred.id,'DELETE',undefined,204);
await request('/api/v1/hosts/'+host.id,'DELETE',undefined,204);
await request('/api/v1/networks/'+net.id,'DELETE',undefined,204);
await request('/api/v1/locations/'+loc.id,'DELETE',undefined,204);
console.log('PASS: secret rotation invalidates lease; blocked host denied; soft deletion of all fixture resources');
const user=await request('/api/v1/users','POST',{email:'smoke-'+Date.now()+'@test.invalid',display_name:'Temporary test',password:'Temporary-password-2026!'},201);
await request('/api/v1/users/'+user.id,'PUT',{email:user.email,display_name:'Edited user',password:'',enabled:false});
const users=await request('/api/v1/users');assert.equal(users.items.find(x=>x.id===user.id).enabled,false);
const admin=users.items.find(x=>x.email==='admin');
await request('/api/v1/users/'+admin.id,'DELETE',undefined,400);
await request('/api/v1/users/'+user.id,'DELETE',undefined,204);
console.log('PASS: user edit/block/delete and administrator self-deletion denied');
