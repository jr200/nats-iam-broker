# nats-iam-broker\*

_\*for the Decentralised Auth Callout flow_

A configurable NATS micro-service that exchanges Identity Provider (IdP) tokens<sup>1</sup> _for_ NATS user authorization JWTs. This component helps implement the decentralised auth callout flow:

1. A prospective NATS user authenticates with their IdP, and passes the obtained token to NATS.
2. This micro-service (re-)validates the IdP token.
3. This micro-service mints a new NATS user access token with specific authorizations.

<sup>1</sup>https://www.iana.org/assignments/jwt/jwt.xhtml

## What does it do?

The assumption is that implementations of auth callout microservices are _almost_ identical, but for the process of determining which authorizations should be assigned to the minted NATS user access tokens.

This component implements the boilerplate code and using YAML configuration and golang templating, allows construction of JWTs with claims of the form:

```
claims(natsJwt) = f(claims(idpJwt))
```

where:

- `natsJwt` is the JWT sent to the NATS server when establishing a new connection.
- `f` is a function that extracts and transforms claims from the IdP's Jwt.
- `idpJwt` is the IdP JWT supplied from the user.

## Examples

The mock OIDC/OAuth2 service https://oidctest.wsweet.org/ is used to obtain an authenticated JWT `id_token`.

The example scripts in the `scripts/` and `examples/` folders demonstrate how to configure, deploy and test this micro-service. These are structred as:

- `scripts`
  - `*-toolkit.sh`, helper bash functions.
  - `/templates/*`, templates to use with vault/openbao and [vault-plugin-secret-nats](https://github.com/edgefarm/vault-plugin-secrets-nats).
- `examples`
  - `/<example_name>`
    - `/<name>_initial_setup.sh`, one-time script to setup decentralised accounts and users
    - `/<name>_start_service.sh`, configures and starts the auth callout micro-service
    - `/<name>_simulate_login.sh`, contacts IdP, authenticates, and uses the obtained token to establish a client connection with NATS
    - `/run.sh`, run all steps in the example

Assuming docker is installed, the examples can be run using: `make example-<example_name>`.

### Example 1: RGB.org

The `RGB.org` organisation has three departments that share a single NATS deployment. A department consists of several teams, with each team developing a set of applications.

<img src="./docs/example_rgb_org_oac.svg">

The blue department has this setup.

- there are three teams: `App Team 1`, `App Team 2`, `App Team 3`

  - they have each developed apps that require access to NATS.
  - users of `App Team`_`i`_'s apps have NATS credentials minted by NATS account `APPi`

- the department has configured and deployed a shared instance of the nats-iam-broker microservice.
  - the microservice connects to NATS using _user_ `minter` of NATS _account_ `MINT_11`.
  - minting accounts have an additional _user_ `nobody`, that has no permissions.

Bob is a member of `App Team 3`, and wishes to use their in-house app `demo-app`. This section describes the authentication and authorization flow.

<img src="./docs/example_rgb_org_flow.svg">

1. Bob launches `demo-app`.
   - `demo-app` directs him to `OIDC Provider2` for authentication, which Bob completes.
   - `demo-app` receives back a signed JWT token `jwt.provider2.bob`.
2. `demo-app` obtains credentials<sup>1</sup> for `MINT_11(nobody)` and packages this into a NATS `authorization_request` (perhaps by calling `nats.connect()`)<sup>2</sup>.

3. NATS creates a `connection_id` for the Bob's instance of `demo-app`, and directs the `MINT_11(nobody)` connection to the blue department's nats-iam-broker microservice. This is because the microservice connects to NATS using `MINT_11(minter)`, and the two have a common NATS account. The connection between NATS and nats-iam-broker is private<sup>3</sup>.

4. nats-iam-broker microservice receives the request.

   - it unpacks `jwt.provider2.bob` and validates the token against `OIDC Provider2`.
   - it performs additional validations, like checking JWT expiry/clock skew, etc.
   - an unsuccessful validation reports an `Authorization Violation` back to the user.

5. nats-iam-broker microservice mints a new JWT.

   - it inspects Bob's user/profile information in `jwt.provider2.bob`.
   - it determines that the account to issue+sign the minted token is `APP3`.
   - it creates a set of unique authorizations for Bob's `demo-app` usage.
   - it sets a suitable token expiry etc.

6. nats-iam-broker signs the minted JWT, encrypts it for transport and sends it to NATS server.
7. NATS server decrypts<sup>3</sup> and validates the nats-jwt, and binds the authorizations to the client's `connection_id`. Finally, it notifies Bob's instance of `demo-app` of the successful connection.

---

<sup>1</sup> although `nobody` user credentials have no NATS permissions, storing them externally can facilitate key rotation of `signing_key(MINT_11)`.

<sup>2</sup> a (somewhat) arbitrary decision has been made to pass the third-party JWT in the password field, i.e., `nats.Connect(UserCredentials(MINT_11(nobody)), password=jwt.provider2.bob)`

<sup>3</sup> this uses the `XKey` field. It is optional but recommended.

### Example 2: Basic

This example demonstates a simpler setup consisting of:

- one auth callout microservice `MINT`, with standard users `minter` and `nobody`
- one appliction account `APP1`

As in the previous example, authorised NATS users are created and signed on-the-fly, following successful validation of the IdP token.


## Setup

This section describes some (probably common) deployment methods:

- a single golang binary plus a (mergable set of) YAML configuration file(s).
- a docker/podman compose file.
- a helm-chart, see [charts/nats-iam-broker/README.md](https://github.com/jr200/nats-iam-broker/tree/main/charts/nats-iam-broker/README.md).

## Docker/Podman Compose Setup

NATS accounts must be created locally (with access to keys) and then pushed to the target NATS server.
This section is intended as an example/rough-guide and assumes the NATS infrastructure as described in the [nats-infra](https://github.com/jr200/nats-infra/tree/main) repo is up and running.

1. Create the `MYAPP-MINT` auth-callout account.

```bash
export OPERATOR_NAME=local-operator
export ACCOUNT_NAME=MYAPP-MINT
export NATS_CONTAINER=infra-team-nats-1
export NSC_CONTAINER=infra-team-nsc-admin-1
export OUTPUT_DIR=./nats-secrets

echo Creating account $ACCOUNT_NAME
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/jr200/nats-infra/main/scripts/nats-create-account.sh)"

echo Enabling auth-callout for $ACCOUNT_NAME
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/jr200/nats-infra/main/scripts/nats-enable-auth-callout.sh)"

echo Fetching signing keys for $ACCOUNT_NAME
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/jr200/nats-infra/main/scripts/nats-fetch-signing-creds.sh)"
```

2. Create the `MYAPP` account

```bash
export OPERATOR_NAME=local-operator
export ACCOUNT_NAME=MYAPP
export NATS_CONTAINER=infra-team-nats-1
export NSC_CONTAINER=infra-team-nsc-admin-1
export OUTPUT_DIR=./nats-secrets

echo Creating account $ACCOUNT_NAME
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/jr200/nats-infra/main/scripts/nats-create-account.sh)"

echo Fetching signing keys for $ACCOUNT_NAME
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/jr200/nats-infra/main/scripts/nats-fetch-signing-creds.sh)"
```

3. Start the nats-iam broker instance

```
podman compose -f compose-iam.yml -p dev-team up -d
```


## Kubernetes Setup

Standard go templating is used to construct roles dynamically. Templating functions are listed in the `filters` module.

The configuration structure is outlined here.

| Key                                      | Type                                                                    | Description                                                                                              |
| ---------------------------------------- | ----------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------- |
| `nats.url`                               | `string`                                                                | NATS server URL                                                                                          |
| `service.name`                           | `string`                                                                | name of deployed micro-service instance                                                                  |
| `service.version`                        | `string`                                                                | version of deployed micro-service instance                                                               |
| `service.description`                    | `string`                                                                | description of deployed micro-service instance                                                           |
| `service.creds_file`                     | `string`                                                                | user credentials used to connect to NATS                                                                 |
| `service.account.name`                   | `string`                                                                | (metadata) _human-readable reminder of account used to sign and encrypt communications with NATS server_ |
| `service.account.signing_nkey`           | `string`                                                                | key used to sign new user-jwt returned to NATS                                                           |
| `service.account.encryption.enabled`     | `bool`                                                                  | toggle for communication-encryption with NATS server                                                     |
| `service.account.encryption.xkey_secret` | `string`                                                                | trusted secret-key for encryption with NATS server                                                       |
| `nats_jwt.exp_max`                       | `duration`                                                              | maximum duration of minted nats user-jwt                                                                 |
| `idp.client_id`                          | `string`                                                                | the client identifier registered with the IdP                                                            |
| `idp.issuer_url`                         | `string`                                                                | the url of the IdP issuer                                                                                |
| `idp.validation.claims`                  | `[]string`                                                              | set of required claims on idp token                                                                      |
| `idp.validation.aud`                     | `[]string`                                                              | set of allowed values for _audience_ claim                                                               |
| `idp.validation.exp.min`                 | `duration`                                                              | minimum time to expiry for idp token from _now_                                                          |
| `idp.validation.exp.max`                 | `duration`                                                              | maximum duration of idp token from _now_                                                                 |
| `rbac.user_accounts`                     | -                                                                       | set of accounts configured to issue and sign nats user-jwts                                              |
| `rbac.user_accounts[i].name`             | `string`                                                                | name of user-jwt signing account                                                                         |
| `rbac.user_accounts[i].public_key`       | `string`                                                                | public key of user-jwt signing account                                                                   |
| `rbac.user_accounts[i].signing_nkey`     | `string`                                                                | signing key of user-jwt signing account in nkey format                                                   |
| `rbac.roles`                             | -                                                                       | set of referenceable nats jwt permission groupings                                                       |
| `rbac.roles[i].name`                     | `string`                                                                | role name                                                                                                |
| `rbac.roles[i].permissions`              | [jwt.Permissions](https://github.com/nats-io/jwt/blob/main/v2/types.go) | nats-io/jwt permissions structure (see link)                                                             |
| `rbac.roles[i].limits`                   | [jwt.Limits](https://github.com/nats-io/jwt/blob/main/v2/types.go)      | nats-io/jwt limits structure (see link)                                                                  |
| `rbac.role_binding[i].user_account`      | `string`                                                                | user account to bind                                                                                     |
| `rbac.role_binding[i].roles`             | `[]string`                                                              | set of roles to bind                                                                                     |
| `rbac.role_binding[i].match.claim`       | `string`                                                                | name of IdP JWT claim to match on                                                                        |
| `rbac.role_binding[i].match.value`       | `string`                                                                | corresponding value of IdP JWT claim to match on                                                         |

# References

1. youtube, synadia auth-callout: https://www.youtube.com/watch?v=VvGxrT-jv64
2. youtube: synaida decentralised auth callout: https://www.youtube.com/watch?v=5pQVjN0ym5w
3. natsbyexample: https://natsbyexample.com/examples/auth/callout-decentralized/cli
4. rethinking connectivity 19: https://github.com/synadia-io/rethink_connectivity/tree/main/19-auth-callout
5. mock oidc service: https://lemonldap-ng.org/documentation/latest/testopenidconnect.html
