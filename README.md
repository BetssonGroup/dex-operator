# Dex-Operator

## Background


## Installing

The operator currently requres `Certmanager` and Dex installed. Install the operator by running the `dex-operator` helm 3 chart in `contrib/charts/dex-operator`.

Install DEX using the official helm chart and set at least the following:

```yaml
certs:
  web:
    create: true
    altNames:
      - dex
  grpc:
    create: true
    altNames:
      - dex
    secret:
      serverTlsName: dex-grpc-server-tls
      clientTlsName: dex-grpc-client-tls
      caName: dex-grpc-ca
    server:
      secretName: dex-grpc-server-cert
```

## Using dex-operator

The dex operator is controlled using CRD's. To add a new OIDC client to current running DEX server, deploy the following yaml:

```yaml
apiVersion: dex.betssongroup.com/v1
kind: Client
metadata:
  name: argocd # Must be unique inside DEX
spec:
  name: ArgoCD
  secret: 33559e7361087368bdac8e93f889c963d2c29399
  redirectURIs:
    - https://argocd/auth/callback # Where the oidc client should redirect back
 ```

The complete schema is:

```yaml
apiVersion: dex.betssongroup.com/v1
kind: Client
metadata:
  name: test-client
spec:
  Name: test client
  Secret: faa85ae56aae06999f8681ba2e9b2ff1bc6608b8
  Public: true
  redirectURIs:
    - https://localhost:1234/auth
  trustedPeers:
    - web
  logoURL: https://foo/img.png
```

## Developing

Built using `kubebuilder`


### Adding Controllers

This project is built using `kubebuilder` To add a new controller run:

`kubebuilder create api --group dex.betssongroup.com --version v1 --kind MyKind`

## Building

`make IMG=my-registry.tld/org/dex-operator docker-build docker-push deploy`

