# Kubernetes — staging e estrutura dev/pro

Para o deploy Fleet atual e correção do namespace connectme-pro, siga primeiro
[recuperação Fleet](FLEET.md), incluindo o arquivo de Secret local validado.
Os exemplos genéricos de importação abaixo não substituem essa preparação.

Estrutura inspirada em `../cialinux`: `overlays/dev` e `overlays/pro`, Ingress
NGINX, cert-manager, imagem Docker Hub e seleção de nós `workload=dev/pro`.
Uma base comum evita divergência entre os domínios. Nenhum recurso foi aplicado.

## Arquitetura e limites

- `Deployment/connectme`: um Pod, três contêineres. API na porta 8081,
  Guacamole em 8080 e guacd em 4822. Só a API é exposta pelo Ingress.
- O Service interno `guacamole` mantém compatibilidade com o adaptador atual;
  guacd é acessado em loopback. O Pod compartilha um emptyDir de memória de
  512 MiB entre API/guacd, com UID/GID 1000. Uso conta como memória do Pod.
- Uma réplica e estratégia Recreate: leases estão em memória. Não configurar
  HPA ou rolling update antes de externalizar o estado e a propriedade das sessões.
  Atualização/recriação interrompe sessões e perde os arquivos temporários.
- PostgreSQL em StatefulSet, PVC de 5 GiB com `hcloud-volumes-encrypted`.
  Retenção de PVC ao remover/reduzir StatefulSet; apagar o PVC explicitamente
  continua perigoso, pois a StorageClass tem reclaimPolicy Delete.
- Probes e recursos por contêiner. Sem hostNetwork, portas públicas de guacd,
  token de ServiceAccount ou privilégios de administrador.
- NetworkPolicies limitam entrada ao ingress-nginx, tráfego interno da aplicação,
  DNS e saídas LAN em TCP 22/3389. Dependem do enforcement do CNI; não criam VPN.
  Outro destino/porta precisa de revisão explícita da política.

O staging consultado tem Pods em 10.244.0.0/16, Service Kubernetes 10.43.0.1,
NGINX e issuer `letsencrypt-production-cialinux` disponíveis. Confira esses
valores em outro cluster. Os CIDRs internos permanecem bloqueados no aplicativo.

## Definições obrigatórias antes de expor

1. Domínios confirmados: `connectme.dev.cialinux.com` (dev) e
   `connectme.cialinux.com` (produção). Ambos devem ter registro A para
   `128.140.29.148`, destino público do Ingress. O DNS não é alterado por estes manifests.
2. A allowlist autoriza `94.62.108.14/32`, saída pública da rede do operador.
   O IP do Ingress não é uma origem de cliente. NGINX deve receber o IP real
   do cliente; não amplie a allowlist para contornar SNAT/proxy mal configurado.
3. Publicar apenas a imagem ConnectMe e fornecer `cred-dockerhub` no namespace
   se privada. Guacamole e guacd usam imagens públicas oficiais 1.6.0, sem build.
   Nenhuma imagem foi publicada por esta preparação.
4. O cluster deve alcançar 192.168.1.200:3389 e 192.168.1.248:22 por rota/VPN.
   Os manifests não tornam sua LAN acessível a partir da Hetzner.
5. Preservar banco e chaves do Compose. `.env` não é versionado nem embutido
   nos manifests. Não criar novas chaves para um banco já cifrado.
6. `base/ssh_known_hosts` contém a chave pública fixada para o SSH de teste.
   Ao cadastrar ou rotacionar chaves verificadas, mantenha esse arquivo alinhado
   com `config/ssh_known_hosts`. A mudança altera o hash do ConfigMap e provoca
   recriação do Pod na próxima aplicação do overlay, encerrando sessões abertas.

Use issuer de produção para um certificado confiável também em dev: certificados
Let's Encrypt staging não são confiáveis no navegador e não resolvem o requisito
de HTTPS confiável para clipboard. O cert-manager ainda depende de DNS e challenge.

## Renderizar e validar — sem mutação

```sh
kubectl kustomize overlays/dev
kubectl kustomize overlays/pro
kubectl kustomize overlays/dev | kubectl --kubeconfig "$HOME/.kube/config-staging" create --dry-run=client --validate=strict -f - -o name
```

Use `$HOME/.kube/config-staging`, não `\~/.kube/config-staging` (o til escapado
não é expandido pelo shell). Os comandos abaixo são para o operador executar;
não foram executados pelo assistente.

## Imagens

Guacamole usa `guacamole/guacamole:1.6.0` e `guacamole/guacd:1.6.0` diretamente.
Somente o código próprio (painel/API ConnectMe) requer imagem própria; essas
imagens públicas não contêm o ConnectMe. Referência:
[imagens oficiais Apache](https://guacamole.apache.org/doc/gug/guacamole-docker.html).

```sh
docker build --target server -t virgiliofilhos/connectme-server:dev .
docker push virgiliofilhos/connectme-server:dev
```

Para atualizações posteriores, prefira tags imutáveis e altere `images.newTag`
no overlay correspondente. Publicar novamente a mesma tag não provoca rollout
por si só. O overlay pro começa em latest; fixe uma versão/digest validado antes
do rollout de produção. O workflow publica versão, latest e dev da mesma imagem;
veja [CI da imagem](IMAGE_CI.md).

## Migrar o banco existente antes de iniciar a aplicação

1. Faça backup seguro do banco e do `.env`. Para corte definitivo, interrompa
   escritas no Compose antes do último dump; não opere duas bases independentes
   em paralelo esperando sincronização automática.
2. Crie namespace e Secret com os valores existentes. Confira que
   `CONNECTME_DATABASE_URL` aponta para `postgres:5432/connectme`, com utilizador
   connectme e senha igual a POSTGRES_PASSWORD. As chaves obrigatórias incluem
   CONNECTME_MASTER_KEY, CONNECTME_AUDIT_HMAC_KEY e CONNECTME_GUACAMOLE_KEY.

```sh
kubectl --kubeconfig "$HOME/.kube/config-staging" apply -f overlays/dev/namespace.yaml
kubectl --kubeconfig "$HOME/.kube/config-staging" -n connectme-dev create secret generic connectme-runtime --from-env-file=.env --dry-run=client -o yaml | kubectl --kubeconfig "$HOME/.kube/config-staging" apply -f -
```

O pipe contém segredos: não redirecione a logs, não salve no Git e não publique
seu resultado. Configure o imagePullSecret separadamente se necessário.

3. Para a PRIMEIRA instalação em namespace novo, aplique a infraestrutura do
   próprio overlay, sem Deployment/Ingress (o comando usa `jq`). Assim banco,
   seleção de nó e políticas são idênticos ao ambiente definitivo. São apenas
   dois overlays, dev/pro; não existe overlay de migração.
   Confirme que nenhum Deployment ConnectMe já está ativo nesse namespace.
   Se já existir, este procedimento de base vazia não se aplica: planeje a
   restauração com indisponibilidade e backup antes de continuar.

```sh
kubectl kustomize overlays/dev | kubectl --kubeconfig "$HOME/.kube/config-staging" create --dry-run=client -f - -o json | jq -s '{apiVersion:"v1",kind:"List",items:[.[] | select(.kind != "Deployment" and .kind != "Ingress")]}' | kubectl --kubeconfig "$HOME/.kube/config-staging" apply -f -
kubectl --kubeconfig "$HOME/.kube/config-staging" -n connectme-dev rollout status statefulset/postgres --timeout=180s
```

4. Exemplo de dump/restore para a base NOVA e vazia. Não use pg_restore sobre
   uma base existente com dados importantes; confirme o namespace/destino.

```sh
umask 077
mkdir -p .local
docker compose exec -T postgres pg_dump -U connectme -d connectme -Fc > .local/connectme-cutover.dump
kubectl --kubeconfig "$HOME/.kube/config-staging" -n connectme-dev exec -i postgres-0 -- pg_restore --exit-on-error --no-owner --no-privileges -U connectme -d connectme < .local/connectme-cutover.dump
```

5. Com banco restaurado, chaves preservadas, imagens publicadas, DNS/TLS e
   allowlist revisados, inicie a aplicação:

```sh
kubectl --kubeconfig "$HOME/.kube/config-staging" apply -k overlays/dev
kubectl --kubeconfig "$HOME/.kube/config-staging" -n connectme-dev rollout status deployment/connectme --timeout=240s
kubectl --kubeconfig "$HOME/.kube/config-staging" -n connectme-dev get pods,pvc,svc,ingress,certificate
```

## Verificar por domínio

```sh
kubectl --kubeconfig "$HOME/.kube/config-staging" -n connectme-dev logs deployment/connectme -c server --tail=80
kubectl --kubeconfig "$HOME/.kube/config-staging" -n connectme-dev logs deployment/connectme -c guacamole --tail=80
kubectl --kubeconfig "$HOME/.kube/config-staging" -n connectme-dev logs deployment/connectme -c guacd --tail=80
kubectl --kubeconfig "$HOME/.kube/config-staging" -n connectme-dev exec deployment/connectme -c guacd -- nc -z -v -w 3 192.168.1.200 3389
```

Pod Running não garante acesso à LAN. Teste login, CRUD, RDP/SSH, reconexão,
clipboard e arquivos depois da migração. Manifests renderizados e validados
não são homologação do workload rodando no cluster. Não há promessa de HA nesta fase.

## Validações realizadas nesta preparação

- Os dois overlays (`dev`, `pro`) renderizam e
  passam em `create --dry-run=client --validate=strict`. Nenhum foi aplicado.
- Na stack Compose isolada, Chromium colou texto via Ctrl+V no Bloco de Notas
  de 192.168.1.200 e recebeu o mesmo texto via Ctrl+C, com comparação exata.
- Upload/download de arquivo temporário passou com comparação dos bytes.
  Isso não equivale a copiar arquivos pelo clipboard do Windows Explorer.
- O Windows salvou um documento de teste na unidade redirecionada e o download
  retornou o conteúdo exato, usando a imagem oficial guacd, sem customização.
- O Windows abriu o arquivo enviado pelo navegador na unidade ConnectMe e
  devolveu seu texto via Ctrl+C, com comparação exata. A colagem RDP aguarda
  250 ms após o envio do texto para reduzir a corrida de atualização CLIPRDR;
  isso é uma mitigação de timing, não uma confirmação de prontidão do Windows.
- Regressão de clipboard passou: gesto nativo, foco/permissões, isolamento de
  sessões, encaminhamento de imagem para arquivo e remoção dos listeners.

O teste em localhost usa contexto seguro. Para o navegador dos utilizadores,
configure HTTPS confiável e permissões de clipboard. Screenshots/arquivos são
enviados à unidade ConnectMe; colar imagens diretamente no Paint/Word e copiar
arquivos de volta pelo clipboard nativo ainda não estão implementados.

## Atualização local do Compose

Somente a aplicação precisa ser reconstruída. Execute no projeto quando puder
encerrar as sessões atuais:

```sh
docker compose --env-file .env up -d --build --wait
```

O novo volume `transfer-data-v2` usa UID/GID 1000 da imagem oficial. O antigo
volume temporário não é reaproveitado nem removido automaticamente; os arquivos
de sessões antigas não são migrados. Baixe o que precisar antes da atualização.
Não execute `down --volumes`: isso também apagaria o banco. PostgreSQL, `.env`
e credenciais existentes são preservados pela atualização acima.
