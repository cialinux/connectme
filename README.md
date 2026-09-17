# ConnectMe by cialinux

A self-hosted web console for managing locations, networks, hosts, credentials,
and SSH/RDP connections. Guacamole and guacd use official images; the ConnectMe
image provides the dashboard and API. VNC, remote agents, and connectivity without
an existing route or VPN are not yet available. This version does not provide
high availability.

## Getting started

Clone the repository and choose **Docker** or **Kubernetes**:

```sh
git clone https://github.com/cialinux/connectme.git
cd connectme
```

The setup scripts require Node.js. The following steps are for a **new installation**.

### Docker

With Docker and Docker Compose installed:

```sh
cd docker
node ../scripts/init-env.mjs .env
docker compose --env-file .env up -d --pull always --wait
docker compose ps
```

Open `http://YOUR-SERVER-IP:8080`. Sign in with **admin / admin** and change
the password when prompted. Use HTTPS through a reverse proxy for external access.

### Kubernetes

Run these commands from the repository root. Select your cluster using your
usual kubeconfig or context before continuing.

Before deploying, adapt the manifests to your cluster:

- Edit `overlays/dev/ingress.yaml` or `overlays/pro/ingress.yaml`: set your
  hostname, Ingress class, and certificate issuer or existing TLS Secret.
  If using your own certificate, create its TLS Secret in the target namespace
  and remove the cert-manager issuer annotation.
- If you do not need Ingress, remove or comment out `ingress.yaml` in that
  overlay's `kustomization.yaml`. If no resources remain, use `resources: []`.
  You will need another way to expose the application.
- Set the appropriate `storageClassName` in `base/postgres.yaml`.

For **development**:

```sh
node scripts/prepare-k8s-env.mjs dev
kubectl apply -f .local/kubernetes/dev/bootstrap.yaml
```

For **production**:

```sh
node scripts/prepare-k8s-env.mjs pro
kubectl apply -f .local/kubernetes/pro/bootstrap.yaml
```

The script creates the private configuration directory and manifest automatically.
The single bootstrap apply creates the Namespace and Secret; it does not deploy
the application.

In Rancher Fleet, deploy `base` and the selected overlay as **two independent
bundles**, both targeting the same namespace:

| Environment | Fleet paths | Namespace |
|---|---|---|
| Development | `base`, `overlays/dev` | `connectme-dev` |
| Production | `base`, `overlays/pro` | `connectme-pro` |

Open your configured hostname. A new, empty database starts with **admin / admin**
and requires a password change. See the [Fleet setup guide](docs/FLEET.md)
for details.

## Important notes

- Back up your private configuration and database together. Do not publish
  `.env`, `.local/`, Secrets, or database backups. For an existing database,
  reuse its original keys rather than generating new ones.
- SSH identities are stored in the database on first use; changes require
  confirmation. RDP certificate verification is disabled in the default deployment;
  set `CONNECTME_RDP_IGNORE_CERT=false` to require trusted certificates.
- Updates interrupt remote sessions. Do not use `docker compose down --volumes`
  unless you intend to delete the database.

## Documentation

- [Docker](docker/README.md)
- [Kubernetes and Fleet](docs/FLEET.md)
- [Operations](docs/OPERATIONS.md)
- [SSH host identity](docs/SSH_TRUST.md)
- [Clipboard and file transfers](docs/CLIPBOARD.md)
- [Image builds and releases](docs/IMAGE_CI.md)
