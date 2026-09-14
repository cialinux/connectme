# Recuperação do deploy Fleet

## Diagnóstico confirmado no namespace connectme-pro

- postgres-0: CreateContainerConfigError, Secret connectme-runtime ausente.
- API aguardando PostgreSQL no init container; não é erro de build da imagem.
- Release instalada: connectme-pro-base, apenas base, sem Ingress/nodeSelector.
- Overlay pro estava sem a referência ../../base e não podia renderizar patches.
- O nó workload=pro tem taint dedicated=pro:NoSchedule; toleration adicionada.
- POSTGRES_PASSWORD no .env divergia da senha da URL da aplicação; o arquivo
  local preparado usa a senha da URL e preserva as chaves. O .env não é alterado.
- A imagem virgiliofilhos/connectme-server:latest permite pull anônimo. Removida
  a dependência de cred-dockerhub inexistente nesse namespace.

## Ações do operador (o assistente não as executou)

1. Na raiz do projeto, crie o Secret já referenciado usando o .env do Compose.
   Preserve especialmente MASTER_KEY, AUDIT_HMAC_KEY, GUACAMOLE_KEY e senha do
   banco. Não publique .env no Git nem use valores de exemplo. Confirme que
   CONNECTME_DATABASE_URL usa o serviço postgres:5432 e banco connectme.

```sh
node scripts/prepare-k8s-env.mjs pro
kubectl --kubeconfig "$HOME/.kube/config-staging" -n connectme-pro create secret generic connectme-runtime --from-env-file=.local/connectme-pro.env
```

O arquivo .local/connectme-pro.env já foi preparado nesta correção. Não é preciso
rodar o gerador novamente; ele recusa sobrescrita. Somente o comando kubectl fica
para o operador. O arquivo é ignorado pelo Git e Docker, com permissão 0600.

O comando só serve enquanto o Secret está ausente. Não delete um Secret existente
para repetir o procedimento. Para importar os dados do Compose, siga o roteiro
KUBERNETES.md ANTES de liberar a inicialização da API: criar esse Secret agora
permite que a aplicação atual inicialize um banco vazio e o admin de bootstrap.
Não restaure dump sobre esse banco inicializado sem planejamento.

2. Envie as correções ao GitHub. Configure o GitRepo Fleet para usar somente o
   path `.` e o fleet.yaml da raiz, que seleciona overlays/pro. Não inclua base,
   overlays/dev e overlays/pro como bundles concorrentes da mesma instalação.
   Base é uma biblioteca compartilhada, não um ambiente implantável.

ATENÇÃO ao bundle já existente: preserve a release connectme-pro-base e os dados
ao trocar o path. Antes de remover o bundle antigo, configure retenção de recursos
no gerenciamento Fleet e confirme a transferência; não permita que a limpeza do
bundle antigo desinstale a release usada pelo novo. Não execute helm uninstall,
force/takeOwnership nem apague PVCs para contornar conflito. O keepResources deste
novo fleet.yaml não altera retroativamente a retenção do bundle antigo.

O kubeconfig staging é do cluster downstream e não expõe GitRepo/Bundle do
gerenciamento Fleet. A configuração atual de paths/retention deve ser conferida
no Rancher; não foi possível verificar ou alterar essa configuração daqui.

Dev usa outro GitRepo com path `.` e options file `fleet-dev.yaml`. O padrão root
fleet.yaml é produção porque os dois ambientes compartilham o mesmo cluster;
não se infere ambiente por labels de cluster. Há apenas dois overlays.

3. Após reconciliar, confira:

```sh
kubectl --kubeconfig "$HOME/.kube/config-staging" -n connectme-pro get pods,pvc,ingress
kubectl --kubeconfig "$HOME/.kube/config-staging" -n connectme-pro get events --sort-by=.lastTimestamp
kubectl --kubeconfig "$HOME/.kube/config-staging" -n connectme-pro logs deployment/connectme -c server --tail=60
```

O PVC existente é preservado. O deslocamento do PostgreSQL para o worker pro
pode aguardar detach/attach do volume. Não altere o template imutável do StatefulSet
nem tente reduzir o PVC: o template é 5Gi e o volume provisionado observado é 10Gi.

HTTPS, DNS e rota/VPN à LAN continuam necessários após os Pods ficarem saudáveis.
Nenhum apply/patch, commit/push ou alteração da release foi executado.

Referência: https://fleet.rancher.io/explanations/gitrepo-content
