# Cert-manager-webhook for the Nicru API

### Motivation

Cert-manager automates the management and issuance of TLS certificates in Kubernetes clusters. It ensures that certificates are valid and updates them when necessary.

A certificate authority resource, such as ClusterIssuer, must be declared in the cluster to start the certificate issuance procedure. It is used to generate signed certificates by honoring certificate signing requests.

For some DNS providers, there are no predefined CusterIssuer resources. Fortunately, cert-manager allows you to write your own webhook.

This solver allows you to use cert-manager with the Nicru API. Documentation on the Nicru API is available [here](https://www.nic.ru/help/upload/file/API_DNS-hosting.pdf).

# Usage

## Automated Token Management (v2.0.0+)

**NEW in v2.0.0:** The webhook now automatically manages OAuth tokens for you! You no longer need to manually acquire tokens from NIC.RU.

### Quick Start

1. **Register OAuth Application** at [NIC.RU](https://www.nic.ru/help/oauth-server_3642.html#reg)
   - You'll receive `CLIENT_ID` and `CLIENT_SECRET`

2. **Create Account Secret** with your credentials:
   ```bash
   kubectl create secret generic nicru-account \
     --from-literal=USERNAME='your-nic-username' \
     --from-literal=PASSWORD='your-nic-password' \
     --from-literal=CLIENT_ID='your-oauth-client-id' \
     --from-literal=CLIENT_SECRET='your-oauth-client-secret' \
     -n cert-manager
   ```

3. **Install the webhook** (see installation section below)

4. **Done!** The webhook will automatically:
   - Acquire OAuth tokens from NIC.RU
   - Create and manage the `nicru-tokens` secret
   - Refresh tokens every 3 hours
   - Re-authenticate if refresh fails

For detailed information, see [AUTOMATED_TOKENS.md](docs/features/AUTOMATED_TOKENS.md)

## Manual Token Management (Legacy - v1.0.0)

<details>
<summary>Click to expand legacy manual token setup (not recommended for v2.0.0+)</summary>

### Preparation
First, you must [register](https://www.nic.ru/help/oauth-server_3642.html#reg) the application in your personal Nicru account.
After registering, you will receive the `app_id` and `app_secret` of the OAuth 2.0 protocol, which are needed to get the tokens.

Then you must get 2 tokens: the first token to work with the API, the second token to reissue tokens.
You have to run this command to get them:

```shell
curl --location --request POST 'https://api.nic.ru/oauth/token' \
--header 'Content-Type: application/x-www-form-urlencoded' \
--data-urlencode 'grant_type=password' \
--data-urlencode 'password=<YOUR_PASSWORD_FOR_PERSONAL_ACCOUNT>' \
--data-urlencode 'client_id=<YOUR_APP_ID>' \
--data-urlencode 'client_secret=<YOUR_APP_SECRET>' \
--data-urlencode 'username=<YOUR_LOGIN_FOR_PERSONAL_ACCOUNT>' \
--data-urlencode 'scope=.*' \
--data-urlencode 'offline=999999'
```
You will get an answer
```json
{"refresh_token":"<REFRESH_TOKEN>","expires_in":14400,"access_token":"<ACCESS_TOKEN>","token_type":"Bearer"}
```

</details>

------------------------------
### Install cert-manager (*optional step*)

**ATTENTION!** You should not delete the cert-manager if you are already using it.


Use the following command from the [official documentation](https://cert-manager.io/docs/installation/) to install cert-manager in your Kubernetes cluster:

```shell
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/VERSION/cert-manager.yaml
```
*  where `VERSION` is necessary version (for example, v1.10.1 )

### Install the webhook

**NOTE**: The kubernetes resources used to install the Webhook should be deployed within the same namespace as the cert-manager.


```shell
git clone https://github.com/flant/cert-manager-webhook-nicru.git
```

You must also specify your namespace with the `cert-manager`.

```yaml
certManager:
  namespace: your-cert-manager-namespace
  serviceAccountName: cert-manager
```

### Deploy the webhook

**For v2.0.0+ (Automated Tokens - Recommended):**

After creating the `nicru-account` secret (see Quick Start above), simply install the webhook:

```shell
cd cert-manager-webhook-nicru
helm install -n cert-manager nicru-webhook ./helm
```

The webhook will automatically create and manage the `nicru-tokens` secret for you.

**For v1.0.0 (Manual Tokens - Legacy):**

<details>
<summary>Click to expand legacy manual installation</summary>

Create the `nicru-tokens.yaml` file with the following content:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: nicru-tokens
  namespace: your-cert-manager-namespace
data:
  REFRESH_TOKEN: <REFRESH_TOKEN_BASE64_FORMAT>
  ACCESS_TOKEN: <ACCESS_TOKEN_BASE64_FORMAT>
  APP_ID: <APP_ID>
  APP_SECRET: <APP_SECRET>
type: Opaque
```

Then install the webhook:

```shell
cd cert-manager-webhook-nicru
helm install -n my-namespace-cert-manager nicru-webhook ./helm
```

</details>

### Create a certificate

Create the `certificate.yaml` file with the following contents:

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: changeme
  namespace: changeme
spec:
  secretName: changeme
  issuerRef:
    name: nicru-dns
    kind: ClusterIssuer
  dnsNames:
    -  *.my-domain-test.ru
```

# Known issues

```
Error presenting challenge: the server is currently unable to handle the request (post nicru-dns.acme.nic.ru)
```
This error may indicate that there is a failure to communicate with APIService v1alpha1.acme.nic.ru. In this case, `insecureSkipTLSVerify: true` parameter in apiservice.yaml may help.


# Community

Please feel free to contact us if you have any questions.

You're also welcome to follow [@flant_com](https://twitter.com/flant_com) to stay informed about all our Open Source initiatives.

# License

Apache License 2.0, see [LICENSE](LICENSE).

## Configuration

### Helm Values

The webhook can be configured via Helm values. Here are the key configuration options:

#### APIService TLS Verification

Control whether the APIService should skip TLS certificate verification:

```yaml
apiservice:
  insecureSkipTLSVerify: false
```

**When to use**:
- **`true` **: Use for easier deployment and troubleshooting. Recommended if you encounter errors like "the server is currently unable to handle the request".
- **`false` (default)**: Use for maximum security if your environment properly handles cert-manager's CA certificate injection.

#### Installation Examples

**Standard installation (with default TLS skip)**:
```bash
helm install nicru-webhook ./helm -n cert-manager
```

**Secure installation (with TLS verification enabled)**:
```bash
helm install nicru-webhook ./helm \
  -n cert-manager \
  --set apiservice.insecureSkipTLSVerify=false
```

**Custom values file**:
```yaml
# my-values.yaml
groupName: acme.nic.ru

apiservice:
  insecureSkipTLSVerify: true  # or false for stricter security

certManager:
  namespace: cert-manager
  serviceAccountName: cert-manager

webhook:
  image: ghcr.io/flant/cert-manager-webhook-nicru:v2.0.0
  imagePullPolicy: IfNotPresent

resources:
  requests:
    cpu: 200m
    memory: 256Mi
  limits:
    memory: 256Mi
```

```bash
helm install nicru-webhook ./helm -n cert-manager -f my-values.yaml
```

## Version Compatibility

| Webhook Version | cert-manager | Kubernetes | Go   |
|----------------|--------------|------------|------|
| v2.0.0         | 1.19.1+      | 1.30+      | 1.25 |
| v1.0.0         | 1.14.5+      | 1.30       | 1.22 |

## Development

### Running Tests

The project includes comprehensive unit tests for critical components with 41% overall coverage (78-100% for token management):

```bash
# Run all tests
go test -v ./...

# Run with coverage report
go test -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html

# Run specific tests
go test -v -run TestTokenManager
go test -v -run TestBootstrap
```

#### Integration Tests

Integration tests require real NIC.RU credentials and API calls:

```bash
# Setup credentials
cp test-credentials.example.env test-credentials.env
vim test-credentials.env  # Add credentials + NICRU_TEST_ZONE

# Load and run
source test-credentials.env
go test -v -tags=integration ./tests/integration/
```

**⚠️ Important:** Set `NICRU_TEST_ZONE` to a safe test domain. DNS tests will create/delete records in this zone.

See detailed guide: [tests/integration/README.md](tests/integration/README.md)

### Test Coverage

- **token_manager.go**: 78-100% coverage (29 tests)
- **bootstrap.go**: 100% coverage (6 tests)
- **Critical functions**: 100% coverage (retry logic, credentials, masking)
