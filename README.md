# ConnectMe

Código, build e manifests na raiz: `base/` e `overlays/{dev,pro}`.
[Diagnóstico e configuração Fleet](docs/FLEET.md).
[Build incremental e publicação latest pelo GitHub Actions](docs/IMAGE_CI.md).

Deploy Kubernetes com overlays dev/pro e migração do Compose:
[roteiro de Kubernetes](docs/KUBERNETES.md). Os manifests não foram aplicados;
Domínios e allowlist estão configurados. DNS, publicação da imagem ConnectMe,
restauração do banco e rota para a LAN devem estar prontos antes do teste no cluster.
Guacamole e guacd usam imagens públicas oficiais, sem build customizado.

**Estado atual:** painel com criação, edição, bloqueio e remoção em `/app`,
e integração oficial Guacamole para testar RDP pela LAN.
O desktop abre dentro do dashboard ConnectMe, com Minimizar, Restaurar,
Tela inteira, Reconectar e Encerrar. Até 8 sessões independentes por login;
tokens do gateway não são armazenados no navegador.
Não representa homologação de produção nem implementação do agente entre redes.
Veja [estado verificável e roteiro de teste](docs/TEST_STATUS.md) antes de testar.
Para inicialização, saúde e diagnóstico por domínio, consulte
[operação](docs/OPERATIONS.md).
O ajuste de Ctrl+C/V e seus limites estão em [clipboard sem agente](docs/CLIPBOARD.md).

ConnectMe é uma central web privada e self-hosted para organizar e acessar máquinas
RDP, SSH e VNC atrás de NAT. O core modular e a base de Identity/RBAC estão ativos.

## O que já funciona

- configuração por ambiente com validação;
- registry de módulos, detecção de ciclos e lifecycle ordenado;
- event bus síncrono isolado e seguro para concorrência;
- PostgreSQL via `pgx`, pool configurável e migrations com checksum/advisory lock;
- servidor HTTP com graceful shutdown, correlation ID e security headers;
- logs JSON, métricas Prometheus e endpoints `/livez` e `/readyz`;
- módulo crítico `system` compilado no binário;
- imagens rootless/read-only e Compose compatível com Docker/Podman;
- testes unitários do config, event bus e registry.
- autenticação local Argon2id, sessões server-side e bloqueio temporário;
- MFA TOTP, recovery codes one-shot e secrets cifrados com AES-256-GCM;
- auditoria encadeada com HMAC-SHA-256;
- roles, permissions, groups e associação de administrador.
- schemas e serviços modulares para locations, hosts, credentials e connections;
- envelope encryption com chave de dados individual por credencial;
- validação de CIDR, porta, destino e proteção SSRF/DNS rebinding.

Agentes, WireGuard e execução VNC continuam pendentes. O terminal SSH web está
integrado com identidade de servidor obrigatória; veja [operação SSH](docs/OPERATIONS.md).
SFTP ainda não está habilitado. Guacamole/guacd 1.6.0
estão integrados para RDP, sem portas próprias publicadas.

## Executar com Docker Compose

### Atualização: texto e arquivos no RDP

Esta versão adiciona permissões separadas para copiar texto, colar texto, enviar
e baixar arquivos. Em **Conexões → Editar**, habilite as direções desejadas,
salve e abra uma nova sessão. Use **Texto e arquivos** na janela remota.

- Texto local: cole no painel, clique **Enviar texto** e use Ctrl+V no Windows.
- Texto remoto: use Ctrl+C no Windows; selecione o texto recebido no painel e
  copie manualmente. Os botões de acesso ao clipboard local exigem HTTPS e
  autorização do navegador; o fluxo manual funciona no endereço HTTP atual.
- Arquivos enviados aparecem no Windows em `\\tsclient\ConnectMe`. Para baixar,
  copie um arquivo do Windows para a raiz dessa unidade e atualize a lista.
- Limite de 32 MiB por arquivo na API, 128 MiB por sessão para novos uploads
  pelo painel e 512 MiB de armazenamento temporário compartilhado entre sessões.
  Escritas diretas do Windows estão sujeitas ao limite global, não à quota de upload.
- A lista apresenta arquivos na raiz, não pastas. Nomes existentes não são
  sobrescritos pelos uploads. Cada sessão tem seu próprio diretório.

**Os arquivos temporários são apagados ao encerrar, reconectar, expirar ou
reiniciar o serviço.** Copie o que precisa guardar para uma pasta permanente
antes disso. Não há recuperação desses arquivos. Minimizar não encerra a sessão.
Eventos de transferência são auditados sem o conteúdo; nomes podem aparecer
nos caminhos dos logs HTTP. O limite de texto de 64 KiB é aplicado pelo painel.

Para atualizar uma instalação existente, preserve `.env` e o banco e execute:

```sh
docker compose --env-file .env config --quiet
docker compose --env-file .env up -d --build
```

Nesta atualização não use `--no-deps server`: o guacd também precisa receber
o volume temporário compartilhado e a configuração de utilizador. Não use
`down -v`. Nenhuma alteração de chaves ou de `.env` é necessária.
O Compose constrói uma imagem derivada do guacd oficial, com conta não-root
e diretório pessoal gravável exigido pelo FreeRDP. Não substitua essa conta
por um UID numérico sem registro em `/etc/passwd`.

Diretório: `/home/vf/docs/pessoal/git/k8s/site-k8s/connectme`

```sh
# Somente instalação nova; NÃO sobrescreva o .env existente:
cp .env.example .env
# edite .env e substitua as duas ocorrências da senha
# gere duas chaves diferentes: openssl rand -base64 32
# configure CONNECTME_MASTER_KEY e CONNECTME_AUDIT_HMAC_KEY
# gere também: openssl rand -hex 16
# configure CONNECTME_GUACAMOLE_KEY (chave diferente das anteriores)
docker compose --env-file .env config --quiet
docker compose --env-file .env up --build
```

Em outro terminal:

```sh
curl -fsS http://127.0.0.1:8080/livez
curl -fsS http://127.0.0.1:8080/readyz
curl -fsS http://127.0.0.1:8080/metrics
```

Pare com `docker compose --env-file .env down`. O banco permanece no volume
`connectme_postgres-data`.

## Testes sem instalar Go

```sh
docker run --rm -v "$PWD:/src:ro" -w /src docker.io/library/golang:1.24.6-alpine3.22 go test ./...
```

Com Go instalado: `make test && make vet`.

## Primeiro administrador

Em banco vazio, o servidor cria automaticamente:

```text
utilizador: admin
senha: admin
```

Esse bootstrap acontece somente quando não existe utilizador. O primeiro login
exige a troca por senha de no mínimo 12 caracteres e revoga a sessão inicial. O
servidor nunca redefine a senha em reinícios. Depois acesse `http://IP:8080/`.
Para produção, configure TLS e `CONNECTME_COOKIE_SECURE=true`.

## Configuração

| Variável | Padrão | Uso |
|---|---|---|
| `CONNECTME_DATABASE_URL` | obrigatório | DSN PostgreSQL |
| `CONNECTME_ENV` | `development` | nível de log |
| `CONNECTME_HTTP_ADDRESS` | `:8080` | bind HTTP interno |
| `CONNECTME_MODULES` | `system=true` | lista `nome=bool` |
| `CONNECTME_DATABASE_MIGRATE_ON_START` | `true` | migrations no bootstrap |

Não use `sslmode=disable` fora da rede interna de desenvolvimento.

## Arquitetura

Consulte [ARCHITECTURE.md](docs/ARCHITECTURE.md). O core não contém regras de
domínio. Cada módulo registra contratos e depende apenas de interfaces públicas.

## Estado das fases

- Fase 0: arquitetura aprovada.
- Fase 1: implementada e validada.
- Fase 2: base identity, MFA, audit e authorization implementada.
- Fase 3: fundação de locations, hosts, credentials e connections implementada;
  APIs e painel permitem editar, bloquear e remover cadastros.
- Marco RDP/LAN: autenticação Guacamole e WebSocket integrados;
  homologação com o Windows real deve seguir [o roteiro](docs/TEST_STATUS.md).
