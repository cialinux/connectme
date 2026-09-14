// Prepare local secrets only. Never calls kubectl or prints secret values.
import {readFileSync,mkdirSync,writeFileSync} from 'node:fs';
import {resolve} from 'node:path';
const environment=process.argv[2];
if(!['dev','pro'].includes(environment))throw Error('Uso: node scripts/prepare-k8s-env.mjs dev|pro');
const values=new Map();
for(const raw of readFileSync('.env','utf8').split(/\r?\n/)){
 const line=raw.trim();if(!line||line.startsWith('#'))continue;
 const match=/^([A-Z][A-Z0-9_]*)=(.*)$/.exec(line);if(!match)throw Error('Formato .env não suportado; nenhum segredo exibido.');
 let value=match[2].trim();
 if((value.startsWith('"')&&value.endsWith('"'))||(value.startsWith("'")&&value.endsWith("'")))value=value.slice(1,-1);
 if(value.includes('${')||value.includes('\n')||value.includes('\r'))throw Error('Interpolação/multilinha não suportada; forneça valores literais.');
 if(values.has(match[1]))throw Error('Chave duplicada no .env: '+match[1]);
 values.set(match[1],value);
}
for(const key of ['CONNECTME_DATABASE_URL','CONNECTME_MASTER_KEY','CONNECTME_AUDIT_HMAC_KEY','CONNECTME_GUACAMOLE_KEY'])if(!values.get(key))throw Error('Chave obrigatória ausente: '+key);
let db;try{db=new URL(values.get('CONNECTME_DATABASE_URL'))}catch{throw Error('URL PostgreSQL inválida; valor não exibido.')}
if(!['postgres:','postgresql:'].includes(db.protocol)||db.hostname!=='postgres'||db.pathname!=='/connectme'||db.username!=='connectme'||(db.port&&db.port!=='5432')||!db.password)throw Error('A URL deve apontar para connectme@postgres:5432/connectme e incluir senha.');
// The application's working DSN is the authority; do not rotate encryption keys
// or modify credentials of an already initialized PostgreSQL cluster.
values.set('POSTGRES_PASSWORD',decodeURIComponent(db.password));
const folder=resolve('.local');mkdirSync(folder,{recursive:true,mode:0o700});
const output=resolve(folder,'connectme-'+environment+'.env');
writeFileSync(output,[...values].map(([key,value])=>key+'='+value).join('\n')+'\n',{mode:0o600,flag:'wx'});
console.log('Preparado '+output+' (0600); senhas alinhadas à URL, chaves preservadas. Nenhum recurso aplicado.');
