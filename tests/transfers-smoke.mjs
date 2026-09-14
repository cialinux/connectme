// Disposable stack only. Uses no credentials or files from the real deployment.
import assert from 'node:assert/strict';
const base='http://127.0.0.1:18080';let cookies={};
async function req(path,method='GET',body,expected=200,binary=false){
 const r=await fetch(base+'/api/v1/'+path,{method,headers:{Cookie:Object.entries(cookies).map(([k,v])=>k+'='+v).join('; '),'X-CSRF-Token':cookies.connectme_csrf||'',Origin:base,...(body===undefined?{}:{'Content-Type':binary?'application/octet-stream':'application/json'})},body:body===undefined?undefined:binary?body:JSON.stringify(body)});
 for(const c of r.headers.getSetCookie()){const pair=c.split(';')[0],p=pair.indexOf('=');cookies[pair.slice(0,p)]=pair.slice(p+1)}
 assert.equal(r.status,expected,method+' '+path+' returned '+r.status);
 if(binary&&method==='GET')return Buffer.from(await r.arrayBuffer());
 const text=await r.text();return text?JSON.parse(text):{};
}
await req('auth/login','POST',{email:'admin',password:'Isolated-test-only-2026!'});
let loc,net,host,cred,conn,a,b,restricted;
try{
 loc=await req('locations','POST',{name:'transfer-'+Date.now()},201);
 net=await req('locations/'+loc.id+'/networks','POST',{name:'documentation',cidr:'192.0.2.0/24'},201);
 host=await req('hosts','POST',{name:'transfer-host',location_id:loc.id,address:'192.0.2.20',operating_system:'windows'},201);
 cred=await req('credentials','POST',{name:'transfer-credential',type:'password',username:'dummy',value:'Dummy-only-no-real-host'},201);
 conn=await req('connections','POST',{name:'transfer',host_id:host.id,credential_ref_id:cred.id,protocol:'rdp',port:3389,clipboard_enabled:true,clipboard_copy_enabled:true,clipboard_paste_enabled:false,file_transfer_enabled:true,file_upload_enabled:true,file_download_enabled:true},201);
 a=await req('connections/'+conn.id+'/open','POST',{});b=await req('connections/'+conn.id+'/open','POST',{});
 assert.deepEqual(a.capabilities,{copy:true,paste:false,upload:true,download:true});
 const payload=Buffer.from([0,255,128,10,13,1,2,3,4]);
 await req('remote-sessions/'+a.id+'/files/test.bin','POST',payload,201,true);
 assert.deepEqual(await req('remote-sessions/'+a.id+'/files/test.bin','GET',undefined,200,true),payload);
 await req('remote-sessions/'+a.id+'/files/test.bin','POST',Buffer.from('replacement'),409,true);
 await req('remote-sessions/'+a.id+'/files/CON','POST',Buffer.from('bad'),400,true);
 await req('remote-sessions/'+a.id+'/files/x%5Cy','POST',Buffer.from('bad'),400,true);
 assert.equal((await req('remote-sessions/'+b.id+'/files')).items.length,0);
 await req('remote-sessions/'+b.id+'/files/test.bin','GET',undefined,404);
 const owner={...cookies};await req('auth/login','POST',{email:'admin',password:'Isolated-test-only-2026!'});
 await req('remote-sessions/'+a.id+'/files','GET',undefined,403);cookies=owner;
 await req('remote-sessions/'+a.id+'/clipboard-events','POST',{direction:'paste',bytes:5},403);
 await req('remote-sessions/'+a.id+'/clipboard-events','POST',{direction:'copy',bytes:5},204);
 await req('remote-sessions/'+a.id+'/clipboard-events','POST',{direction:'copy',bytes:65537},413);
 await req('connections/'+conn.id,'PUT',{...conn,file_download_enabled:false});
 await req('remote-sessions/'+a.id+'/files','GET',undefined,403);
 restricted=await req('connections/'+conn.id+'/open','POST',{});
 await req('remote-sessions/'+restricted.id+'/files/allowed.txt','POST',Buffer.from('ok'),201,true);
 await req('remote-sessions/'+restricted.id+'/files/allowed.txt','GET',undefined,403);
 await req('remote-sessions/'+a.id,'DELETE',undefined,204);
 await req('remote-sessions/'+a.id+'/files','GET',undefined,403);
 console.log('PASS: binary round-trip, no overwrite, unsafe names, session/login isolation, direction permissions, revocation, clipboard limits, close');
}finally{
 for(const session of [a,b,restricted])if(session)await req('remote-sessions/'+session.id,'DELETE',undefined,204);
 for(const [kind,obj] of [['connections',conn],['credentials',cred],['hosts',host],['networks',net],['locations',loc]])if(obj)await req(kind+'/'+obj.id,'DELETE',undefined,204);
}
