# Operação

## SSH

Identidades são registradas no banco ao primeiro acesso; mudanças são aprovadas
no painel. Veja [confiança SSH](SSH_TRUST.md). Não há arquivo de chaves por host
nem configuração especial para acessar o próprio servidor. SFTP permanece
indisponível; o terminal e clipboard de texto estão implementados.

## Atualização

Preserve banco, chaves e Secret. A nova migration registra identidades SSH e
histórico. Use a nova imagem junto com os manifests correspondentes: imagens
antigas ainda dependem de arquivo e não funcionam com os mounts removidos.
Atualizações encerram sessões remotas e descartam transferências temporárias.

## Diagnóstico

No Docker: `docker compose logs --tail=100 server guacamole guacd postgres`.
No Kubernetes: consulte cada container de `deployment/connectme` e `postgres-0`.
Não publique logs com dados sensíveis. Teste login, persistência, SSH/RDP e
transferências após atualizar; estado Running não comprova todos esses fluxos.
