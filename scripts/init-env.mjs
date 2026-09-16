// New installations only. Never overwrites keys or touches a running database.
import {randomBytes} from 'node:crypto';
import {writeFileSync} from 'node:fs';
import {resolve} from 'node:path';
const output=resolve(process.argv[2]||'.env');
const password=randomBytes(32).toString('hex');
const values={
 POSTGRES_DB:'connectme',POSTGRES_USER:'connectme',POSTGRES_PASSWORD:password,
 CONNECTME_DATABASE_URL:`postgres://connectme:${password}@postgres:5432/connectme?sslmode=disable`,
 CONNECTME_ENV:'development',CONNECTME_HTTP_ADDRESS:':8080',CONNECTME_MODULES:'system=true',
 CONNECTME_MASTER_KEY:randomBytes(32).toString('base64'),
 CONNECTME_AUDIT_HMAC_KEY:randomBytes(32).toString('base64'),
 CONNECTME_GUACAMOLE_KEY:randomBytes(16).toString('hex'),
 CONNECTME_COOKIE_SECURE:'false',CONNECTME_REMOTE_DENY_CIDRS:'',CONNECTME_RDP_IGNORE_CERT:'true'
};
try {
 writeFileSync(output,Object.entries(values).map(([k,v])=>`${k}=${v}`).join('\n')+'\n',{mode:0o600,flag:'wx'});
 console.log('Configuração privada criada (0600). Guarde backup junto com o banco. Nenhum recurso implantado.');
} catch(e) {
 if(e.code==='EEXIST'){console.error('Arquivo já existe; chaves preservadas. Use o arquivo existente para atualizações.');process.exitCode=1}
 else throw e;
}
