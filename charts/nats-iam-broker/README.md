# nats-iam-broker Helm Chart

A helm chart for deploying the nats-iam-broker application on kubernetes.

## Overview

This Helm chart installs the NATS IAM Broker, a native NATS decentralised auth-callout micro-service that manages identity and access for NATS deployments.

## Installation

To install the chart with the release name `my-iam-broker`:

```bash
helm repo add jr200 https://jr200.github.io/helm-charts/
helm install my-iam-broker jr200/nats-iam-broker
```

### kubernetes secrets

A set of credentials (secrets) must be mounted into the nats-iam-broker pod.
These credentials are:

- signing key for the MINT account
- user-creds for the MINT account's auth-callout user
- (optional) xkey for the auth-callout service to communicate with nats-server
- for each application account:
  - the public key
  - the signing key

A script is provided to help make these secrets.

```
echo Creating kubernetes secrets...
curl  -fsSL https://raw.githubusercontent.com/jr200/nats-infra/refs/heads/main/scripts/k8s-make-nats-secrets.sh | bash -s -- /Users/primary/code/jr200/janeway/janeway-infra/nats-secrets
```

The generated `nats-secrets.yaml` can be uploaded to the nats-iam-broker target namespace.


## Configuration

The following table lists the configurable parameters of the `nats-iam-broker` chart and their default values.

| Parameter                                                 | Description                                                                               | Default                           |
| --------------------------------------------------------- | ----------------------------------------------------------------------------------------- | --------------------------------- |
| `devDebug`                                                | Enable or disable debug mode for development                                              | `false`                           |
| `replicaCount`                                            | Number of replicas for the deployment                                                     | `1`                               |
| `image.repository`                                        | The Docker image repository                                                               | `ghcr.io/jr200/nats-iam-broker`   |
| `image.pullPolicy`                                        | The image pull policy (Always, IfNotPresent, Never)                                       | `Always`                          |
| `image.tag`                                               | Overrides the image tag whose default is the chart appVersion                             | `""`                              |
| `imagePullSecrets`                                        | Docker registry secrets                                                                   | `[]`                              |
| `nameOverride`                                            | Override the default name of the chart                                                    | `""`                              |
| `fullnameOverride`                                        | Override the full name of the chart                                                       | `""`                              |
| `logging.json`                                            | Set to true to enable JSON logging                                                        | `false`                           |
| `logging.level`                                           | Set logging level (error, warn, info, debug, trace)                                       | `trace`                           |
| `env`                                                     | Environment variables for the nats-iam-broker container                                   | See values.yaml                   |
| `rbac.accounts`                                           | Configuration of decentralized accounts that can sign newly minted NATS user JWTs         | Example: see `values.yaml`        |
| `rbac.roles`                                              | Roles (groups of permissions + limits) that can be attached to minted NATS user JWTs      | Example: see `values.yaml`        |
| `rbac.bindings`                                           | Configuration to associate RBAC roles to RBAC accounts                                    | Example: see `values.yaml`        |
| `serviceAccount.create`                                   | Specifies whether a service account should be created                                     | `true`                            |
| `serviceAccount.automount`                                | Automatically mount a ServiceAccount's API credentials                                    | `true`                            |
| `serviceAccount.annotations`                              | Annotations to add to the service account                                                 | `{}`                              |
| `serviceAccount.name`                                     | The name of the service account to use.                                                   | `""`                              |
| `podAnnotations`                                          | Additional annotations to add to the pods                                                 | `{}`                              |
| `podLabels`                                               | Additional labels to add to the pods                                                      | `{}`                              |
| `podSecurityContext`                                      | Define security context for the pod                                                       | `{}`                              |
| `securityContext`                                         | Define security context for the container                                                 | `{}`                              |
| `resources`                                               | Resource requests and limits for the container                                            | `{}`                              |
| `volumes`                                                 | Additional volumes on the output Deployment definition                                    | `[]`                              |
| `volumeMounts`                                            | Additional volumeMounts on the output Deployment definition                               | `[]`                              |
| `nodeSelector`                                            | Node selector for pod scheduling                                                          | `{}`                              |
| `tolerations`                                             | Tolerations for pod scheduling                                                            | `[]`                              |
| `affinity`                                                | Affinity rules for pod scheduling                                                         | `{}`                              |

For a complete list of configuration options, see the `values.yaml` file.
