# vault-plugins
HashiCorp Vault Plugins for Redis Enterprise

## Building the plugin

```
go build
gox -osarch="linux/amd64" ./...
```

The plugin architecture must be for the target vault architecture. If you are
running via docker, this is likely `linux/amd64`.

## Testing the plugin

You you need a Redis Enterprise REST API endpoint and cluster administrator
username and password to run the tests.

```
export RS_API_URL=...
export RS_USERNAME=...
export RS_PASSWORD=...
go test
```

## Setup

### Run a local Vault

```
docker run --rm --cap-add=IPC_LOCK -e 'VAULT_DEV_ROOT_TOKEN_ID=xyzzyxyzzy' -v `pwd`:/etc/vault/plugins -e 'VAULT_LOCAL_CONFIG={"plugin_directory":"/etc/vault/plugins"}' vault
```

### Configure the plugin

Calculate the sha256 checksum:

```
shasum -a 256 vault-plugin-database-redisenterprise_linux_amd64 | cut -d' ' -f1
```

Attach to the container:

```
docker exec -it {name} sh
```

In the shell, setup the local vault authentication:

```
export VAULT_TOKEN=$VAULT_DEV_ROOT_TOKEN_ID
export VAULT_ADDR=http://127.0.0.1:8200
```

Enable the plugin:

```
vault write sys/plugins/catalog/database/redisenterprise-database-plugin command=vault-plugin-database-redisenterprise_linux_amd64 sha256=...
vault secrets enable database
```

### Configure a cluster:
```
vault write database/config/redis-test plugin_name="redisenterprise-database-plugin" url="https://host.docker.internal:9443" allowed_roles="test" username="demo@redislabs.com" password=...
```

### Configure database acccess

Note: users are associated with roles in Redis Enterprise. As such, the
association of user to role in database is accomplished via the role binding
in the database. On K8s, this is via the REDB CR (i.e., via the database controller).

Create a role for the database user:

```
vault write database/roles/test db_name=redis-test creation_statements="{\"role\":\"db_member\"}" default_ttl=3m max_ttl=5m
```

Read a credential:

```
vault read database/creds/test
Key                Value
---                -----
lease_id           database/creds/test/zgVJfei8P0Tw7cKX3g9Hx89l
lease_duration     3m
lease_renewable    true
password           ZWI87ddZMPR7hR8U-3sJ
username           vault-test-69dea4c9-4da8-4e34-bf93-eebf60095766
```

Renew a lease:
```
vault lease renew database/creds/test/zgVJfei8P0Tw7cKX3g9Hx89l
```
