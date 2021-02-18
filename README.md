# Vault Database Secrets Engine - Redis Enterprise Plugin

A Redis Enterprise plugin for the HashiCorp Vault Database Secrets Engine.

This is a standalone backend plugin for use with Hashicorp Vault.
This plugin generates credentials for use with Redis Enterprise Database clusters.

## Quick Links
- [Redis Enterprise Docs](https://redislabs.com/redis-enterprise-software/overview)
- [Vault Website](https://www.vaultproject.io) - primarily the area focused on
  [Custom Database Secrets Engine](https://www.vaultproject.io/docs/secrets/databases/custom)
- [Vault Github Project](https://www.github.com/hashicorp/vault)

## Development

If you wish to work on this plugin, you'll first need [Go](https://www.golang.org) installed on your machine
(version 1.15+ is *required*).

Make sure Go is properly installed, including setting up a [GOPATH](https://golang.org/doc/code.html#GOPATH).

To run the tests locally you will need to have [Docker](https://docs.docker.com/get-docker) installed on your machine,
or have access to a Kubernetes cluster such as by using [kind](https://kind.sigs.k8s.io/).

Clone this repository:

```sh
$ git clone https://github.com/RedisLabs/vault-plugin-database-redis-enterprise
$ cd vault-plugin-database-redis-enterprise
```

or use `go get github.com/RedisLabs/vault-plugin-database-redis-enterprise`

### Building

To compile this plugin, run `make build`.  This will put the plugin binary in a local `bin` directory.
By default, this will generate a binary for your local platform and a binary for `linux`/`amd64`.

```sh
$ make build
```

### Testing

Before being able to run the tests, you need to have access to a running Redis Enterprise Cluster.  This can either be
done by using the [Redis Enterprise Operator](https://docs.redislabs.com/latest/platforms/kubernetes/) to deploy Redis
into Kubernetes, or by using the [Redis Enterprise container](https://hub.docker.com/r/redislabs/redis) which can be
started up locally by running `make start-docker`.

To execute the tests run `make` or `make test`.
```sh
$ make test
```

The plugin's makefile contains default values to locate a Redis Enterprise cluster provisioned through
the makefile.  The following example shows how these values can be overridden to locate your own cluster.

```sh
$ export TEST_USERNAME=admin
$ export TEST_PASSWORD=xyzzyxyzzy
$ export TEST_DB_NAME=mydb
$ export TEST_DB_URL=https://localhost:9443

$ make test
```

### Running Vault + Plugin + Redis Enterprise

The repo provides a means to run a local Vault server configured with the Vault Plugin and backed by a Redis Enterprise
cluster.  To start the Vault server and Redis Enterprise cluster run `make start-docker` followed by `make configure-docker`
to configure Vault with a locally built plugin binary.

```sh
$ make start-docker

cd bootstrap && docker-compose up --detach
Creating network "bootstrap_vault" with the default driver
Creating bootstrap_v_1  ... done
Creating bootstrap_rp_1 ... done
./bootstrap/redis-setup.sh -u admin -p xyzzyxyzzy -db mydb
waiting for the container to finish starting up
...
waiting for the container to finish starting up
waiting for cluster bootstrap
...
waiting for cluster bootstrap
waiting for database setup
done

$ make configure-docker
...
```

A docker compose file representing a Redis Enterprise cluster and a Vault server will be provisioned.
Once setup is complete run `make test` to execute the acceptance test against the local containers.

```sh
$ make test
```

After local testing is complete the docker containers can be removed through the `make stop-docker`.

```sh
$ make stop-docker

cd bootstrap && docker-compose down
Stopping bootstrap_rp_1 ... done
Stopping bootstrap_v_1  ... done
Removing bootstrap_rp_1 ... done
Removing bootstrap_v_1  ... done
Removing network bootstrap_vault
```

### Building for Multiple Architectures

Vault operates across a number of different architectures and as a result Vault plugins must also be built to execute
across the same architectures.  This repo supports building the appropriate binaries through [goreleaser](https://github.com/goreleaser/goreleaser)
and these steps are coordinated through the repo's GitHub Action workflows.  To test changes to the goreleaser
configuration or to build the different binaries locally the following command can be executed.

```sh
$ goreleaser --snapshot --skip-publish --rm-dist
```

A `dist` directory will be created and there will be one binary for each OS/Arch combination defined in the root
`goreleaser.yml` file.  A SHA256SUMS file will also be produced with entries for each binary.  
The SHA values can be used when installing the plugin to a running Vault server.

**Note:**  If you are running Vault via Docker the plugin architecture if likely to be `linux/amd64`.
This binary is also produced through the `make build`

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
