# vault-plugins
HashiCorp Vault Plugins for Redis Enterprise

## Overview

This plugin supports:

 * cluster-wide users with access to multiple databases.
 * database users via a role bound in the database
 * database users with a specific Redis ACL

A **cluster-wide user** are provided access to databases by binding the role
in the database. This binding is outside of the scope of the plugin and
typically done through the administrator interfaces (UI or API) or via
the REDB CR on K8s.

A **database user with a role** is provided access by replicating a role
that is bound in a database. A new role is generated and assigned to the
user with the same binding. This enables role-based modeling of user
capabilities.

A **database user with an ACL** is provided access by creating a new role
and role binding in the database. This is the most dynamic and requires no
configuration by the administrator except when a new ACL is required to be
created.

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

### Configure a cluster or database:

To configure all databases:

```
vault write database/config/redis-test plugin_name="redisenterprise-database-plugin" url="https://host.docker.internal:9443" allowed_roles="*" username="demo@redislabs.com" password=...
```

To configure a specific database, add `database` (note the different configuration name 'redis-mydb'):

```
vault write database/config/redis-mydb plugin_name="redisenterprise-database-plugin" url="https://host.docker.internal:9443" allowed_roles="*" username="demo@redislabs.com" database=mydb password=...
```


### Configure database acccess

Note: users are associated with roles in Redis Enterprise. As such, the
association of user to role in database is accomplished via the role binding
in the database. On K8s, this is via the REDB CR (i.e., via the database controller).

Create a role for the user:

```
vault write database/roles/test db_name=redis-test creation_statements="{\"role\":\"DB Member\"}" default_ttl=3m max_ttl=5m
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

The above credential will work for whatever database has the role bound to an ACL.

If you use a role that is bound to a specific database:

```
vault write database/roles/mydb db_name=redis-mydb creation_statements="{\"role\":\"DB Member\"}" default_ttl=3m max_ttl=5m
```

Or you can use the Redis ACl directly:

```
vault write database/roles/mydb db_name=redis-mydb creation_statements="{\"acl\":\"Not Dangerous\"}" default_ttl=3m max_ttl=5m
```

Note that an ACL can only be used when there is a database
(via the definition in `db_name`). When providing cluster-wide users,
the database ACL binding is only provided via the pre-defined role.

The credentials return will be bound to a new role assigned to only that database
with the requested Redis ACL. When using the role, an administrator can use
existing roles like "DB Member" or "DB Viewer" to model the desired capabilities.
This allows static users to have the same capabilities as the dynamic user.
As the dynamic role that is created and bound in the database is only
assigned to a particular user and granted by the vault plugin, that particular
access is not available to other
users.

Once the role is configured, a workload can create a new credential by just
reading the role:

```
vault read database/creds/mydb
```

A workload can renew the password before the lease expires up to the maximum expiry:

```
vault lease renew database/creds/test/zgVJfei8P0Tw7cKX3g9Hx89l
```

A lease renewal changes the password for the user. If the lease expires, the
user is deleted by Vault. The renew is allowed up to the expiry for the user.
Once the user expires, Vault will delete the user. When the user expires,
all the corresponding create items (i.e., the role,
binding, and user) are deleted.
