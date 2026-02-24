# Cert-manager webhook для API nic.ru

Этот solver позволяет использовать cert-manager с API nic.ru. Документация по API доступна [здесь](https://www.nic.ru/help/upload/file/API_DNS-hosting.pdf).

# Использование

### Подготовка

Сначала необходимо [зарегистрировать](http://nic.ru/api/oauth/register_app.html) приложение в личном кабинете nic.ru.
После регистрации вы получите `app_id` и `app_secret` протокола OAuth 2.0, которые нужны для получения токенов. Эти два токена нужно использовать для получения ACCESS_TOKEN && REFRESH_TOKEN, помимо них нужен административный ( или технический пароль) и номер договора.

Для получения ACCESS_TOKEN && REFRESH_TOKEN выполните команду:

```shell
curl --location --request POST 'https://api.nic.ru/oauth/token' \
--header 'Content-Type: application/x-www-form-urlencoded' \
--data-urlencode 'grant_type=password' \
--data-urlencode 'password=<административный_или_технический_пароль>' \
--data-urlencode 'client_id=<ВАШ_APP_ID>' \
--data-urlencode 'client_secret=<ВАШ_APP_SECRET>' \
--data-urlencode 'username=<номер_договора_например_53XXX/NIC-D>' \
--data-urlencode 'scope=.*' \
--data-urlencode 'offline=999999'
```

В ответ придёт:

```json
{"refresh_token":"<REFRESH_TOKEN>","expires_in":14400,"access_token":"<ACCESS_TOKEN>","token_type":"Bearer"}
```

------------------------------

### Установка cert-manager (*необязательный шаг*)

**ВНИМАНИЕ!** Не удаляйте cert-manager, если вы уже им пользуетесь.

Для установки cert-manager в кластер Kubernetes используйте команду из [официальной документации](https://cert-manager.io/docs/installation/):

```shell
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/VERSION/cert-manager.yaml
```

### Установка webhook

**ВАЖНО**: Ресурсы Kubernetes для установки webhook должны быть развёрнуты в том же namespace, что и cert-manager.

```shell
git clone https://github.com/flant/cert-manager-webhook-nicru.git
```

Укажите ваш namespace с cert-manager:

```yaml
certManager:
  namespace: ваш-namespace-cert-manager
  serviceAccountName: cert-manager
```

### Создание секрета с токенами

Создайте файл `nicru-tokens.yaml` со следующим содержимым:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: nicru-tokens
  namespace: ваш-namespace-cert-manager
data:
  REFRESH_TOKEN: <REFRESH_TOKEN_В_BASE64>
  ACCESS_TOKEN: <ACCESS_TOKEN_В_BASE64>
  APP_ID: <APP_ID>
  APP_SECRET: <APP_SECRET>
type: Opaque
```

Затем выполните команды для установки webhook:

```shell
cd cert-manager-webhook-nicru
helm install -n my-namespace-cert-manager nicru-webhook ./helm
```

### Выпуск сертификата

Создайте файл `certificate.yaml` со следующим содержимым:

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
    - "*.my-domain-test.ru"
```

### Диагностика проблем

Токены ACCESS_TOKEN && REFRESH_TOKEN доступны только 4 часа, соответсвенно их ообновляет наш деплоймент с вебхуком. Об этом, а также о том, что токены вообще валидны - пишется в логах.

В целом все действия логгируются достаточно подробно - используйте это для дебага ошибок.

Важно также смотреть ордеры и челенджи дескрайбом.

```
get orders
get challenges
describe challenges ...
```

В них как правило вообще все ошибки и действия.

Апи у nic.ru меняется регулярно, зачем - спросите у них, соотвественно поломка ишьюра лишь дело времени, обратите на это внимание. 
