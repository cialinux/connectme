# Fleet: dois bundles independentes

## Instalação nova em dev — sequência completa

Execute na raiz da cópia do projeto (não dentro de `docker/`). Estes comandos
de preparação só são apropriados para banco novo; para banco existente,
recupere a configuração e chaves originais. Não reutilize as chaves de produção
num ambiente dev independente.

```sh
node scripts/init-env.mjs
node scripts/prepare-k8s-env.mjs dev
node scripts/prepare-k8s-env.mjs dev --check
```

O primeiro comando cria `.env`; o segundo lê esse arquivo e cria
`.local/connectme-dev.env`. O gerador recusa sobrescrever configurações existentes.

O operador confere o namespace e o cria somente se não existir:

```sh
kubectl --kubeconfig "$HOME/.kube/config-staging" get namespace connectme-dev
# Somente se a consulta retornar NotFound:
kubectl --kubeconfig "$HOME/.kube/config-staging" create namespace connectme-dev
```

Depois, em instalação nova sem Secret existente:

```sh
kubectl --kubeconfig "$HOME/.kube/config-staging" -n connectme-dev create secret generic connectme-runtime --from-env-file=.local/connectme-dev.env
```

Não use `connectme-pro` para esse arquivo e não insira espaços no caminho.
Sincronize os paths independentes `base` e `overlays/dev` no Fleet, ambos no
namespace `connectme-dev`. O domínio dev é `connectme.dev.cialinux.com`.
Login inicial em banco vazio: `admin/admin`, com troca de senha obrigatória.

## Segredos no deploy

O arquivo `.local/connectme-pro.env` é uma entrada privada de preparação, não
uma dependência permanente da aplicação. Em execução, Kubernetes injeta o
Secret `connectme-runtime`; ele é reutilizado nos próximos deploys.
Não é necessário recriá-lo em cada release.

É possível automatizar o provisionamento usando um gerenciador de segredos
integrado ao cluster ou uma etapa de CI autorizada que receba os valores de
um cofre. Isso exige configurar o provedor e suas credenciais. Não incluímos
chaves reais em `base/`, overlays ou no workflow público, nem criamos um Job
que gere novas chaves ao perder o Secret: isso inutilizaria dados cifrados
de um banco restaurado. A publicação da imagem não recebe segredos de runtime.

O exemplo `replace-with-a-long-random-value` não é uma senha segura de produção.
Sua substituição em banco existente exige alterar a senha do PostgreSQL e o
Secret de forma coordenada; mudar somente o arquivo não muda o banco.
Chaves expostas devem ser rotacionadas com migração dos dados cifrados e backup,
nunca simplesmente substituídas. `CONNECTME_ENV=development` também deve ser
revisto para produção. Os valores explícitos de ambiente no Deployment, como
HTTP e opções de destino/RDP, prevalecem sobre `envFrom` do Secret.

Configuração adotada neste projeto, conforme a instalação do operador:

| Ambiente | Paths no GitRepo | Namespace de destino |
|---|---|---|
| Produção | base e overlays/pro | connectme-pro |
| Desenvolvimento | base e overlays/dev | connectme-dev |

Configure o namespace de destino também para o bundle base. Use GitRepos
separados para dev/pro. Não misture os dois ambientes num mesmo namespace.

base contém Deployment, StatefulSet, Services.
Cada overlay renderiza somente o Ingress do ambiente. O namespace deve existir
antes da instalação (criado pelo Rancher ou pelo operador); `namespace.yaml`
fica disponível apenas para preparação manual, fora do kustomization. Assim,
o overlay não tenta assumir a propriedade Helm de um namespace do Rancher.
Os overlays não
importam a base e não contêm patches ou transformações de imagens de recursos
pertencentes ao outro bundle. Cada objeto possui um único proprietário Fleet.

Não configure a mesma release Helm para os dois bundles. O releaseName fixo e
a seleção kustomize.dir da raiz foram retirados dos exemplos de opções Fleet.
Preserve os recursos/dados existentes ao alterar opções de gerenciamento.

Imagem, recursos e agendamento dos workloads são definidos no bundle base;
um overlay independente não consegue aplicar seus patches ao outro bundle.
Os arquivos de scheduling antigos foram removidos por não serem utilizados.

Validação independente, sem deploy:

```sh
kubectl kustomize base
kubectl kustomize overlays/pro
kubectl kustomize overlays/dev
```

Secret connectme-runtime precisa existir no namespace escolhido. O Ingress
aceita todas as origens; TLS e login continuam obrigatórios. Não há NetworkPolicy
nos manifests padrão. Recursos antigos retidos devem ser tratados pelo operador.

## Instalação nova: preparar antes da sincronização

Uma instalação vazia não contém as credenciais do banco nem as chaves da aplicação.
Elas não são publicadas no Git nem geradas novamente sobre um banco existente.
Em instalação realmente nova, gere primeiro o `.env` com
`node scripts/init-env.mjs` (não sobrescreve arquivos existentes).
Na raiz do projeto, prepare o arquivo a partir do `.env` privado:

```sh
node scripts/prepare-k8s-env.mjs pro
```

Se já existir, não o sobrescreva: valide o arquivo preparado:

```sh
node scripts/prepare-k8s-env.mjs pro --check
```

Com `connectme-pro` já criado, o operador executa **uma vez**:

```sh
kubectl --kubeconfig "$HOME/.kube/config-staging" -n connectme-pro create secret generic connectme-runtime --from-env-file=.local/connectme-pro.env
```

Esse comando não sobrescreve um Secret existente. Não use o `.env` bruto:
a preparação alinha POSTGRES_PASSWORD com a senha da URL da aplicação.
Em restaurações, use as chaves e senha originais do banco; alterar o Secret
não altera a senha armazenada num PostgreSQL já inicializado.
Guarde backup seguro do Secret junto com o backup do banco.

Depois sincronize **ambos** os paths `base` e `overlays/pro` no Fleet,
com namespace `connectme-pro` e releases distintas. Não adicione `../../base`.
Só o bundle base não publica o site: o Ingress pertence ao overlay.

## Verificação após sincronizar

```sh
kubectl --kubeconfig "$HOME/.kube/config-staging" -n connectme-pro rollout status statefulset/postgres --timeout=180s
kubectl --kubeconfig "$HOME/.kube/config-staging" -n connectme-pro rollout status deployment/connectme --timeout=300s
kubectl --kubeconfig "$HOME/.kube/config-staging" -n connectme-pro get pods,pvc,svc,ingress,certificate
kubectl --kubeconfig "$HOME/.kube/config-staging" -n connectme-pro get events --sort-by=.lastTimestamp
```

`CreateContainerConfigError` com `secret not found` bloqueia o banco e,
consequentemente, o init container `wait-postgres`. Não exige rebuild de imagem.
Se não houver Ingress, examine o bundle **overlay** no Rancher/Fleet. A ausência
do recurso não comprova a causa do erro do Fleet: consulte o status desse bundle
no cluster de gerenciamento, pois o kubeconfig downstream não expõe seus GitRepos.
