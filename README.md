# ConnectMe by cialinux

Central web self-hosted para organizar localizações, redes, hosts, credenciais
e conexões SSH/RDP. Guacamole e guacd usam imagens oficiais; a imagem ConnectMe
contém o painel e a API. VNC, agentes e acesso entre redes sem rota/VPN continuam
pendentes. Não há alta disponibilidade nesta versão.

## Organização

- `base/` e `overlays/dev|pro/`: Kubernetes, dois bundles Fleet independentes.
- `docker/`: instalação Docker Compose com imagens públicas.
- `modules/`, `core/`, `pkg/`, `cmd/`, `migrations/`: aplicação e banco.
- `tests/`, `scripts/`, `docs/`: validação, preparação e operação.
- Dockerfile e `.github/workflows/`: build e publicação da imagem.

## Primeira instalação Docker

Requer Docker Compose e Node.js para gerar a configuração privada:

```sh
cd docker
node ../scripts/init-env.mjs .env
docker compose --env-file .env config --quiet
docker compose --env-file .env up -d --pull always --wait
```

Abra `http://IP-DO-SERVIDOR:8080`. O utilizador inicial é `admin`, senha `admin`;
a troca de senha é obrigatória. Configure HTTPS no proxy para uso externo e
clipboard do navegador. O atalho Compose da raiz usa o `.env` da raiz.

O gerador cria senhas aleatórias e chaves consistentes, com permissão 0600,
sem sobrescrever arquivos existentes. Guarde backup privado junto ao banco.
**Não execute o gerador para substituir chaves de uma instalação existente.**
A imagem pública precisa conter esta versão antes do teste com `--pull always`.

## Primeira instalação Kubernetes

Siga [Fleet](docs/FLEET.md): namespace e Secret preparados antes da sincronização,
paths `base` e `overlays/pro` (ou dev), ambos no namespace de destino.
Não há importação `../../base`. Os manifests atuais usam domínio, issuer e
StorageClass do cluster cialinux: outras instalações devem adaptar esses valores.
O Secret não é versionado; pode ser provisionado por cofre/CI autorizado.

## Acesso e segurança

Sem NetworkPolicy padrão ou lista de IPs obrigatória. Login, autorização,
estado habilitado dos cadastros e redes cadastradas na aplicação continuam
válidos. A ausência de bloqueio não cria rotas ou encaminhamentos NAT.

RDP está configurado para ignorar certificados de qualquer destino, por opção
do operador. TLS permanece cifrado, mas a identidade não é validada. Configure
`CONNECTME_RDP_IGNORE_CERT=false` para exigir certificados confiáveis.

SSH registra a chave no banco no primeiro acesso e solicita confirmação pelo
painel se mudar. Não depende de `ssh_known_hosts` no repositório.
Veja [SSH_TRUST.md](docs/SSH_TRUST.md) para limites e migração.

Credenciais são cifradas; perder ou trocar arbitrariamente as chaves torna dados
inacessíveis. Não publique `.env`, backups ou Secrets. O `.gitignore` não apaga
conteúdo já incluído no histórico Git.

## Operação e testes

Atualizações interrompem sessões; arquivos temporários não são persistentes.
Não use `docker compose down --volumes` em ambientes com dados importantes.

- [Docker](docker/README.md)
- [Kubernetes](docs/KUBERNETES.md)
- [Operação](docs/OPERATIONS.md)
- [Clipboard e limites](docs/CLIPBOARD.md)
- [CI da imagem](docs/IMAGE_CI.md)

Execute `go test ./...` e `go vet ./...`. Os testes de navegador usam Playwright;
testes com serviços devem apontar somente para uma stack descartável.
