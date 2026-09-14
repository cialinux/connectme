# Build e publicação da imagem

Workflow: `.github/workflows/docker-build.yml`, inspirado em images-saft-validator.

Configure no GitHub os secrets `DOCKER_USERNAME` e `DOCKER_PASSWORD` (token Docker Hub
com permissão de publicação). Com `DOCKER_USERNAME=virgiliofilhos`, o destino é
`virgiliofilhos/connectme-server`, igual ao definido nos manifests.

Push em main ou execução manual na main executa testes Go, vet e build do target
`server`. Publica a mesma imagem linux/amd64 com versão `1.0.<run_number>`,
`sha-<commit>`, `latest` e `dev`. Cache de camadas BuildKit/GitHub reduz trabalho
repetido. Guacamole/guacd continuam sendo imagens oficiais, sem build próprio.

A numeração é incremental por execução deste workflow (1.0.1, 1.0.2, ...), não
por tag Git. Falhas podem deixar intervalos. Reexecutar uma execução mantém sua
versão e pode reconstruir a imagem: para pinagem estrita utilize o digest exibido
no resumo. Não há criação de tags Git, commits, Git push ou acesso ao Kubernetes.
Publicações são serializadas; uma execução de commit antigo é recusada quando
a main já avançou. O workflow só é ativo depois que o operador enviar o arquivo
ao GitHub; não foi disparado ou publicado durante a preparação local.

Dev usa o alias dev. Para produção, altere `site/overlays/pro/kustomization.yaml`
para uma versão testada (ou digest). O valor inicial latest existe no registry
após o primeiro build, mas é mutável; fixe a versão antes do rollout de produção.
Não há deploy automático. Se o namespace Docker
Hub for diferente, ajuste `images.newName` nos dois overlays.

Código e build ficam na raiz; manifests em `site/` não entram no contexto Docker.
Veja [Kubernetes](KUBERNETES.md) para a implantação manual.
