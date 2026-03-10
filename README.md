# newt-sidecar

Moved to https://github.com/home-operations/newt-sidecar



A Kubernetes sidecar that watches HTTPRoute resources and dynamically generates a [Pangolin](https://github.com/fosrl/pangolin) blueprint YAML for the [newt](https://github.com/fosrl/newt) tunnel daemon.

## How it works

The sidecar runs alongside newt in the same pod, sharing a volume. It watches HTTPRoutes referencing a configured gateway, and writes `/etc/newt/blueprint.yaml` whenever routes change. newt detects the file change and updates the tunnel accordingly.

```
┌─────────────────────────────────────┐
│  Newt Pod                           │
│  ┌──────────────┐  ┌─────────────┐  │
│  │ newt-sidecar │  │    newt     │  │
│  │  (watches    │  │  (reads     │  │
│  │  HTTPRoutes) │  │  blueprint) │  │
│  └──────┬───────┘  └──────┬──────┘  │
│         │   emptyDir vol  │         │
│         └──► blueprint ◄──┘         │
│              .yaml                  │
└─────────────────────────────────────┘
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--gateway-name` | `""` | Gateway name to filter HTTPRoutes (required) |
| `--gateway-namespace` | `""` | Gateway namespace (empty = any) |
| `--namespace` | `""` | Watch namespace (empty = all) |
| `--output` | `/etc/newt/blueprint.yaml` | Output blueprint file path |
| `--site-id` | `""` | Pangolin site nice ID (required) |
| `--target-hostname` | `""` | Backend gateway hostname (required) |
| `--target-port` | `443` | Backend gateway port |
| `--target-method` | `https` | Backend method (`http`/`https`/`h2c`) |
| `--deny-countries` | `""` | Comma-separated country codes to deny |
| `--ssl` | `true` | Enable SSL on resources |
| `--annotation-prefix` | `newt-sidecar` | Annotation prefix for per-resource overrides |

## Annotations

Add these to an HTTPRoute to override per-resource behaviour:

| Annotation | Description |
|------------|-------------|
| `newt-sidecar/enabled: "false"` | Skip this HTTPRoute |
| `newt-sidecar/name: "Custom Name"` | Override the resource display name |
| `newt-sidecar/ssl: "false"` | Disable SSL for this resource |


## Kubernetes deployment

Deploy using the [bjw-s app-template](https://bjw-s-labs.github.io/helm-charts/docs/app-template/) chart. The sidecar runs as a native Kubernetes sidecar (`initContainer` with `restartPolicy: Always`, requires K8s 1.29+). An `emptyDir` volume is shared between the sidecar and newt at `/etc/newt`.

### OCIRepository

```yaml
apiVersion: source.toolkit.fluxcd.io/v1beta2
kind: OCIRepository
metadata:
  name: newt
spec:
  interval: 1h
  url: oci://ghcr.io/bjw-s-labs/helm/app-template
  ref:
    tag: 4.6.2
```

### HelmRelease

```yaml
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: newt
spec:
  chartRef:
    kind: OCIRepository
    name: newt
  interval: 1h
  values:
    defaultPodOptions:
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        runAsGroup: 1000
    controllers:
      newt:
        replicas: 2
        initContainers:
          newt-sidecar:
            image:
              repository: ghcr.io/home-operations/newt-sidecar
              tag: latest
            args:
              - --gateway-name=kgateway-external
              - --site-id=<pangolin-site-id>
              - --target-hostname=kgateway-external.network.svc.cluster.local
              - --deny-countries=RU,CN,KP,IR,BY,IL
            restartPolicy: Always
            resources:
              limits:
                memory: 64Mi
        containers:
          app:
            image:
              repository: fosrl/newt
              tag: 1.10.1
            env:
              PANGOLIN_ENDPOINT:
                valueFrom:
                  secretKeyRef:
                    name: newt-secret
                    key: NEWT_SERVER_URL
              NEWT_ID:
                valueFrom:
                  secretKeyRef:
                    name: newt-secret
                    key: NEWT_ID
              NEWT_SECRET:
                valueFrom:
                  secretKeyRef:
                    name: newt-secret
                    key: NEWT_SECRET
              BLUEPRINT_FILE: /etc/newt/blueprint.yaml
            securityContext:
              allowPrivilegeEscalation: false
              readOnlyRootFilesystem: true
              capabilities: {drop: ["ALL"]}
            resources:
              requests:
                cpu: 10m
              limits:
                memory: 256Mi
    rbac:
      roles:
        newt:
          type: ClusterRole
          rules:
            - apiGroups:
                - gateway.networking.k8s.io
              resources:
                - httproutes
              verbs:
                - get
                - watch
                - list
      bindings:
        newt:
          type: ClusterRoleBinding
          roleRef:
            identifier: newt
          subjects:
            - identifier: newt
    serviceAccount:
      newt: {}
    persistence:
      blueprint:
        type: emptyDir
        globalMounts:
          - path: /etc/newt
```
