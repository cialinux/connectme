# Arquitetura da Fase 1

```text
cmd/connectme-server -> bootstrap -> config/database/httpserver
                                  -> events -> module registry -> system module
                                  -> logging/metrics
cmd/connectme-migrate -> database migration runner -> embedded SQL
```

O registry ordena o DAG e executa Register/Start/Stop. Migrations são imutáveis e
identificadas por módulo, versão e SHA-256. O readiness agrega health dos módulos
críticos; liveness comprova apenas que o processo HTTP responde. O shutdown deixa
de aceitar HTTP, encerra módulos em ordem inversa e fecha o pool.

Rotas usam o router da biblioteca padrão Go 1.22+. Métricas foram implementadas no
formato Prometheus sem dependência externa. A única dependência Go da Fase 1 é
`pgx/v5`, escolhida pelo protocolo PostgreSQL nativo, pool e suporte a context.

O PostgreSQL não é publicado no host. O servidor é publicado somente em loopback
no Compose até existir proxy TLS na fase de deployment. Containers executam como
API com UID 1000 (alinhado à imagem oficial guacd para transferências privadas),
read-only, sem privilege escalation. O utilitário de migração mantém UID 10001.

## Domínio administrativo

Os módulos `locations`, `hosts`, `credentials` e `connections` possuem schemas e
serviços próprios. Hosts obtêm redes autorizadas somente pelo contrato público de
locations. Credentials usa envelope encryption: uma DEK CSPRNG por secret cifra o
valor com AES-256-GCM, e a master key externa ao banco cifra a DEK. Connections
guarda apenas UUIDs publicados pelos módulos, protocolo e opções não secretas.

Antes de uma sessão, todos os resultados DNS são validados contra os CIDRs da
localização e denylist; qualquer resultado inválido rejeita a operação. O IP aceito
é fixado para impedir DNS rebinding.
