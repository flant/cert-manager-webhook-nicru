# Cert-manager-webhook for the Nicru API

### Motivation

Cert-manager automates the management and issuance of TLS certificates in Kubernetes clusters. It ensures that certificates are valid and updates them when necessary.

A certificate authority resource, such as ClusterIssuer, must be declared in the cluster to start the certificate issuance procedure. It is used to generate signed certificates by honoring certificate signing requests.

For some DNS providers, there are no predefined CusterIssuer resources. Fortunately, cert-manager allows you to write your own webhook.

This solver allows you to use cert-manager with the Nicru API. Documentation on the Nicru API is available [here](https://www.nic.ru/help/upload/file/API_DNS-hosting.pdf).

# Usage

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

### Create a secret with tokens
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

Next, run the following commands for the install webhook.

```shell
cd cert-manager-webhook-nicru
helm install -n my-namespace-cert-manager nicru-webhook ./helm
```

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
