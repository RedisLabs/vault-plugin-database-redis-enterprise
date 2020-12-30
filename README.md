# vault-plugins
HashiCorp Vault Plugins for Redis Enterprise

## Overview

This plugin supports:

 * database users with a specific Redis ACL
 * database users via a role bound in the database
 * cluster-wide users with access to multiple databases.

A **database user with an ACL** provides access by creating a new role
and role binding in the database. This is the most dynamic and requires no
configuration by the administrator except when a new ACL is required to be
created.

A **database user with a role** provides access by replicating a role
that is bound in a database. A new role is generated and assigned to the
user with the same binding. This enables role-based modeling of user
capabilities.

A **cluster user** are provided access to databases by the role
of the user. The role binding in the database is controlled by the
cluster administrator and not the plugin.

In all cases, the user is created dynamic and deleted when it expires.

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

### Configure a database:

The following will config a Vault configuration of a Redis database called `mydb`. Note
that the `allow_roles` specifies the Vault role names and not the Redis user role. In
this example, we have enabled all vault roles with a wildcard.

```
vault write database/config/redis-mydb plugin_name="redisenterprise-database-plugin" url="https://host.docker.internal:9443" allowed_roles="*" username="demo@redislabs.com" database=mydb password=...
```


### Configure database user role

A user is associated with a role binding in the database. You must either
reference a currently configured role binding or a Redis ACL.

If you want to use a role that is bound in your database:

```
vault write database/roles/mydb db_name=redis-mydb creation_statements="{\"role\":\"DB Member\"}" default_ttl=3m max_ttl=5m
```

If you want to use a Redis ACL directly:

```
vault write database/roles/mydb db_name=redis-mydb creation_statements="{\"acl\":\"Not Dangerous\"}" default_ttl=3m max_ttl=5m
```

The user credentials returned will be bound to a new role assigned assigned
to a new user where that role is bound the requested ACL in the requested
database.

When using the role, an administrator can model the capabilities of the user
with the reference role but a new role is always created with the same ACLs
as the referenced role. This allows static users to have the same capabilities
as the dynamic users without duplicating the role to ACL definition.

In either case, as the dynamic role that is created and bound in the database
is only assigned to a dynamically created user that is managed by the vault
plugin, that particular access is not available to other users.

Once the Vault role is configured, a workload can create a new credential by just
reading the role:

```
vault read database/creds/mydb
```

The result is similar to:

```
Key                Value
---                -----
lease_id           database/creds/mydb/zgVJfei8P0Tw7cKX3g9Hx89l
lease_duration     3m
lease_renewable    true
password           ZWI87ddZMPR7hR8U-3sJ
username           vault-test-69dea4c9-4da8-4e34-bf93-eebf60095766
```

A workload can renew the password before the lease expires up to the maximum expiry:

```
vault lease renew database/creds/test/zgVJfei8P0Tw7cKX3g9Hx89l
```

A lease renewal changes the password for the user.

If the lease expires or the maximum expiry is reached, the user is revoked by
Vault. When the user is revoked, the plugin will delete the user and all
the corresponding create items (i.e., the role, binding, and user) are deleted.

Note that the `roles_permissions` on the database will be updated during this process.

### Configuring a cluster user role

A cluster user has access to whatever database the associated role has been
given by the administrator. This may be a single database with a specific
ACL or multiple databases with different ACLs. The plugin does not manage
the role bindings and does not update the `roles_permissions` on the
database.

To configure a database that allows any database, just omit the `database`
parameter:

```
vault write database/config/redis-test plugin_name="redisenterprise-database-plugin" url="https://host.docker.internal:9443" allowed_roles="*" username="demo@redislabs.com" password=...
```

When you create the Vautl role for the user, you must specify a database role
(Redis ACLs are not allowed):

```
vault write database/roles/test db_name=redis-test creation_statements="{\"role\":\"DB Member\"}" default_ttl=3m max_ttl=5m
```

Read a credential is the same:

```
vault read database/creds/test
```

with the same output:

```
Key                Value
---                -----
lease_id           database/creds/test/zgVJfei8P0Tw7cKX3g9Hx89l
lease_duration     3m
lease_renewable    true
password           ZWI87ddZMPR7hR8U-3sJ
username           vault-test-69dea4c9-4da8-4e34-bf93-eebf60095766
```

The only difference is the user credentials will work for whatever databases
the Redis Enterprise cluster administrator has configured. No role is
dynamically generated. Instead, the user has the reference role directly.
