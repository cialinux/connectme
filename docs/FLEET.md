# Fleet: dois bundles independentes

Configuração adotada neste projeto, conforme a instalação do operador:

| Ambiente | Paths no GitRepo | Namespace de destino |
|---|---|---|
| Produção | base e overlays/pro | connectme-pro |
| Desenvolvimento | base e overlays/dev | connectme-dev |

Configure o namespace de destino também para o bundle base. Use GitRepos
separados para dev/pro. Não misture os dois ambientes num mesmo namespace.

base contém Deployment, StatefulSet, Services e ConfigMap de confiança SSH.
Cada overlay contém somente Namespace e Ingress do ambiente. Os overlays não
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
