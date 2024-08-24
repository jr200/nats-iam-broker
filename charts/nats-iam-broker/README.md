# nats-iam-broker Helm Chart

A helm chart for deploying the nats-iam-broker application on kubernetes.

## Overview

This Helm chart installs the NATS IAM Broker, a native NATS decentralised auth-callout micro-service that manages identity and access for NATS deployments. It integrates with Vault for secret management and supports features like RBAC, JWT validation, and encryption.

## Installation

To install the chart with the release name `my-nats-iam-broker`:

```bash
helm repo add jr200 https://jr200.github.io/helm-charts/
helm install my-nats-iam-broker jr200/nats-iam-broker
```

If the `vault` and `vault-actions` component are enabled in the chart, the deployed vault-actions `Job` will fail and print some instructions resembling:

```bash
vault write sys/policy/autogen:tmp-bootstrap:ad81f81237ad4ebd \
  policy=- <<EOF
    path "sys/policy/my-nats-iam-broker-policy" {
        capabilities = ["create", "update"]
    }

    path "auth/kubernetes/role/my-nats-iam-broker-role" {
        capabilities = ["create", "update"]
    }

    path "sys/policy/autogen:tmp-bootstrap:ad81f81237ad4ebd" {
        capabilities = ["delete"]
    }
EOF

vault token create                                                           \
  -policy='autogen:tmp-bootstrap:ad81f81237ad4ebd'                           \
  -policy='default'                                                          \
  -metadata=cleanup_path='sys/policy/autogen:tmp-bootstrap:ad81f81237ad4ebd' \
  -display-name='nats-bootstrap'                                             \
  -explicit-max-ttl='5m'                                                     \
  -field='wrapping_token'                                                    \
  -format='table'                                                            \
  -renewable='False'                                                         \
  -wrap-ttl='60s'
```

Running the above vault-actions script will output a wrapped vault token with the policy as defined above. This is the `vault-actions.bootstrapToken` it can be passed to the chart using the following command:

To specify the vault bootstrap token, set it in the `values.yaml` or pass it in as:

```bash
helm upgrade my-nats-iam-broker jr200/nats-iam-broker \
    --install \
    --set vault-actions.bootstrapToken=<<WRAPPED_VAULT_TOKEN>>> \
	-f values.yaml
```

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
| `logging.level`                                           | Set logging level (error, warn, info, debug, trace)                                       | `info`                            |
| `config.params.left_delim`                                | The left delimiter for template parsing                                                   | `<<<`                             |
| `config.params.right_delim`                               | The right delimiter for template parsing                                                  | `>>>`                             |
| `config.app.name`                                         | The name of the NATS microservice                                                         | `my-nats-iam-broker`              |
| `config.app.description`                                  | A description for the NATS microservice                                                   | `my-nats-iam-broker auth-callout` |
| `config.app.nats.url`                                     | URL of the NATS server                                                                    | `nats.nats.svc:4222`              |
| `config.app.nats.jwtExpiry`                               | Expiration duration of granted NATS user JWTs                                             | `1h15m`                           |
| `config.app.natsMicroService.userCredsFile`               | Path to the NATS user credentials file - this is the user the nats-iam-broker runs as     | `""`                              |
| `config.app.natsMicroService.account.name`                | The name of the decentralized NATS account                                                | `""`                              |
| `config.app.natsMicroService.account.signingNkey.type`    | The type of key storage (raw, env, file)                                                  | `file`                            |
| `config.app.natsMicroService.account.signingNkey.value`   | Path to the signing key                                                                   | `""`                              |
| `config.app.natsMicroService.encryption.enabled`          | Enable or disable encryption between the NATS server and the nats-iam-broker microservice | `true`                            |
| `config.app.natsMicroService.encryption.xkeySecret.type`  | The type of key storage (raw, env, file)                                                  | `file`                            |
| `config.app.natsMicroService.encryption.xkeySecret.value` | Private encryption xkey used to secure communication                                      | `""`                              |
| `idp.client_id`                                           | The client ID for the identity provider                                                   | `public`                          |
| `idp.issuer_url`                                          | URLs of the identity provider issuer                                                      | `https://oidctest.wsweet.org/`    |
| `idp.validation.claims`                                   | Claims required to be present on the incoming JWT validation                              | `aud, iat, exp, sub`              |
| `idp.validation.aud`                                      | Expected values for the audience claim of the JWT                                         | `public`                          |
| `idp.validation.exp.min`                                  | Minimum acceptable expiration duration for JWT                                            | `1m0s`                            |
| `idp.validation.exp.max`                                  | Maximum acceptable expiration duration for JWT                                            | `2h`                              |
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
| `vault.enabled`                                           | Enable or disable Vault-Agent Sidecar Injector                                            | `false`                           |
| `vault.url`                                               | The URL of the Vault server                                                               | `http://vault.vault.svc`          |
| `vault.brokerRole`                                        | The vault role to assume for the vault-agent init container                               | `my-nats-iam-broker-role`         |
| `vault.authMount`                                         | The Vault Kubernetes authentication mount path                                            | `auth/kubernetes`                 |
| `vault.xkeySecret`                                        | Path to the xkey secret in Vault                                                          | `""`                              |
| `vault.mintSigningNKey`                                   | Path to the mint signing key in Vault                                                     | `""`                              |
| `vault.mintUserCreds`                                     | Path to the mint user credentials in Vault                                                | `"`                               |
| `vault.rbacCredentials`                                   | List of additional RBAC account credentials to fetch from Vault                           | Example: see `values.yaml`        |

### vault-actions subchart values.yaml (inline)

| Parameter                                                | Description                                                              | Default                               |
| -------------------------------------------------------- | ------------------------------------------------------------------------ | ------------------------------------- |
| `vault-actions.enabled`                                  | Enable or disable Vault actions                                          | `false`                               |
| `vault-actions.devDebug`                                 | Enable or disable debug mode for Vault actions                           | `false`                               |
| `vault-actions.hookConfiguration.hook`                   | The hook phase for Vault actions                                         | `pre-install`                         |
| `vault-actions.hookConfiguration.hookWeight`             | The weight for hook ordering                                             | `-5`                                  |
| `vault-actions.hookConfiguration.hookDeletePolicy`       | The delete policy for the hook                                           | `before-hook-creation,hook-succeeded` |
| `vault-actions.bootstrapToken`                           | The bootstrap token for Vault actions                                    | `~`                                   |
| `vault-actions.secretName`                               | The name of the secret for vault-actions bootstrap                       | `va-bootstrap-secret`                 |
| `vault-actions.policyName`                               | The name of the created policy, used by the vault-agent sidecar injector | `my-nats-iam-broker-policy`           |
| `vault-actions.config.bootstrap.cleanup_after_install`   | Clean up the bootstrap policy after installation                         | `true`                                |
| `vault-actions.config.bootstrap.params.display-name`     | Display name for the bootstrap                                           | `nats-bootstrap`                      |
| `vault-actions.config.bootstrap.params.explicit-max-ttl` | Maximum TTL for the bootstrap token                                      | `5m`                                  |
| `vault-actions.config.bootstrap.params.renewable`        | Whether the bootstrap token is renewable                                 | `false`                               |
| `vault-actions.config.bootstrap.params.wrap-ttl`         | The TTL for the one-time use wrapping tokens                             | `60s`                                 |
| `vault-actions.config.bootstrap.params.format`           | The format for displaying secrets                                        | `table`                               |
| `vault-actions.config.bootstrap.params.field`            | The field to use for wrapping tokens                                     | `wrapping_token`                      |
| `vault-actions.config.additionalActions`                 | Additional actions to perform during vault-actions bootstrap             | `[]`                                  |

For a complete list of configuration options, see the `values.yaml` file.
