# Estado verificável — edição e RDP/LAN

O projeto ainda não cumpre todo o prompt inicial. Não há evidência de um ano de
operação nem certificação de produção. Este marco prepara o teste RDP pela LAN,
com Guacamole e guacd oficiais 1.6.0.

## Disponível

Correção de privacidade: o campo de credencial usa `type=password`, sem
revelação automática. Chaves privadas podem ser importadas de arquivo sem
exibir seu conteúdo, preservando quebras de linha. A edição continua vazia e
preserva o segredo existente quando não preenchida. Validado em Chromium por
`tests/credential-privacy-browser.mjs` com dados fictícios.

SSH agora tem execução no workspace; VNC continua identificado como indisponível.
Correção no destino real: a entrada Ed25519 foi substituída pela chave RSA
local, conferida com o SSH ativo. O teste `tests/sshverify` confirmou a identidade
de 192.168.1.248:22 e abortou antes de autenticar qualquer utilizador. O arquivo
foi atualizado preservando o inode do bind mount; hashes no host e contêiner
coincidiram. Não foi necessário reiniciar serviços. A autenticação de `vf`
continua dependendo da senha e das políticas do destino.
A exceção de destino próprio permite apenas SSH no IP:porta explicitamente
aprovado. A identidade do SSH é obrigatória em `config/ssh_known_hosts`.
O teste `tests/ssh-browser.mjs` validou, com o servidor descartável
`tests/sshfixture/main.go`, login por senha, renderização, eco de teclado,
minimizar/restaurar e reconexão. O fixture nunca executa comandos do sistema.
O utilizador real `vf` não foi autenticado nesses testes; a homologação dele
permanece para depois do deploy. SFTP não está habilitado. Autenticação por
chave privada está integrada, mas não foi homologada neste teste por senha.

- Criar, listar, editar, bloquear e remover localizações, redes, hosts,
  credenciais, conexões e utilizadores. Remoções são lógicas; não há restauração
  pelo painel. Remover credencial também apaga seu conteúdo cifrado do registro.
- Editar endereço do host com validação CIDR e fixação do IP ao ativar.
- Editar utilizador/domínio de credenciais. Segredo vazio na edição preserva o
  atual; segredo preenchido gera nova chave de dados e nova cifra.
- Proteção contra remover/bloquear a própria conta. Redefinir senha de outro
  utilizador revoga sessões e exige troca no próximo login.
- Botão **Abrir** mantém o dashboard e apresenta um desktop integrado, usando
  a biblioteca oficial de renderização Guacamole (sem redirecionar para sua UI).
- Controles Minimizar, Restaurar pelo dock, Tela inteira, Reconectar e Encerrar.
  Minimizar mantém a conexão. Encerrar desconecta; não faz logoff no Windows.
- Até 8 autorizações independentes por sessão ConnectMe, cada uma com 30 minutos
  de validade. Abrir outra conexão não invalida a anterior. Tokens e envelopes
  Guacamole ficam somente no servidor, sem localStorage nem URLs de autenticação
  no navegador. Estado antigo do Chrome não é reutilizado.
- Atualizar/sair da página encerra o transporte; ao retornar, abrir/reconectar
  cria autorização nova. Minimizar é o fluxo para manter a sessão ativa.
  A política do próprio Windows pode limitar conexões simultâneas à mesma conta.
- Revalidação de host/CIDR, conexão, credencial e permissão antes do tráfego;
  checagem a cada 2 segundos para encerrar WebSockets em uso quando revogados.
  Rotacionar senha ou editar dados invalida a autorização anterior.
- Auditoria de mutações administrativas sem corpo do pedido nem segredos.
  Falha de auditoria ainda não reverte a mutação de domínio.

## Validações em ambiente isolado

Correção da colagem: teste integrado com teclado Guacamole real mostrou a
interceptação de Ctrl+V antes do evento paste. Captura movida para window e
atalho SSH corrigido. O fixture recebeu os bytes de `SSH-paste-test`; também
foi verificado que contextmenu sobre o terminal não é cancelado.

No diagnóstico RDP desta rodada, houve timeout TCP para 192.168.1.200:3389,
EHOSTUNREACH e vizinhança ARP FAILED em ens192. A conectividade desse destino
precisa ser restabelecida; não foi alterada a negociação NLA como contorno.

### Histórico: correção da regressão de identidade do guacd

A configuração de transferências usava `user: 10001:10001` sobre a imagem
oficial, sem conta correspondente em `/etc/passwd`. No deploy real, FreeRDP
reportou home `/` não gravável e falha de negociação NLA. A imagem derivada em
`docker/guacd.Dockerfile` cria uma conta não-root com UID/GID 10001 e diretório
privado gravável, mantendo as permissões do volume de transferências.

O guacd corrigido foi testado isoladamente contra `192.168.1.200`: negociação
NLA, logon RDPDR e duas instruções de imagem recebidas. Não foram enviados
teclado ou comandos ao Windows. A exceção de certificado permanece restrita
ao destino de laboratório; não foi ampliada. Esse teste não homologa todo o
fluxo de clipboard e arquivos. `tests/guacd-runtime-smoke.sh` verifica a conta
e o diretório para impedir a repetição dessa regressão.

Essa imagem derivada foi substituída pela imagem pública oficial 1.6.0.
API e volume temporário agora usam UID/GID 1000, compatível com a conta guacd
já existente. Não se injeta um UID desconhecido na imagem oficial. O teste de
runtime verifica essa identidade. Só a aplicação ConnectMe precisa de build.
O Compose usa `transfer-data-v2` para não reutilizar as opções UID 10001 do
volume antigo; o banco e as chaves não mudam. O deploy ativo não foi alterado.

- `go test ./...`, `go vet ./...`, sintaxe JavaScript e construção Docker.
- Banco vazio/migrations, login, troca inicial e edição `.246` para `.200`.
- Guacamole real: envelope JSON aceito, token emitido e conexão autorizada listada.
- Rotação invalida autorização; host bloqueado impede abertura; token anterior
  rejeitado; exclusão dos cadastros de teste.
- Edição/bloqueio/remoção de utilizador e proteção da conta do próprio operador.
- Chromium headless: armazenamento antigo simulado, dashboard preservado,
  minimizar/restaurar/reconectar/encerrar/reabrir, duas janelas e navegação Voltar.
  Sessão de outro login é rejeitada e fechar uma não revoga a outra.

Após o diagnóstico do certificado, foi realizada uma tentativa autorizada contra
192.168.1.200, com exceção de certificado restrita a esse IP. Chromium recebeu
53 frames WebSocket, incluindo 17 instruções de imagem do desktop. Nenhum comando
ou interação de teclado/rato foi enviado. Isso valida abertura e imagem RDP,
mas não substitui testes de interação, reconexão prolongada ou homologação.
Os testes de regressão com credenciais fictícias continuam usando destino reservado.

No marco do workspace integrado, foi repetido o teste real em 192.168.1.200:
abertura, minimizar/restaurar, reconectar, encerrar/reabrir e navegação Voltar.
O Chromium recebeu 162 frames WebSocket e 62 instruções de imagem ao longo do
teste, incluindo novas imagens após reconexão e retorno à página. Não houve
envio de comandos ao Windows. Duas sessões simultâneas foram testadas com
destinos fictícios; não se presume suporte multiusuário no Windows de destino.

O marco seguinte, de texto e arquivos, adiciona migration de permissões e um
volume temporário compartilhado com guacd. Atualize o Compose completo com
`docker compose --env-file .env up -d --build`, preservando o `.env`.
Feche abas antigas do Guacamole e abra `/app` novamente. O desktop não navega
mais para fora do dashboard. Use Minimizar para manter uma sessão enquanto
trabalha em outra; atualizar/fechar a página interrompe o transporte.

## Novo deploy e teste real

### Texto e arquivos: evidências e limites

Testes automatizados adicionados em `tests/transfers-smoke.mjs` e
`tests/transfers-browser.mjs`: upload/download binário, isolamento entre sessões
e logins, permissões direcionais, revogação após edição, rejeição de nomes
inválidos e sobrescrita, seletor de arquivos e download no Chromium, leitor de
clipboard Guacamole com stream simulado e seleção manual do texto recebido.
Testes Go cobrem armazenamento, links simbólicos, falha parcial, quota e tamanho.

Esses testes não comprovam ainda a cópia de texto e a unidade redirecionada no
Windows real. Após o deploy, habilite as quatro permissões na conexão de teste
e abra uma sessão nova. Em **Texto e arquivos**, envie uma palavra e cole no
Bloco de Notas; copie outra palavra no Windows e confira o painel. Envie um
arquivo pequeno, abra `\\tsclient\ConnectMe` no Explorador e confirme o conteúdo.
Copie um arquivo de teste para a raiz dessa unidade e baixe pelo painel.

As transferências pelo painel têm limites de 32 MiB por arquivo e 128 MiB para
novos uploads por sessão. O tmpfs compartilhado tem 512 MiB no total, inclusive
para escritas diretas do Windows. Não há navegação em subpastas. O clipboard
local programático exige HTTPS; a seleção/colagem manual funciona em HTTP.
Encerrar/reconectar/expirar/reiniciar apaga os arquivos temporários; minimizar
preserva-os. Não use essa unidade como armazenamento permanente.

### Procedimento

Execute você no diretório do projeto, preservando `.env` e o volume PostgreSQL:

```sh
docker compose --env-file .env config --quiet
docker compose --env-file .env up -d --build
docker compose ps
```

Não use `down -v`: apagaria o banco. Faça backup do banco e das chaves antes de
atualizar. As novas migrations são aditivas e aplicadas no início.

1. Acesse `http://192.168.1.248:8080/app` com sua conta existente.
2. **Redes**: confirme CIDR `192.168.1.0/24` ativo na localização do host.
3. **Hosts → Editar**: altere para `192.168.1.200`, Windows, estado Ativo. Salve.
4. **Credenciais → Editar**: informe utilizador Windows e senha real. Domínio
   fica vazio para conta local; use o domínio adequado para conta AD. Senha
   vazia preserva a já cadastrada; ela não aparece na listagem.
5. **Conexões → Editar**: selecione esse host/credencial, RDP, porta 3389,
   estado Ativo. Salve e clique **Abrir**. O desktop aparece dentro do ConnectMe.
   Clique **Minimizar**, abra outro dispositivo e alterne pelo dock de sessões.
6. Confirme imagem, teclado, rato e reconexão. Em outra aba, bloqueie o host:
   a sessão deve cair; reative e abra novamente pelo painel.

## Pré-requisitos e diagnóstico

- Windows com servidor RDP habilitado, conta autorizada e porta 3389 liberada.
- O contêiner guacd precisa alcançar `.200:3389`; firewall/VPN/roteamento podem
  impedir mesmo quando o navegador abre o painel.
- Proxy HTTPS deve permitir WebSocket/Upgrade. O túnel HTTP legado é negado;
  esta integração requer WebSocket.
- `CONNECTME_RDP_IGNORE_CERT=true` ignora a verificação do certificado RDP
  globalmente, sem lista de IPs, inclusive para hosts cadastrados por DNS.
  Kubernetes e Compose habilitam essa opção por decisão do operador.
  `CONNECTME_RDP_INSECURE_CIDRS` foi aposentada e não tem efeito.
  TLS continua cifrado, mas há risco de personificação do destino; use
  `CONNECTME_RDP_IGNORE_CERT=false` e certificados confiáveis para verificar identidade.

- A UI antiga `/guacamole/` redireciona ao painel. Reconexão cria uma nova
  autorização no servidor; não há dependência de refresh de token no navegador.
- `CONNECTME_REMOTE_DENY_CIDRS` bloqueia destinos internos; padrão
  `192.168.1.248/32`. Acrescente CIDRs de infraestrutura/cluster proibidos.
  CIDRs autorizados não substituem firewall egress.
- `/readyz` verifica a aplicação e o HTTP do Guacamole, não a sessão RDP.
  Consulte logs de `server`, `guacamole` e `guacd` localmente; não publique
  tokens, URLs de autorização nem informações sensíveis.
- HTTP 8080 é somente laboratório confiável. Não envie credenciais sensíveis
  por rede não confiável: instale TLS e use `CONNECTME_COOKIE_SECURE=true`.

## Pendências de produção

- Agente, enrollment Linux/Windows, CSAP/mTLS, PKI, WireGuard e acesso entre NATs.
- Execução VNC, transferência SFTP e homologação de autenticação SSH por chave.
- Administração de roles/grupos e escopo por recurso/localização. Utilizadores
  novos não recebem administração automaticamente; atribuição de roles ainda
  não está no painel. RBAC atual é global.
- Auditoria transacional/fail-closed, proteção de replay TOTP, hardening completo
  de egress, concorrência/soak, recuperação de backup e alta disponibilidade.
- TLS empacotado, Helm, air-gap e políticas operacionais de atualização.

## Repetir testes

`tests/remote-smoke.mjs` e `tests/browser-smoke.mjs` usam exclusivamente
`127.0.0.1:18080`, banco descartável e credenciais fictícias. O override
`tests/compose.override.yaml` exige Docker Compose com suporte a `!override`.
O navegador usa Playwright externo via `PLAYWRIGHT_MODULE`; não é dependência
de runtime. Não apontar os testes para o banco real.

Integração conforme [autenticação JSON oficial](https://guacamole.apache.org/doc/gug/json-auth.html)
e [imagens Docker oficiais](https://guacamole.apache.org/doc/gug/guacamole-docker.html).
