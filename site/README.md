# ConnectMe — Kubernetes

Somente manifests: `base/`, `overlays/dev/` e `overlays/pro/`.
Código da aplicação, Dockerfile, Compose e CI da imagem ficam na raiz do repositório.

Execute os comandos a partir da raiz:

```sh
kubectl kustomize site/overlays/dev
kubectl kustomize site/overlays/pro
```

Consulte [deploy, segredos e migração](../docs/KUBERNETES.md).
A aplicação não é implantada automaticamente pelo workflow da imagem.
