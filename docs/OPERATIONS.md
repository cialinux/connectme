# Operação por domínio

## Exceção SSH para o próprio servidor

A autorização do operador pode ser configurada no `.env` com
`CONNECTME_SELF_SSH_TARGET=192.168.1.248:22`. Mantenha
`CONNECTME_REMOTE_DENY_CIDRS=192.168.1.248/32`: a exceção libera somente o
protocolo SSH nesse IP e porta, sem abrir RDP ou portas da aplicação. É opcional
e desativada por padrão. Não ignora autenticação, CIDR da localização, IP fixado
ou bloqueios de host/credencial. Uma lista de bloqueios inválida continua negada.

O terminal SSH web está integrado ao workspace, com autenticação por senha
ou chave privada. O fluxo por senha foi validado com servidor SSH descartável;
formatos de chave privada e prompts de passphrase ainda exigem homologação.
O arquivo `config/ssh_known_hosts`, montado somente para leitura no servidor,
deve conter uma chave pública aprovada por destino: `[IP]:porta tipo base64`.
Para este servidor, a chave foi lida diretamente de `/etc/ssh/ssh_host_rsa_key.pub`
e comparada com a identidade efetivamente anunciada pelo SSH. A chave Ed25519
também existe em disco, mas não é apresentada pelo serviço ativo. Não basta
escolher uma chave pública local sem confirmar qual está habilitada.
Não há aceitação automática de chaves pela rede. Destinos desconhecidos ou com
chave divergente são recusados. Alterar a entrada invalida autorizações em uso.
Inclua chaves de outros dispositivos somente após verificar sua autenticidade
por canal independente. Chaves públicas não são senhas nem chaves privadas.

SFTP/upload/download SSH estão desativados neste marco; não se confundem com
a unidade de arquivos RDP. Texto pode ser enviado pelo painel de clipboard
quando permitido; a interação do terminal deve ser homologada no destino real.
Nenhuma credencial existente foi alterada. Faça o deploy completo para incluir
o novo bind mount de `config/ssh_known_hosts`, depois atualize `/app`.

| Domínio | Sinal de saúde | O que não comprova |
|---|---|---|
| PostgreSQL | pg_isready | Credenciais de uma máquina Windows |
| Core/API | /livez e /readyz | Sucesso de uma sessão RDP |
| Gateway HTTP | GET /guacamole/ | Autenticação no destino |
| Transporte RDP | TCP local 4822 | Senha, políticas NLA ou logon remoto |

O Compose aguarda PostgreSQL saudável e gateway HTTP saudável antes do core;
o gateway aguarda guacd saudável. As sondas são locais a cada domínio e não
dependem da disponibilidade de uma máquina de teste. Assim, um Windows offline
não coloca o banco ou o painel em ciclo de reinício.

Todos os serviços usam `restart: unless-stopped`. Essa política recupera a saída
do processo, mas não reinicia automaticamente um processo apenas `unhealthy`.
Uma parada manual continua respeitada. As dependências regulam a inicialização
pelo Compose, não são um supervisor contínuo depois de iniciado.

## Atualizar sem apagar dados

```sh
docker compose --env-file .env config --quiet
docker compose --env-file .env up -d --build --wait --wait-timeout 180
docker compose ps -a
```

Não use `down -v`. Preserve o `.env` e os backups do banco. Uma recriação de
server/guacd interrompe sessões remotas e pode descartar arquivos temporários.
Não há migration de banco nesta correção operacional.

## Diagnóstico sem mudanças

```sh
docker compose ps -a
docker compose logs --since=10m --tail=100 server guacamole guacd
curl -fsS http://127.0.0.1:8080/readyz
```

`running` não é sinônimo de `healthy`; `starting` é espera da sonda, não queda.
O intervalo herdado de cinco minutos do guacd foi substituído por dez segundos.

`Authentication failure (invalid credentials?)` vem do destino RDP: revisar
credencial vinculada, utilizador/domínio, senha e políticas da conta Windows.
Não trocar a senha de login do ConnectMe pensando que é a senha Windows.
Não repetir tentativas em série, pois podem bloquear a conta. Não desabilitar
NLA nem ampliar exceções de certificados para contornar autenticação.

Em 14/09/2026, os quatro contêineres ativos foram inspecionados: todos running,
zero reinícios e nenhum OOM. A tentativa RDP mais recente falhou por autenticação,
não por saída do guacd. Metadados confirmados: 192.168.1.200:3389, admin, domínio
vazio. A senha armazenada não foi exibida nem alterada durante esse diagnóstico.
