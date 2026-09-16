# Kubernetes — dev/pro

Siga [FLEET.md](FLEET.md) para instalação, Secret e recuperação.

## Organização e dependências

- `base/`: Deployment, StatefulSet PostgreSQL, Services.
- `overlays/dev/`: Ingress de `connectme.dev.cialinux.com`.
- `overlays/pro/`: Ingress de `connectme.cialinux.com`.
- `docker/`: instalação alternativa por Docker Compose.

Fleet usa dois bundles independentes por ambiente: `base` e o overlay escolhido.
Configure o namespace de destino de ambos. Não importe `../../base` e não use
patches de workloads nos overlays: esses recursos pertencem ao outro bundle.
`namespace.yaml` é auxiliar de preparação manual, não renderizado pelo overlay,
para não disputar propriedade Helm com o namespace criado pelo Rancher.

Antes de iniciar: namespace e Secret `connectme-runtime` devem existir.
Preserve as chaves e a senha ao restaurar um banco. Não versione segredos.
O cluster precisa de Ingress NGINX, cert-manager com ClusterIssuer
`letsencrypt-production-cialinux` e StorageClass `hcloud-volumes-encrypted`.
Ambos os domínios usam o Ingress público em `128.140.29.148`.

## Acesso e arquitetura

O Ingress aceita todas as origens (`0.0.0.0/0,::/0`). Não há NetworkPolicy
nos manifests padrão. O Deployment define `CONNECTME_REMOTE_DENY_CIDRS` vazio.
Login, autorização, TLS e verificação de identidade remota continuam ativos.
Isso não cria rotas, VPN, encaminhamento NAT nem configura proxies externos.

Um Pod contém API (8081), Guacamole (8080) e guacd (4822). Somente a API é
publicada pelo Ingress. Guacd é acessado em loopback. O Service Guacamole
publica endpoints ainda não prontos para evitar ciclo de readiness.

Uma réplica e estratégia Recreate: sessões/leases não suportam distribuição
entre réplicas nesta versão. Atualizações encerram sessões. Transferências usam
emptyDir de memória de 512 MiB, sem persistência após recriação do Pod.

PostgreSQL usa PVC retido após exclusão/redução do StatefulSet. O pedido é 5 GiB;
o provisionador pode arredondar para seu tamanho mínimo. Excluir PVC/volume pode
apagar dados: mantenha backups testados e cópia segura das chaves da aplicação.
Os workloads possuem probes e limites, sem token de ServiceAccount.
Agendamento e imagens pertencem à base, não aos overlays independentes.

Guacamole e guacd usam imagens oficiais 1.6.0. A imagem ConnectMe está definida
em `base/deployment.yaml`; veja [IMAGE_CI.md](IMAGE_CI.md). Republicar uma tag
não recria automaticamente os Pods. Para rollout reproduzível, atualize a
referência da imagem na base para uma versão/digest validado.

## Validação sem implantação

```sh
kubectl kustomize base
kubectl kustomize overlays/dev
kubectl kustomize overlays/pro
node scripts/prepare-k8s-env.mjs pro --check
```

Não imprima o Secret em logs. O arquivo preparado fica em `.local/`, ignorado
pelo Git. FLEET.md contém o comando do operador para criar o Secret.

## Aceite após deploy

1. PostgreSQL pronto, PVC Bound e ConnectMe com três containers prontos.
2. Ingress instalado pelo overlay, certificado Ready e HTTPS acessível externamente.
3. Login, cadastros, RDP/SSH, reconexão e transferências testados.

A identidade SSH agora é registrada no banco; consulte [SSH_TRUST.md](SSH_TRUST.md).
Clipboard de texto e transferência pela unidade ConnectMe não equivalem a
copiar arquivos bidirecionalmente pelo clipboard do Explorer.

Manifests renderizados não comprovam operação ponta a ponta. Esta instalação
não oferece alta disponibilidade. Restaurações exigem backup do banco e chaves
originais, evitando escritas simultâneas em bases divergentes.
