// Local preparation only: never invokes kubectl or prints secret values.
import {randomBytes} from 'node:crypto';
import {chmodSync,existsSync,lstatSync,mkdirSync,readFileSync,writeFileSync} from 'node:fs';
import {resolve,join} from 'node:path';

const usage='Uso: node scripts/prepare-k8s-env.mjs dev|pro [--check | --from arquivo.env]';
function fail(message){throw new Error(message)}
function regular(path){
 const stat=lstatSync(path);
 if(!stat.isFile()||stat.isSymbolicLink())fail('Esperado arquivo regular privado, não link simbólico.');
 return stat;
}
function parse(source){
 const values={};
 for(const raw of source.split(/\r?\n/)){
  const line=raw.trim();if(!line||line.startsWith('#'))continue;
  const match=/^([A-Z][A-Z0-9_]*)=(.*)$/.exec(line);
  if(!match)fail('Formato env inválido; nenhum valor exibido.');
  let value=match[2].trim();
  if((value.startsWith('"')&&value.endsWith('"'))||(value.startsWith("'")&&value.endsWith("'")))value=value.slice(1,-1);
  if(value.includes('${')||/[\r\n\0]/.test(value))fail('Use valores literais de uma linha no arquivo env.');
  if(Object.hasOwn(values,match[1]))fail('Chave duplicada: '+match[1]);
  values[match[1]]=value;
 }
 return values;
}
function validate(values){
 let db;try{db=new URL(values.CONNECTME_DATABASE_URL)}catch{fail('URL PostgreSQL inválida; valor não exibido.')}
 if(!['postgres:','postgresql:'].includes(db.protocol)||db.hostname!=='postgres'||db.pathname!=='/connectme'||db.username!=='connectme'||(db.port&&db.port!=='5432')||!db.password)fail('A URL deve apontar para connectme@postgres:5432/connectme com senha.');
 let password;try{password=decodeURIComponent(db.password)}catch{fail('Senha da URL possui codificação inválida.')}
 if(/[\r\n\0]/.test(password)||password!==values.POSTGRES_PASSWORD)fail('POSTGRES_PASSWORD deve coincidir com a senha da URL; nenhum valor foi alterado.');
 if(password.startsWith('replace-with-'))fail('Substitua a senha de exemplo; não é uma configuração válida para instalar.');
 if(values.POSTGRES_DB!=='connectme'||values.POSTGRES_USER!=='connectme')fail('POSTGRES_DB e POSTGRES_USER devem ser connectme.');
 for(const key of ['CONNECTME_MASTER_KEY','CONNECTME_AUDIT_HMAC_KEY']){
  const value=values[key];
  if(!/^[A-Za-z0-9+/]{43}=$/.test(value||'')||Buffer.from(value,'base64').toString('base64')!==value)fail('Chave deve conter exatamente 32 bytes em base64: '+key);
 }
 if(!/^[a-fA-F0-9]{32}$/.test(values.CONNECTME_GUACAMOLE_KEY||''))fail('CONNECTME_GUACAMOLE_KEY deve conter 32 caracteres hexadecimais.');
}
function fresh(environment){
 const password=randomBytes(32).toString('hex');
 return {
  POSTGRES_DB:'connectme',POSTGRES_USER:'connectme',POSTGRES_PASSWORD:password,
  CONNECTME_DATABASE_URL:`postgres://connectme:${password}@postgres:5432/connectme?sslmode=disable`,
  CONNECTME_ENV:environment==='pro'?'production':'development',
  CONNECTME_HTTP_ADDRESS:':8081',CONNECTME_MODULES:'system=true',CONNECTME_COOKIE_SECURE:'true',
  CONNECTME_MASTER_KEY:randomBytes(32).toString('base64'),
  CONNECTME_AUDIT_HMAC_KEY:randomBytes(32).toString('base64'),
  CONNECTME_GUACAMOLE_KEY:randomBytes(16).toString('hex'),
  CONNECTME_REMOTE_DENY_CIDRS:'',CONNECTME_RDP_IGNORE_CERT:'true'
 };
}
function manifest(environment,values){
 return JSON.stringify({apiVersion:'v1',kind:'List',items:[
  {apiVersion:'v1',kind:'Namespace',metadata:{name:'connectme-'+environment}},
  {apiVersion:'v1',kind:'Secret',metadata:{name:'connectme-runtime',namespace:'connectme-'+environment},type:'Opaque',
   data:Object.fromEntries(Object.entries(values).map(([key,value])=>[key,Buffer.from(value).toString('base64')]))}
 ]},null,2)+'\n'; // JSON is valid YAML, avoiding quoting/interpolation ambiguities.
}
function main(){
 const [environment,...args]=process.argv.slice(2);
 if(!['dev','pro'].includes(environment))fail(usage);
 const check=args.length===1&&args[0]==='--check';
 const from=args.length===2&&args[0]==='--from'?resolve(args[1]):null;
 if(args.length&&!check&&!from)fail(usage);
 const folder=resolve('.local','kubernetes',environment);
 const envFile=join(folder,'runtime.env'),manifestFile=join(folder,'bootstrap.yaml');
 const legacy=resolve('.local','connectme-'+environment+'.env');
 const dirs=[resolve('.local'),resolve('.local','kubernetes'),folder];
 for(const dir of dirs){
  if(existsSync(dir)){
   const stat=lstatSync(dir);
   if(!stat.isDirectory()||stat.isSymbolicLink())fail('Diretório privado inválido ou link simbólico.');
  } else if(!check)mkdirSync(dir,{mode:0o700});
 }
 let values;
 if(existsSync(envFile)){
  regular(envFile);values=parse(readFileSync(envFile,'utf8'));validate(values);
  if(from){regular(from);const imported=parse(readFileSync(from,'utf8'));validate(imported);
   for(const key of ['POSTGRES_PASSWORD','CONNECTME_DATABASE_URL','CONNECTME_MASTER_KEY','CONNECTME_AUDIT_HMAC_KEY','CONNECTME_GUACAMOLE_KEY'])if(values[key]!==imported[key])fail('Importação diverge da configuração existente. Não sobrescrevi chaves.');
  }
 } else {
  if(check)fail('Preparação ausente. Para instalação nova: node scripts/prepare-k8s-env.mjs '+environment);
  // An orphaned manifest may be the only remaining backup. Never replace its keys.
  if(existsSync(manifestFile))fail('Existe bootstrap.yaml sem runtime.env. Recupere a configuração; não gere chaves novas.');
  const source=from||(existsSync(legacy)?legacy:null);
  if(source){regular(source);values=parse(readFileSync(source,'utf8'));validate(values)}
  else values=fresh(environment);
  validate(values);
  writeFileSync(envFile,Object.entries(values).map(([key,value])=>key+'='+value).join('\n')+'\n',{mode:0o600,flag:'wx'});
  console.log(source?'Configuração existente importada; chaves preservadas.':'Configuração nova gerada. Use somente com banco novo; para banco existente, recupere suas chaves originais.');
 }
 const rendered=manifest(environment,values);
 if(existsSync(manifestFile)){
  regular(manifestFile);
  if(readFileSync(manifestFile,'utf8')!==rendered)fail('bootstrap.yaml diverge de runtime.env. Nenhum manifesto foi sobrescrito; confira os arquivos privados.');
 } else {
  if(check)fail('Manifesto ausente. Execute a preparação novamente para gerá-lo com as mesmas chaves.');
  writeFileSync(manifestFile,rendered,{mode:0o600,flag:'wx'});
 }
 for(const path of [envFile,manifestFile]){
  if(check){if(regular(path).mode&0o077)fail('Restrinja as permissões dos arquivos privados a 0600.')}
  else chmodSync(path,0o600);
 }
 console.log('Validado: .local/kubernetes/'+environment+'/runtime.env e bootstrap.yaml (privados, fora do Git).');
 console.log('Nenhum recurso aplicado. Para a primeira instalação, o operador executa:');
 console.log('kubectl --kubeconfig "$HOME/.kube/config-staging" apply -f .local/kubernetes/'+environment+'/bootstrap.yaml');
 console.log('Depois sincronize base e overlays/'+environment+' no Fleet, ambos em connectme-'+environment+'.');
}
try{main()}catch(error){
 console.error(error.code?'Não foi possível acessar os arquivos privados ('+error.code+'); nenhum segredo exibido.':error.message);
 process.exitCode=1;
}
