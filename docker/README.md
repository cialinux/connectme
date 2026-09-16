# Instalação Docker

Este diretório usa a imagem pública existente, sem build local. Kubernetes
permanece em base/ e overlays/ na raiz. Use a imagem atualizada com a migration SSH.

```sh
cd docker
node ../scripts/init-env.mjs .env
# Gera chaves e senha consistentes; recusa sobrescrever arquivo existente.
docker compose --env-file .env up -d --pull always --wait
```

Mantenha o nome do projeto connectme e os volumes ao migrar uma instalação
existente. Use o .env existente: não gere novas chaves para um banco já cifrado.
O compose.yaml da raiz é um atalho compatível que usa o .env da raiz.
Ao iniciar dentro de docker/, use --env-file ../.env para reutilizar essa configuração.
Nunca execute down --volumes em uma instalação com dados importantes.

O Docker precisa de acesso à rede dos destinos. Apenas a porta web 8080 é publicada;
banco/guacd não são expostos ao host. Login e TLS/validação de identidades não são
substituídos por abertura de rede. Use HTTPS no proxy para acesso externo.
