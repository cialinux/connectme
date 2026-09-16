# Validação da limpeza e instalação nova

Executado em stack Docker isolada, com banco vazio e imagem construída localmente:

- Inicialização saudável de PostgreSQL, API, Guacamole e guacd.
- Login inicial admin/admin e troca obrigatória de senha pelo navegador.
- SSH pelo guacd oficial: descoberta sem senha, registro de chave no banco,
  autenticação, terminal, teclado, minimizar/restaurar e reconexão.
- Servidor SSH com Ed25519 e RSA: identidade consistente entre descoberta e gateway.
- Reutilização da identidade, detecção de rotação, recusa de aprovação desatualizada
  e atualização explícita persistida no banco.
- Suíte Go e análise estática; manifests base/dev/pro renderizados separadamente.

Não foi feita instalação Kubernetes nova nem teste RDP real nesta limpeza.
Validação local não garante rotas, DNS, StorageClass, certificados de Ingress ou
credenciais de outro ambiente. A imagem nova precisa ser publicada antes de
usar o Compose padrão ou sincronizar os manifests sem mounts SSH antigos.

Testes de browser dependem de Playwright. `bootstrap-browser.mjs` exige banco
vazio. `ssh-browser.mjs` usa SSH_FIXTURE_IP. `ssh-trust-api.mjs` reinicia somente
um fixture explicitamente nomeado da stack descartável. Nunca aponte esses
testes destrutivos a dados de produção.
