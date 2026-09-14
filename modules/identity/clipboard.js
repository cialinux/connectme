'use strict';
const clipboardStyle=document.createElement('style');clipboardStyle.textContent=`
.clipboard-notice{margin:0;padding:8px 14px;background:#18324b;color:#dcecff;font-size:13px;border-bottom:1px solid #33516d}
.transfer-panel{background:#101c2c;color:#e6edf5;display:grid;grid-template-columns:1fr 1fr;gap:12px}.transfer-panel[hidden]{display:none}
.transfer-panel fieldset{min-width:0;margin:0;border:1px solid #35455d;border-radius:8px;padding:12px}.transfer-panel legend{font-weight:600;padding:0 6px}
.transfer-panel textarea{box-sizing:border-box;display:block;width:100%;resize:vertical;min-height:64px;margin:8px 0}.transfer-panel p{font-size:13px;line-height:1.4}
.transfer-panel button{margin:4px 6px 4px 0;padding:7px 10px}.transfer-panel progress{display:block;width:100%;height:8px;margin:10px 0}.transfer-notice{grid-column:1/-1;margin:0}
.transfer-files>div{padding:8px 0;border-bottom:1px solid #35455d;overflow-wrap:anywhere}@media(max-width:800px){.transfer-panel{grid-template-columns:1fr}}
`;document.head.append(clipboardStyle);
// Session-scoped browser integration. No polling, global clipboard cache or
// clipboard reads when focus changes. Only explicit paste/copy gestures.
function installDirectClipboard(entry,caps,bridge){
 const sink=document.createElement('textarea');sink.tabIndex=-1;sink.setAttribute('aria-label','Entrada de colagem da sessão');
 sink.style.cssText='position:absolute;width:1px;height:1px;opacity:0;pointer-events:none';entry.screen.append(sink);
 const ssh=entry.connection.protocol==='ssh';
 // A real editable event target enables the browser's native Paste menu even
 // on HTTP. Left-button events still bubble into Guacamole's mouse handler.
 const rightDown=event=>{if(event.button===2){event.stopImmediatePropagation();releaseKeys();sink.focus();sink.value=caps.copy?(bridge.currentText?.()||''):'';sink.select()}};
 const rightUp=event=>{if(event.button===2)event.stopImmediatePropagation()};
 const nativeMenu=event=>{event.stopPropagation()};
 if(ssh){
  const display=entry.client.getDisplay().getElement();
  sink.style.cssText='position:absolute;inset:0;width:100%;height:100%;opacity:0;resize:none;cursor:text;z-index:100;box-sizing:border-box';
  display.append(sink);sink.addEventListener('mousedown',rightDown,true);sink.addEventListener('mouseup',rightUp,true);sink.addEventListener('contextmenu',nativeMenu);
 }
 const toast=document.createElement('p');toast.className='clipboard-notice';toast.setAttribute('role','status');toast.hidden=true;entry.pane.insertBefore(toast,entry.screen);
 let stopped=false,pasting=false,copyRequested=false,copyTimer;
 const focused=()=>!stopped&&!entry.closed&&entry.connected&&activeDesktop===entry&&!entry.pane.hidden&&entry.screen.contains(document.activeElement)&&document.hasFocus();
 const notify=message=>{if(stopped)return;toast.textContent=message;toast.hidden=false};
 const keys=()=>{if(ssh){entry.client.sendKeyEvent(1,0xffe3);entry.client.sendKeyEvent(1,0xffe1);entry.client.sendKeyEvent(1,86);entry.client.sendKeyEvent(0,86);entry.client.sendKeyEvent(0,0xffe1);entry.client.sendKeyEvent(0,0xffe3)}else{entry.client.sendKeyEvent(1,0xffe3);entry.client.sendKeyEvent(1,118);entry.client.sendKeyEvent(0,118);entry.client.sendKeyEvent(0,0xffe3)}};
 function keydown(event){
  if(!focused())return;
  if((event.ctrlKey||event.metaKey)&&event.key.toLowerCase()==='c'){
   copyRequested=!!caps.copy;clearTimeout(copyTimer);copyTimer=setTimeout(()=>copyRequested=false,5000);return;
  }
  const paste=((event.ctrlKey||event.metaKey)&&event.key.toLowerCase()==='v')||(event.shiftKey&&event.key==='Insert');
  if(!paste)return;
  event.stopImmediatePropagation();releaseKeys();
  if(event.repeat||pasting){event.preventDefault();return}
  // Focus an invisible textarea for the native paste event, including HTTP.
  // Do not preventDefault here: that would suppress the browser paste gesture.
  sink.focus();sink.value='';
 }
 async function paste(event){
  if(!focused())return;event.preventDefault();event.stopImmediatePropagation();releaseKeys();
  if(pasting)return;entry.screen.focus();
  const files=Array.from(event.clipboardData?.files||[]);
  const text=event.clipboardData?.getData('text/plain')||'';
  pasting=true;
  try{
   if(files.length){if(!caps.upload)throw Error('Envio de arquivos não permitido nesta sessão.');await bridge.files(files);notify('Arquivos/imagens enviados à unidade ConnectMe; não são colados dentro do aplicativo remoto.');return}
   if(!caps.paste)throw Error('Colagem de texto não permitida nesta sessão.');
   if(!text){notify('O navegador não forneceu conteúdo. Ctrl+C/V de arquivos do Explorador não é suportado como no RDP nativo. Arraste o arquivo para a sessão ou use Texto e arquivos → Enviar arquivo.');return}
   if(entry.connection.protocol==='ssh'&&/[\r\n]/.test(text)&&!confirm('Colar múltiplas linhas no terminal pode executar comandos. Continuar?'))return;
   await bridge.text(text);
   // Tunnel ordering is not an RDP clipboard-ready acknowledgement. Windows
   // processes the CLIPRDR format announcement asynchronously; allow it to
   // register the new clipboard before sending the paste keystroke. Never
   // retry the keystroke automatically (terminal/app side effects).
   if(entry.connection.protocol==='rdp')await new Promise(resolve=>setTimeout(resolve,250));
   if(!focused()){notify('Texto enviado, mas a colagem foi cancelada porque a sessão perdeu o foco.');return}
   keys();notify('Texto enviado; atalho de colagem aplicado à sessão.');
  }catch(error){notify(error.message)}finally{pasting=false;sink.value=''}
 }
 async function drop(event){
  event.preventDefault();event.stopPropagation();if(!focused()||pasting)return;
  if(!caps.upload){notify('Envio de arquivos não permitido nesta sessão.');return}
  const files=Array.from(event.dataTransfer?.files||[]);if(!files.length)return;
  pasting=true;try{await bridge.files(files);notify('Arquivos enviados à unidade ConnectMe.')}catch(error){notify(error.message)}finally{pasting=false}
 }
 const drag=event=>{event.preventDefault();if(event.dataTransfer)event.dataTransfer.dropEffect=caps.upload?'copy':'none'};
 // Guacamole captures keys on document. Window capture must run first or
 // Guacamole cancels native Ctrl+V before the browser can deliver paste.
 window.addEventListener('keydown',keydown,true);entry.screen.addEventListener('paste',paste);entry.screen.addEventListener('drop',drop);entry.screen.addEventListener('dragover',drag);
 return {
  notify,
  async received(text){
   if(!copyRequested||!focused())return;copyRequested=false;clearTimeout(copyTimer);
   if(!window.isSecureContext||!navigator.clipboard?.writeText){notify('Texto remoto recebido. Em HTTP, use Copiar texto remoto no menu Texto e arquivos.');return}
   try{await navigator.clipboard.writeText(text);if(focused())notify('Texto remoto copiado para este computador.')}catch{notify('O navegador bloqueou o clipboard. Use o botão de cópia no menu Texto e arquivos.')}
  },
  close(){stopped=true;clearTimeout(copyTimer);window.removeEventListener('keydown',keydown,true);entry.screen.removeEventListener('paste',paste);entry.screen.removeEventListener('drop',drop);entry.screen.removeEventListener('dragover',drag);sink.value='';sink.remove();toast.remove()}
 };
}
