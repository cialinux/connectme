# Kubernetes: preparação local e dois bundles Fleet

O Docker permanece inalterado. Para Kubernetes, não é mais necessário gerar
`.env` na raiz nem criar Namespace e Secret em comandos separados.

## Dev — instalação nova

Na raiz do projeto:

```sh
node scripts/prepare-k8s-env.mjs dev
kubectl --kubeconfig "$HOME/.kube/config-staging" apply -f .local/kubernetes/dev/bootstrap.yaml
```

## Produção — instalação nova

```sh
node scripts/prepare-k8s-env.mjs pro
kubectl --kubeconfig "$HOME/.kube/config-staging" apply -f .local/kubernetes/pro/bootstrap.yaml
```

O script só prepara arquivos; **o operador executa o apply**. Esse apply único
instala o Namespace e o Secret, não os workloads. Depois sincronize no Fleet:

| Ambiente | Paths independentes | Namespace dos dois bundles |
|---|---|---|
| dev | `base` e `overlays/dev` | `connectme-dev` |
| pro | `base` e `overlays/pro` | `connectme-pro` |

Não importe `../../base`. A base contém Deployment, StatefulSet e Services.
O overlay contém somente Ingress. Use releases distintas e configure o namespace
do bundle base também. O Namespace/Secret são preparados pelo operador e não
são reivindicados pelos bundles Fleet. Não misture Fleet com apply manual dos
mesmos workloads.

## Arquivos privados e reexecução

O script cria automaticamente, por ambiente:

```text
.local/kubernetes/dev/
  runtime.env       # senha e chaves da instalação
  bootstrap.yaml    # Namespace + Secret connectme-runtime
```

Para pro, o diretório final é `pro/`. Pastas novas usam 0700 e arquivos 0600.
Tudo está excluído do Git e do contexto Docker por `.local/`. O manifesto usa
JSON, que é YAML válido, e valores base64: **base64 não é criptografia**.
Não publique esses arquivos, não imprima seu conteúdo em logs, nem os envie
para revisão de código. Guarde backup privado junto com o banco.

Repetir a preparação reutiliza a configuração, sem trocar chaves. Divergências
entre manifesto e env causam erro, não sobrescrita. A validação opcional é:

```sh
node scripts/prepare-k8s-env.mjs dev --check
```

O apply é necessário uma vez na instalação. Em atualizações, mantenha o Secret
e apenas sincronize os workloads pelo Fleet. Se apagar somente o Secret,
pode reaplicar o **mesmo manifesto privado** para recuperar os mesmos valores.
Não há PVC extra, recuperação via banco ou Job gerador de chaves no cluster.

## Banco existente e migração do fluxo anterior

**Não gere configuração nova para um PostgreSQL já inicializado.** O script
é local e não inspeciona seu cluster ou PVCs. Um novo clone sem os arquivos
privados não distingue uma instalação vazia de um banco existente.

Se `.local/connectme-dev.env` (ou pro) do fluxo anterior existir, o script
importa esse arquivo automaticamente, validando e preservando os valores.
Se estiver em outro caminho, indique-o explicitamente:

```sh
node scripts/prepare-k8s-env.mjs dev --from /caminho/privado/dev-original.env
```

A senha POSTGRES_PASSWORD e a senha da URL devem coincidir. Uma divergência
é recusada; o script não altera senha do banco. Chaves não podem ser substituídas
arbitrariamente: isso pode tornar credenciais cifradas ilegíveis.
`.env` da raiz e `docker/.env` não são lidos automaticamente, evitando misturar
instalações. Dev e pro novos recebem segredos aleatórios independentes.

Antes de aplicar em namespace existente, confirme que os valores pertencem
àquele banco: `kubectl apply` pode atualizar um Secret existente. Se perdeu
os arquivos privados, recupere seu backup/Secret original antes de continuar.

## Pré-requisitos e aceite

Os manifests usam Ingress NGINX, cert-manager com
`letsencrypt-production-cialinux` e `hcloud-volumes-encrypted`.
Outros clusters precisam adaptar issuer, StorageClass e domínios.
Não há NetworkPolicy padrão. RDP mantém a opção global de ignorar certificados;
SSH registra identidades no banco. Docker não foi alterado por este fluxo.

```sh
kubectl --kubeconfig "$HOME/.kube/config-staging" -n connectme-dev rollout status statefulset/postgres --timeout=180s
kubectl --kubeconfig "$HOME/.kube/config-staging" -n connectme-dev rollout status deployment/connectme --timeout=300s
kubectl --kubeconfig "$HOME/.kube/config-staging" -n connectme-dev get pods,pvc,svc,ingress,certificate
```

Para pro, substitua o namespace. Domínios: `connectme.dev.cialinux.com` e
`connectme.cialinux.com`. Login em banco vazio: **admin/admin**, com troca
obrigatória de senha. Novo deploy não redefine senha de um banco existente.
