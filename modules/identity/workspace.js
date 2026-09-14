'use strict';
const desktops=new Map();let activeDesktop=null;
const desktopStyle=document.createElement('style');desktopStyle.textContent=`
.desktop-window{position:fixed;inset:80px 24px 20px;z-index:20;background:#0b1220;border:1px solid #438cff;border-radius:12px;box-shadow:0 12px 60px #000b;display:flex;flex-direction:column;overflow:hidden}
.desktop-window[hidden]{display:none}.desktop-toolbar{display:flex;align-items:center;gap:10px;padding:10px;flex-wrap:wrap;background:#1a2537}.desktop-toolbar strong{margin-right:auto}.desktop-screen{flex:1;min-height:0;overflow:auto;background:black;outline:none}.desktop-status{padding:6px 12px;margin:0;font-size:14px}.desktop-window:fullscreen{inset:0;border:0;border-radius:0}#session-tabs{flex-wrap:wrap}`;
document.head.append(desktopStyle);
const remoteKeyboard=typeof Guacamole==='undefined'?null:new Guacamole.Keyboard(document);
const remotePressed=new Set();
function remoteFocused(){return activeDesktop&&!activeDesktop.pane.hidden&&activeDesktop.screen.contains(document.activeElement)&&activeDesktop.connected}
if(remoteKeyboard){remoteKeyboard.onkeydown=key=>{if(remoteFocused()){remotePressed.add(key);activeDesktop.client.sendKeyEvent(1,key);return false}return true};remoteKeyboard.onkeyup=key=>{if(remotePressed.delete(key)&&activeDesktop?.connected)activeDesktop.client.sendKeyEvent(0,key)}}
function releaseKeys(){if(activeDesktop?.connected)for(const key of remotePressed)activeDesktop.client.sendKeyEvent(0,key);remotePressed.clear();remoteKeyboard?.reset()}
window.addEventListener('blur',releaseKeys);
function showDesktop(entry){releaseKeys();for(const d of desktops.values())d.pane.hidden=true;entry.pane.hidden=false;activeDesktop=entry;entry.fit();entry.screen.focus()}
function minimizeDesktop(entry){releaseKeys();entry.pane.hidden=true;if(activeDesktop===entry)activeDesktop=null;entry.tab.focus()}
async function endDesktop(entry){releaseKeys();entry.closed=true;entry.closeTransfers?.();entry.client.disconnect();entry.observer.disconnect();entry.pane.remove();entry.tab.remove();desktops.delete(entry.connection.id);if(activeDesktop===entry)activeDesktop=null;await api('remote-sessions/'+entry.id,'DELETE')}
async function openDesktop(connection){
 if(typeof Guacamole==='undefined')throw Error('Cliente remoto indisponível. Atualize o painel e verifique o gateway.');
 const existing=desktops.get(connection.id);if(existing){if(!existing.disconnected){showDesktop(existing);return}await endDesktop(existing)}
 const data=await api('connections/'+connection.id+'/open','POST',{});
 const pane=document.createElement('div');pane.className='desktop-window';pane.setAttribute('aria-label','Desktop '+connection.name);
 const toolbar=document.createElement('div');toolbar.className='desktop-toolbar';const title=document.createElement('strong');title.textContent=connection.name;toolbar.append(title);
 const status=document.createElement('p');status.className='desktop-status';status.setAttribute('role','status');status.textContent='Conectando…';
 const screen=document.createElement('div');screen.className='desktop-screen';screen.tabIndex=0;screen.setAttribute('aria-label','Tela remota '+connection.name);
 const tunnel=new Guacamole.WebSocketTunnel(data.tunnel);const client=new Guacamole.Client(tunnel);const display=client.getDisplay();screen.append(display.getElement());
 const entry={id:data.id,connection,pane,screen,status,client,connected:false,closed:false,disconnected:false};
 entry.fit=()=>{if(!pane.hidden&&display.getWidth()>0)display.scale(Math.min(screen.clientWidth/display.getWidth(),screen.clientHeight/display.getHeight(),1))};
 entry.observer=new ResizeObserver(entry.fit);entry.observer.observe(screen);display.onresize=entry.fit;
 entry.tab=action(connection.name,()=>showDesktop(entry));entry.tab.dataset.connectionId=connection.id;
 toolbar.append(action('Minimizar',()=>minimizeDesktop(entry)),action('Tela inteira',()=>pane.requestFullscreen()),action('Reconectar',async()=>{if(!confirmTransferClose(entry))return;await endDesktop(entry);await openDesktop(connection)}),action('Encerrar',()=>{if(confirmTransferClose(entry))return endDesktop(entry)}));
 pane.append(toolbar,status,screen);installTransfers(entry,data.capabilities,toolbar);$('desktop-workspace').append(pane);$('session-tabs').append(entry.tab);desktops.set(connection.id,entry);
 client.onstatechange=state=>{entry.connected=state===3;if(state===3){entry.lastError=null;status.textContent='Conectado · teclado e rato ativos ao focar a tela'}else if(state===5&&!entry.closed){entry.disconnected=true;status.textContent=entry.lastError||'Desconectado. Use Reconectar para uma nova autorização.'}};
 client.onerror=error=>{entry.connected=false;entry.disconnected=true;entry.lastError='Conexão interrompida ('+error.code+'). Verifique o destino e os logs do guacd antes de reconectar.';status.textContent=entry.lastError};
 const mouse=new Guacamole.Mouse(display.getElement());mouse.onmousedown=mouse.onmouseup=mouse.onmousemove=state=>{if(!entry.closed&&entry.connected){screen.focus();client.sendMouseState(state,true)}};
 screen.addEventListener('blur',releaseKeys);screen.addEventListener('contextmenu',event=>event.preventDefault());
 showDesktop(entry);client.connect(new URLSearchParams({GUAC_WIDTH:Math.max(640,screen.clientWidth),GUAC_HEIGHT:Math.max(480,screen.clientHeight),GUAC_DPI:96}).toString());
}
window.addEventListener('pagehide',()=>{releaseKeys();for(const d of desktops.values()){d.connected=false;d.disconnected=true;d.closeTransfers?.();d.client.disconnect();d.status.textContent='Página restaurada. Use Reconectar.';fetch('/api/v1/remote-sessions/'+d.id,{method:'DELETE',headers:{'X-CSRF-Token':csrf()},keepalive:true}).catch(()=>{})}});
