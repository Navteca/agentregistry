# Local Kubernetes Development Environment

This directory contains scripts for running AgentRegistry in a local [Kind](https://kind.sigs.k8s.io/) cluster.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/)
- [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [Helm](https://helm.sh/docs/intro/install/)
- [Make](https://www.gnu.org/software/make/)

## Quick Start

```bash
make setup-kind-cluster
```

This single command sets up the full local environment.

## What It Does

`make setup-kind-cluster` runs the following sub-targets in order:

1. **`make create-kind-cluster`** — creates a Kind cluster named `agentregistry` with a local container registry on `localhost:5001` and MetalLB for LoadBalancer support
2. **`make install-agentregistry`** — builds server images, pushes them to the local registry, and Helm installs AgentRegistry with a bundled database instance

You can also run any sub-target individually, e.g. `make install-agentregistry` to redeploy after a code change.

## Database Details

PostgreSQL is bundled in the Helm chart and deployed automatically. The default configuration is:

| Setting  | Value                            |
|----------|----------------------------------|
| Host     | `agentregistry-postgresql.agentregistry.svc.cluster.local` (in-cluster) |
| Port     | `5432`                           |
| Database | `agentregistry`                  |
| Username | `agentregistry`                  |
| Password | `agentregistry`                  |

Local setup uses `pgvector` for development of vector dependent capabilities.

### Connecting Directly

Port-forward to access PostgreSQL from your local machine:

```bash
kubectl --context kind-agentregistry port-forward -n agentregistry svc/agentregistry-postgresql 5432:5432
```

Then connect with psql:

```bash
psql -h localhost -U agentregistry -d agentregistry
```

### pgvector Extension

The `pgvector` extension is automatically available via the `pgvector/pgvector` image. The AgentRegistry server enables it on startup.

To verify manually:

```sql
SELECT * FROM pg_extension WHERE extname = 'vector';
```

### Data Persistence

- Data is stored in a `PersistentVolumeClaim` and survives pod restarts
- Data is **lost** when the Kind cluster is deleted (`make delete-kind-cluster`)

## Accessing AgentRegistry

Port-forward to access the API/UI:

```bash
kubectl --context kind-agentregistry port-forward -n agentregistry svc/agentregistry 12121:12121
```

Then open `http://localhost:12121` in your browser.

Alternatively, use the MetalLB LoadBalancer IP:

```bash
kubectl --context kind-agentregistry get svc -n agentregistry agentregistry
```

## Teardown

```bash
make delete-kind-cluster
```

This deletes the Kind cluster (and all data).

## Configuration

The setup script accepts environment variables to override defaults:

| Variable            | Default                           | Description                        |
|---------------------|-----------------------------------|------------------------------------|
| `KIND_CLUSTER_NAME` | `agentregistry`                   | Kind cluster name                  |
| `KIND_NAMESPACE`    | `agentregistry`                   | Kubernetes namespace               |
| `DOCKER_REGISTRY`   | `localhost:5001`                  | Local registry address             |
| `DOCKER_REPO`       | `agentregistry-dev/agentregistry` | Image repository prefix for local image builds |
| `VERSION`           | `git describe --tags --always`    | Image tag to deploy                |
| `JWT_KEY`           | Random 32-byte hex                | JWT private key for AgentRegistry  |

Example with custom values:

```bash
JWT_KEY=mysecretkey VERSION=v0.2.0 make setup-kind-cluster
```

## Troubleshooting

### PostgreSQL pod not ready

Check pod logs:

```bash
kubectl --context kind-agentregistry logs -n agentregistry -l app.kubernetes.io/component=database
```

### Images not found

Ensure Docker is running and the local registry is accessible:

```bash
curl http://localhost:5001/v2/_catalog
```

If the registry is empty, rebuild images:

```bash
make docker-server docker-agentgateway
```

### Helm install fails

Check AgentRegistry pod logs:

```bash
kubectl --context kind-agentregistry logs -n agentregistry -l app.kubernetes.io/name=agentregistry
```

### Cluster already exists

The setup script is idempotent — it skips cluster creation if the cluster already exists.

To start fresh:

```bash
make delete-kind-cluster && make setup-kind-cluster
```

## OIDC / Keycloak local dev setup

`scripts/kind/keycloak/` deploys a local Keycloak instance into the Kind cluster
for testing OIDC login and role-based permissions in the browser. It is **not**
part of `make setup-kind-cluster`/`make run` by default (anonymous auth remains
the default local flow) — opt in when you need to exercise OIDC.

### Quick start

```bash
# 1. The Keycloak port mapping is baked into kind-config.yaml at cluster
#    creation time. If you already have a cluster from before this was added,
#    recreate it once:
make delete-kind-cluster && make create-kind-cluster

# 2. Deploy Keycloak with the realm (roles, users, client) pre-imported
make install-keycloak

# 3. Install AgentRegistry with the OIDC overlay instead of anonymous auth
make install-agentregistry
helm upgrade --install agentregistry charts/agentregistry \
  --kube-context kind-agentregistry \
  --namespace agentregistry \
  --reuse-values \
  -f scripts/kind/values-oidc.yaml

# 4. Open the UI (port-forward as usual, see "Accessing AgentRegistry" above)
kubectl --context kind-agentregistry port-forward -n agentregistry svc/agentregistry 12121:12121
```

Then open `http://localhost:12121` — you'll be redirected to Keycloak
(`http://keycloak.agentregistry.local:8080`, reachable directly via the kind port mapping, no
`/etc/hosts` edit or extra `port-forward` needed) to log in.

### Test users

All four users are pre-created by `scripts/kind/keycloak/realm-export.json`
with password `password`:

| Username       | Realm role         | Purpose                                  |
|----------------|---------------------|-------------------------------------------|
| `alice-user`   | `registry-user`     | Baseline authenticated user               |
| `bob-curator`  | `registry-curator`  | Curator-level access                      |
| `carol-admin`  | `registry-admin`    | Admin-level access                        |
| `dave-none`    | *(none)*            | Tests the default-permission-bundle fallback |

Realm roles are emitted at the standard Keycloak claim path `realm_access.roles`.

### Resetting Keycloak

Keycloak runs with no `PersistentVolumeClaim` — its realm state lives only in
the pod's container filesystem and is re-imported from the mounted
`realm-export.json` on every start. A full reset is just:

```bash
make keycloak-reset
```

To remove it entirely:

```bash
make delete-keycloak
```

### Why the ID token needs an explicit role mapper

Keycloak's built-in `roles` client scope only adds `realm_access.roles` to the
**access token** by default, not the ID token. Since the app validates the ID
token (`keycloak.idToken`, exchanged via `POST /v0/auth/oidc`), the client in
`realm-export.json` defines its own `realm-roles-id-token` protocol mapper
with `id.token.claim: "true"` to guarantee the claim actually lands where the
backend looks for it.

### Why `keycloakURL` and `oidcIssuer` point at different addresses

This is intentional, not a mistake. Keycloak's `hostname` + `hostname-strict`
settings ([docs](https://www.keycloak.org/server/hostname)) fix the issuer
advertised in tokens to one canonical address, regardless of which network
path was actually used to reach the server — this is the same mechanism that
lets Keycloak sit behind a reverse proxy. Here:

- `oidcIssuer` (backend) = the in-cluster Service DNS name
  (`http://keycloak.keycloak.svc.cluster.local:8080/realms/agentregistry`) —
  reachable natively by the AgentRegistry pod.
- `keycloakURL` (browser) = `http://localhost:8888` — reachable natively by
  your browser via the kind `extraPortMapping` in `kind-config.yaml`.
- Keycloak's `KC_HOSTNAME` is fixed to the in-cluster DNS name, so tokens
  always carry that as `iss` no matter which of the two addresses was used to
  request them.

This avoids needing a live `kubectl port-forward` process or an `/etc/hosts`
entry just to test locally.

## Scripts

| File                          | Purpose                                        |
|-------------------------------|-------------------------------------------------|
| `setup-kind.sh`               | Creates Kind cluster with local registry        |
| `setup-metallb.sh`            | Installs and configures MetalLB                 |
| `kind-config.yaml`            | Kind cluster configuration                      |
| `keycloak/keycloak.yaml`      | Keycloak Namespace/Deployment/Service manifest  |
| `keycloak/realm-export.json`  | Pre-imported realm: roles, users, OIDC client   |
