'use strict';
const TEXT_LIMIT=64*1024,FILE_LIMIT=32*1024*1024;
function confirmTransferClose(entry){return !entry.hasTemporaryFiles||confirm('Encerrar/reconectar apaga os arquivos temporários desta sessão. Copiou o que precisa para uma pasta permanente?')}
function installTransfers(entry,capabilities,toolbar){
 const caps=capabilities||{},panel=document.createElement('section');panel.className='transfer-panel';panel.hidden=true;
 panel.style.cssText='margin:0;padding:12px;overflow:auto;max-height:55%;flex-shrink:0;border-radius:0';
 const note=document.createElement('p');note.setAttribute('role','status');note.className='transfer-notice';
 const base='remote-sessions/'+entry.id;
 let stopped=false,busy=false,cancelTransfer=null;
 const btn=(label,fn)=>{const b=document.createElement('button');b.type='button';b.textContent=label;b.onclick=async()=>{b.disabled=true;releaseKeys();try{await fn()}catch(error){note.textContent=error.name==='AbortError'?'Transferência cancelada.':error.message}finally{b.disabled=false}};return b};
 const toggle=btn('Texto e arquivos',()=>{panel.hidden=!panel.hidden;if(!panel.hidden){note.textContent='';if(caps.upload||caps.download)return refreshFiles()}});
 toolbar.append(toggle);entry.pane.insertBefore(panel,entry.screen);
 const text=document.createElement('fieldset');const title=document.createElement('legend');title.textContent='Área de transferência · somente texto';text.append(title);
 const outgoing=document.createElement('textarea');outgoing.rows=3;outgoing.maxLength=TEXT_LIMIT;outgoing.setAttribute('aria-label','Texto para enviar ao remoto');outgoing.placeholder='Cole aqui (Ctrl+V) e clique Enviar texto. Depois use Ctrl+V no Windows remoto.';outgoing.disabled=!caps.paste;
 const incoming=document.createElement('textarea');incoming.rows=3;incoming.readOnly=true;incoming.setAttribute('aria-label','Texto recebido do remoto');incoming.placeholder='Copie texto no Windows remoto (Ctrl+C); ele aparecerá aqui se permitido.';
 async function sendText(value){
  if(!caps.paste)throw Error('Colagem de texto não permitida nesta sessão.');
  if(!entry.connected)throw Error('Conecte ao desktop antes de enviar texto.');
  const bytes=new TextEncoder().encode(value).length;if(bytes>TEXT_LIMIT)throw Error('Limite de 64 KiB de texto.');
  await api(base+'/clipboard-events','POST',{direction:'paste',bytes});
  if(stopped||!entry.connected)throw Error('Sessão encerrada.');
  const writer=new Guacamole.StringWriter(entry.client.createClipboardStream('text/plain'));
  writer.onack=status=>{if(status.isError())note.textContent='O remoto recusou o texto ('+status.code+').'};
  writer.sendText(value);writer.sendEnd();
 }
 const send=btn('Enviar texto',async()=>{await sendText(outgoing.value);note.textContent='Texto enviado à área de transferência remota. Use Ctrl+V no Windows.'});send.disabled=!caps.paste;
 const readLocal=btn('Ler clipboard local',async()=>{outgoing.value=await navigator.clipboard.readText();note.textContent='Texto local carregado. Revise e clique Enviar texto.'});readLocal.disabled=!caps.paste||!window.isSecureContext||!navigator.clipboard?.readText;
 const copyLocal=btn('Copiar recebido para meu computador',async()=>{if(window.isSecureContext&&navigator.clipboard?.writeText){await navigator.clipboard.writeText(incoming.value)}else{incoming.focus();incoming.select();if(!document.execCommand('copy'))throw Error('Use Ctrl+C no texto selecionado.')}note.textContent='Texto copiado para a área de transferência local.'});copyLocal.disabled=!caps.copy;
 const select=btn('Selecionar texto recebido',()=>{incoming.focus();incoming.select();note.textContent='Use Ctrl+C para copiar o texto selecionado.'});select.disabled=!caps.copy;
 text.append(outgoing,readLocal,send,incoming,copyLocal,select);
 const hint=document.createElement('p');hint.textContent='Use Ctrl+V diretamente na tela remota. Copiar do remoto automaticamente exige HTTPS e permissão; em HTTP, use o botão de cópia abaixo. Este painel é a alternativa manual.';text.append(hint);
 if(!caps.copy&&!caps.paste){const disabled=document.createElement('p');disabled.textContent='Clipboard desativado. Habilite as direções em Conexões → Editar e reconecte.';text.append(disabled)}
 entry.client.onclipboard=(stream,mimetype)=>{
  if(stopped||!caps.copy||mimetype!=='text/plain'){stream.sendAck('Unsupported clipboard',0x0100);return}
  let value='',size=0,rejected=false;const reader=new Guacamole.StringReader(stream);
  reader.ontext=chunk=>{if(rejected)return;size+=new TextEncoder().encode(chunk).length;if(size>TEXT_LIMIT){rejected=true;value='';stream.sendAck('Text limit',0x030D);note.textContent='Texto remoto excedeu 64 KiB.';return}value+=chunk;stream.sendAck('OK',0)};
  reader.onend=async()=>{if(rejected||stopped)return;try{await api(base+'/clipboard-events','POST',{direction:'copy',bytes:size});if(!stopped){incoming.value=value;note.textContent='Texto remoto recebido.';await direct.received(value)}}catch(error){note.textContent=error.message}};
 };
 const files=document.createElement('fieldset');const legend=document.createElement('legend');legend.textContent='Arquivos temporários · unidade ConnectMe';files.append(legend);
 const fileHelp=document.createElement('p');fileHelp.textContent='Arquivos não usam Ctrl+C/V entre computadores. Local → remoto: arraste para a tela ou envie abaixo e copie da unidade ConnectMe para a pasta desejada. Remoto → local: copie para a raiz da unidade ConnectMe, clique Atualizar arquivos e Baixar.';files.append(fileHelp);
 const warning=document.createElement('p');warning.textContent='32 MiB por arquivo; 128 MiB por sessão via upload. Encerrar, reconectar ou reiniciar apaga os temporários. No Windows, copie arquivos da/para a raiz da unidade ConnectMe (\\tsclient\\ConnectMe).';files.append(warning);
 const chooser=document.createElement('input');chooser.type='file';chooser.setAttribute('aria-label','Selecionar arquivo para enviar');chooser.disabled=!caps.upload;
 const progress=document.createElement('progress');progress.max=100;progress.value=0;progress.setAttribute('aria-label','Progresso da transferência');
 const cancel=btn('Cancelar transferência',()=>cancelTransfer?.());
 const list=document.createElement('div');list.className='transfer-files';
 async function refreshFiles(){const result=await api(base+'/files');if(stopped)return;entry.hasTemporaryFiles=result.items.length>0;list.replaceChildren();for(const item of result.items){const row=document.createElement('div');const label=document.createElement('span');label.textContent=item.name+' · '+item.size+' bytes ';row.append(label);if(caps.download)row.append(btn('Baixar '+item.name,()=>download(item)));list.append(row)}if(!result.items.length)list.textContent='Nenhum arquivo na raiz da unidade temporária.'}
 async function upload(file=chooser.files[0]){
  if(stopped||!caps.upload)throw Error('Envio de arquivos indisponível.');
  if(busy)throw Error('Aguarde ou cancele a transferência atual.');if(!file)throw Error('Selecione um arquivo.');if(file.size>FILE_LIMIT)throw Error('Limite de 32 MiB por arquivo.');
  busy=true;progress.value=0;note.textContent='Enviando…';
  try{await new Promise((resolve,reject)=>{const xhr=new XMLHttpRequest();cancelTransfer=()=>xhr.abort();xhr.open('POST','/api/v1/'+base+'/files/'+encodeURIComponent(file.name));xhr.setRequestHeader('X-CSRF-Token',csrf());xhr.setRequestHeader('Content-Type','application/octet-stream');xhr.timeout=120000;xhr.upload.onprogress=e=>{if(e.lengthComputable)progress.value=e.loaded/e.total*100};xhr.onload=()=>{if(xhr.status===201)resolve();else{let message='Envio recusado ('+xhr.status+').';try{message=JSON.parse(xhr.responseText).error.detail}catch{}reject(Error(message))}};xhr.onerror=()=>reject(Error('Falha de rede no envio.'));xhr.ontimeout=()=>reject(Error('Tempo de envio excedido.'));xhr.onabort=()=>reject(new DOMException('Cancelado','AbortError'));xhr.send(file)});chooser.value='';await refreshFiles();progress.value=100;note.textContent='Arquivo enviado à unidade ConnectMe do remoto.'}finally{busy=false;cancelTransfer=null}
 }
 async function download(item){
  if(busy)throw Error('Aguarde ou cancele a transferência atual.');if(item.size>FILE_LIMIT)throw Error('Arquivo maior que 32 MiB.');
  busy=true;progress.value=0;const controller=new AbortController();cancelTransfer=()=>controller.abort();
  try{const response=await fetch('/api/v1/'+base+'/files/'+encodeURIComponent(item.name),{signal:controller.signal});if(!response.ok)throw Error('Download recusado ('+response.status+').');const reader=response.body.getReader();let count=0;const parts=[];for(;;){const {done,value}=await reader.read();if(done)break;count+=value.length;if(count>FILE_LIMIT){controller.abort();throw Error('Arquivo excedeu o limite.')}parts.push(value);progress.value=item.size?count/item.size*100:100}if(stopped)return;const url=URL.createObjectURL(new Blob(parts,{type:'application/octet-stream'}));const anchor=document.createElement('a');anchor.href=url;anchor.download=item.name;anchor.click();setTimeout(()=>URL.revokeObjectURL(url),1000);progress.value=100;note.textContent='Download concluído.'}finally{busy=false;cancelTransfer=null}
 }
 const uploadButton=btn('Enviar arquivo',()=>upload());uploadButton.disabled=!caps.upload;
 const refresh=btn('Atualizar arquivos',refreshFiles);refresh.disabled=!caps.upload&&!caps.download;
 files.append(chooser,uploadButton,refresh,cancel,progress,list);
 if(!caps.upload&&!caps.download){const disabled=document.createElement('p');disabled.textContent='Arquivos desativados. Habilite envio/download em Conexões → Editar e reconecte.';files.append(disabled)}
 panel.append(note,text,files);
 const direct=installDirectClipboard(entry,caps,{text:sendText,currentText:()=>incoming.value,files:async input=>{
  if(input.length>8)throw Error('Envie no máximo 8 arquivos por vez.');
  direct.notify('Enviando '+input.length+' arquivo(s)… Abra Texto e arquivos para ver o progresso ou cancelar.');
  let completed=0;
  try{for(let file of input){if(stopped)throw Error('Sessão encerrada.');if(file.type.startsWith('image/')&&/^image\.(png|jpg|jpeg)$/i.test(file.name))file=new File([file],'captura-'+Date.now()+'-'+crypto.getRandomValues(new Uint32Array(1))[0].toString(16)+'.'+file.name.split('.').pop(),{type:file.type});await upload(file);completed++}}
  catch(error){throw Error(completed+' de '+input.length+' arquivos enviados. '+error.message)}
 }});
 entry.closeTransfers=()=>{stopped=true;direct.close();cancelTransfer?.();outgoing.value='';incoming.value='';chooser.value='';panel.remove()};
}
