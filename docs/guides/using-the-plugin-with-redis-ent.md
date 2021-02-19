## Using the plugin with Redis Enterprise

### Running with a local Vault

For demonstration purposes in the follow guide, you can run a local version
of Vault. In your build directory for the plugin, run Vault so that it has access to the
plugin binary:

```
docker run --rm --cap-add=IPC_LOCK -e 'VAULT_DEV_ROOT_TOKEN_ID=xyzzyxyzzy' -v `pwd`/bin:/etc/vault/plugins -e 'VAULT_LOCAL_CONFIG={"plugin_directory":"/etc/vault/plugins"}' vault
```

### Configure the plugin

From your build directory, calculate the sha256 checksum:

```
shasum -a 256 bin/vault-plugin-database-redisenterprise_linux_amd64 | cut -d' ' -f1
```

Now, attach to the running Vault container:

```
VAULT_NAME=`docker ps -f ancestor=vault --format "{{.Names}}"`
docker exec -it $VAULT_NAME sh
```

In the shell, setup the local Vault authentication:

```
export VAULT_TOKEN=$VAULT_DEV_ROOT_TOKEN_ID
export VAULT_ADDR=http://127.0.0.1:8200
```

Using the sha256 that you calculated above, modify this command to
register the plugin:

```
vault write sys/plugins/catalog/database/redisenterprise-database-plugin command=vault-plugin-database-redisenterprise_linux_amd64 sha256=...
```

Finally, enable the database secrets engine:

```
vault secrets enable database
```

At this point, you can configure database roles for Redis Enterprise.

### Configure a database

The following will config a Vault configuration of a Redis database called `mydb`. Note
that the `allowed_roles` specifies the Vault role names and not the Redis user role. In
this example, we have enabled all vault roles with a wildcard.

Using the defaults for a cluster setup, there is a cluster administrator account
in the kubernetes secret for the cluster. You can retrieve these by:

```
kubectl get secret/test -o=jsonpath={.data.username} | base64 -d
kubectl get secret/test -o=jsonpath={.data.password} | base64 -d
```

Use these values to configure a database, replacing the `...` at the end with
the username and password, respectively:

```
vault write database/config/redis-mydb plugin_name="redisenterprise-database-plugin" url="https://host.docker.internal:9443" allowed_roles="*" database=mydb username=... password=...
```


### Configure database user with a role

A user is associated with a role binding in the database. You
reference a role bound to an ACL within the database. This role binding
can be defined via the K8s database controller or via the administrative
user interface.

You can reference only the role:

```
vault write database/roles/mydb db_name=redis-mydb creation_statements="{\"role\":\"DB Member\"}" default_ttl=3m max_ttl=5m
```

or add the ACL as well as an assertion:

```
vault write database/roles/mydb-role-acl db_name=redis-mydb creation_statements="{\"role\":\"DB Member\",\"acl\":\"Not Dangerous\"}" default_ttl=3m max_ttl=5m
```

If the ACL is also specified, the plugin will check to ensure it has the same
binding in the database. It is an error if the role does not have the same
binding to the same ACL in the database.

When used, a new user is generated and associated with the role referenced,

A role binding in a database is never generated when using an existing role as this would
allow escalation of privileges in the database for others users with the same role.

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

### Configuring a database user with an ACL only

This feature must be enabled When the database is configured via the "features"
parameter with the vault "acl_only":

```
vault write database/config/redis-mydb plugin_name="redisenterprise-database-plugin" url="https://host.docker.internal:9443" allowed_roles="*" database=mydb features=acl_only username=... password=...
```

With this feature turned on, a database role can reference only an ACL. A role
is dynamically generated for the user and bound in the database. In doing
so, it changes the database definition. As such, it cannot be used with the
K8s database controller as it also manages the database role permission bindings.

This feature is used by specifying only the ACL:

```
vault write database/roles/mydb-acl db_name=redis-mydb creation_statements="{\"acl\":\"Not Dangerous\"}" default_ttl=3m max_ttl=5m
```

When used, the generated user will have a generated role that is dynamically
bound in the database to the ACL. When the user expires, the role and role
binding is removed.


### Reading credentials

Once the Vault role is configured, a workload can create a new credential by just
reading the Vault role:

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
username           vault-mydb-69dea4c9-4da8-4e34-bf93-eebf60095766
```

A workload can renew the lease on the password before the lease expires up to the maximum expiry:

```
vault lease renew database/creds/test/zgVJfei8P0Tw7cKX3g9Hx89l
```

If the lease expires or the maximum expiry is reached, the user is revoked by
Vault. When the user is revoked, the plugin will delete the user and all
the corresponding create items (i.e., the role, binding, and user) are deleted.

Note that the `roles_permissions` on the database will be updated during this process
if the ACL-only feature is used.

### Using credentials

The username and password can be directly used in the Redis `AUTH` command:

```
AUTH vault-mydb-69dea4c9-4da8-4e34-bf93-eebf60095766 ZWI87ddZMPR7hR8U-3sJ
```

On a test cluster, you can forward the database port:

```
kubectl port-forward service/mydb `kubectl get service/mydb -o=jsonpath="{.spec.ports[0].targetPort}"`
```

Use the `redis-cli` to connect and authenticate:

```
redis-cli -p `kubectl get service/mydb -o=jsonpath="{.spec.ports[0].targetPort}"` --user vault-mydb-942fb9fe-f5c7-49d9-bce2-151c4c3c5343 --pass bxp-8GDDdZrDpbRfDzxg
```

where the username and password are the credentials returned by vault.
