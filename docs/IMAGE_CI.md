# Build e publicação da imagem

Workflow: `.github/workflows/docker-build.yml`, inspirado em images-saft-validator.

Configure no GitHub os secrets `DOCKER_USERNAME` e `DOCKER_PASSWORD` (token Docker Hub
com permissão de publicação). Com `DOCKER_USERNAME=virgiliofilhos`, o destino é
`virgiliofilhos/connectme-server`, igual ao definido nos manifests.

Push em main com alteração em código, migrations, testes, dependências Go,
Dockerfile, `.dockerignore` ou neste workflow executa testes Go, vet e build do target
`server`. Publica a mesma imagem linux/amd64 com versão `1.0.<run_number>`,
`sha-<commit>`, `latest` e `dev`. Cache de camadas BuildKit/GitHub reduz trabalho
repetido. Guacamole/guacd continuam sendo imagens oficiais, sem build próprio.

Alterações apenas em `README.md`, `docs/`, `base/`, `overlays/`, `docker/`,
`compose.yaml` ou scripts locais de instalação não disparam publicação.
Um push misto que também altere um arquivo incluído em `paths` dispara a build.
Execução manual (`workflow_dispatch`) na main continua disponível, inclusive
para reconstruir a imagem sem alterações nesses arquivos.
O filtro não muda a reconciliação do Fleet nem a numeração das releases.
Como este ajuste altera o próprio workflow, seu primeiro push dispara uma build.
Veja [filtros de caminhos do GitHub Actions](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax#onpushpull_requestpull_request_targetpathspaths-ignore).

A numeração é incremental por execução deste workflow (1.0.1, 1.0.2, ...), não
por tag Git. Falhas podem deixar intervalos. Reexecutar uma execução mantém sua
versão e pode reconstruir a imagem: para pinagem estrita utilize o digest exibido
no resumo. Não há criação de tags Git, commits, Git push ou acesso ao Kubernetes.
Publicações são serializadas; uma execução de commit antigo é recusada quando
a main já avançou. O workflow só é ativo depois que o operador enviar o arquivo
ao GitHub; não foi disparado ou publicado durante a preparação local.

A imagem dos workloads é definida em `base/deployment.yaml`; os overlays Fleet
independentes não transformam recursos da base. Prefira uma versão testada ou
digest no deploy. Se o namespace Docker Hub for diferente, ajuste a referência
na base e em `docker/compose.yaml`. Alterar esses manifests não publica imagem.

Código, build e manifests ficam na raiz; `base/` e `overlays/` não entram no contexto Docker.
Veja [Kubernetes](KUBERNETES.md) para a implantação manual.
