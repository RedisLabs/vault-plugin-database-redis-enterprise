# Using the plugin on K8s

## Install

The [override-values.yaml](../../examples/k8s/override-values.yaml) file shows an example of
configuring vault to provide a plugin via a volume.

Install vault into the `vault` namespace via helm:
```
helm install vault hashicorp/vault --namespace vault -f override-values.yaml
```

Once running, copy the plugin to the container:

```
kubectl cp -n vault ../bin/vault-plugin-database-redisenterprise_linux_amd64 vault-0:/usr/local/libexec/vault
```

Attach to vault:

```
kubectl exec -it -n vault vault-0 /bin/sh
```

Initialize the vault:
```
vault operator init
```

You should see your vault keys:
```
Unseal Key 1: OtthRGm05X3B2zQ7+JpE0pfrw40tSiW+meUsSu3UIIGm
Unseal Key 2: yVEJSEXm6ZTWitIrmjZbmUFNUu1HKPrXDLlua8UTANWF
Unseal Key 3: 5FDnnl9qNWah7PayrLBCJhwyEaL9Uq6CTfnSKY3Ij7uV
Unseal Key 4: oj6USpR1uWsrhsRF6T69xspECoO9v2qbnx94InkDTBls
Unseal Key 5: NXO7viNCzi6KgLY63IsJmk4WJ0aUGFZP1TAfet0b0rq6

Initial Root Token: s.4NZawd2Ti83poa4gRzPXJLC6

```

Afterwards, unseal the vault (using three different keys):

```
vault operator unseal
vault operator unseal
vault operator unseal
```

Get the sha256:

```
shasum -a 256 ../bin/vault-plugin-database-redisenterprise_linux_amd64 | awk '{print $1}'
```

Setup your vault token and enable the plugin by replacing the sha256 of
the plugin in the last part of the `vault write ... sha256=...`:

```
export VAULT_TOKEN=...
vault write sys/plugins/catalog/database/redisenterprise-database-plugin command=vault-plugin-database-redisenterprise_linux_amd64 sha256=...
vault secrets enable database
```

## Setup a database role

Throughout this section we'll assume a database created with the following CR named `mydb`:

```YAML
apiVersion: app.redislabs.com/v1alpha1
kind: RedisEnterpriseDatabase
metadata:
  name: mydb
spec:
  memory: 100MB
  rolesPermissions:
  - type: redis-enterprise
    role: "DB Member"
    acl: "Not Dangerous"
```

Assuming a cluster with a name of "test" in a "redis" namespace, you can
get the credentials for the cluster administrator:

```
kubectl -n redis get secret/test -o=jsonpath={.data.username} | base64 -d
kubectl -n redis get secret/test -o=jsonpath={.data.password} | base64 -d
```

The endpoint will be `https://test.redis.svc:9443`:

Attach to the vault pod:

```
kubectl exec -it -n vault vault-0 /bin/sh
```

Setup the vault token:

```
export VAULT_TOKEN=...
```

and configure a database using your
Redis Enteprrise endpoint and credentials:

```
vault write database/config/redis-test-mydb plugin_name="redisenterprise-database-plugin" url="https://test.redis.svc:9443" allowed_roles="*" database=mydb username=... password=...
```

Then configure a database role:

```
vault write database/roles/mydb db_name=redis-test-mydb creation_statements="{\"role\":\"DB Member\"}" default_ttl=3m max_ttl=10m
```

## Testing the role

Forward your database locally:

```
kubectl port-forward -n redis service/mydb `kubectl get -n redis service/mydb -o=jsonpath="{.spec.ports[0].targetPort}"`
```

Run the redis-cli:

```
redis-cli -p `kubectl get -n redis service/mydb -o=jsonpath="{.spec.ports[0].targetPort}"`
```

and authenticate with the AUTH command using the credentials returned by
reading the vault database role path via the vault CLI within the vault
pod.

Get a credential for your database via :

```
vault read database/creds/mydb
```

which will output something similar to:

```
Key                Value
---                -----
lease_id           database/creds/mydb/Vhpim6mizcc4UOKfEpmTSLj5
lease_duration     3m
lease_renewable    true
password           7L-YHEbMAQB38CtwAF5l
username           v_root_mydb_ychqg0ueic1abdkyehdi_1613680470
```

In the Redis CLI, you can now do:

```
AUTH v_root_mydb_ychqg0ueic1abdkyehdi_1613680470 7L-YHEbMAQB38CtwAF5l
```

## Using the sidecar injector

Enable Kubernetes authentication:

```
vault auth enable kubernetes
vault write auth/kubernetes/config \
    token_reviewer_jwt="$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
    kubernetes_host="https://$KUBERNETES_PORT_443_TCP_ADDR:443" \
    kubernetes_ca_cert=@/var/run/secrets/kubernetes.io/serviceaccount/ca.crt
```

Create a policy that enables reading the credential:

```
vault policy write mydb - <<EOF
path "database/creds/mydb" {
  capabilities = ["read"]
}
EOF
```

Enable the service account to use the policy:

```
vault write auth/kubernetes/role/mydb \
      bound_service_account_names=workload \
      bound_service_account_namespaces=redis \
      policies=mydb \
      ttl=24h
```

Add annotations to inject the secret:

```
annotations:
 vault.hashicorp.com/agent-inject: 'true'
 vault.hashicorp.com/agent-inject-secret-mydb: 'database/creds/mydb'
 vault.hashicorp.com/agent-inject-template-mydb: |
    {{- with secret "database/creds/mydb" -}}
    {"username":"{{ .Data.username }}","password":"{{ .Data.password }}"}
    {{- end }}
 vault.hashicorp.com/role: 'mydb'
```

See the full example of a [workload deployment](../../examples/k8s/log-auth.yaml) for all the details.

## Upgrading the plugin

Copy the new version of the plugin to the container:

```
kubectl cp -n vault ../vault-plugin-database-redisenterprise_linux_amd64 vault-0:/usr/local/libexec/vault
```

Attach to the vault pod:

```
kubectl exec -it -n vault vault-0 /bin/sh
```

Setup the vault token and register the new version:

```
export VAULT_TOKEN=...
vault write sys/plugins/catalog/database/redisenterprise-database-plugin command=vault-plugin-database-redisenterprise_linux_amd64 sha256=...
```
