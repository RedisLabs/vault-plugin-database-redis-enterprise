
# Redis Enterprise Database Secrets Engine

This secrets engine is a part of the Database Secrets Engine. If you have not read the [Database Backend](https://www.vaultproject.io/docs/secrets/databases)
page, please do so now as it explains how to set up the database backend and gives an overview of how the engine functions.

Redis Enterprise is a partner supported plugin for the database secrets engine. It is capable of dynamically generating
credentials based on configured roles for Redis Enterprise clusters.

## Capabilities

|Plugin Name	Root | Credential Rotation | Dynamic Roles | Static Roles|
|:----------------:|:-------------------:|:-------------:|:-----------:|
| Customizable (see: [Custom Plugins](https://www.vaultproject.io/docs/secrets/databases/custom)) | Yes | Yes | Yes |

## Setup
The Redis Enterprise database plugin is not bundled in the core Vault code tree and can be found at its own
git repository here: https://github.com/RedisLabs/vault-plugin-database-redis-enterprise

For linux/amd64, pre-built binaries can be found at the [releases page](http://TBC)

**1. Enable the database secrets engine if it is not already enabled:**

```shell
$ vault secrets enable database
Success! Enabled the database secrets engine at: database/
```

By default, the secrets engine will enable at the name of the engine.
To enable the secrets engine at a different path, use the `-path` argument.

**2. Download and register the plugin:**

```shell
$ vault write sys/plugins/catalog/database/vault-plugin-database-redisenterprise \
    sha256="..." \
    command=vault-plugin-database-redisenterprise
Success! Data written to: sys/plugins/catalog/database/vault-plugin-database-redisenterprise
```

**3. Configure Vault with the proper plugin and connection information:**

```shell
$ vault write database/config/redis-mydb plugin_name="vault-plugin-database-redisenterprise" \
    url="https://localhost:9443" \
    allowed_roles="*" \
    database={{database}} \
    username={{username}} \
    password={{password}}
```

**Note:** It is highly recommended that you immediately rotate the "root" user's password.
(see [Rotate Root Credentials](https://www.vaultproject.io/api/secret/databases#rotate-root-credentials)).
This will ensure that only Vault is able to access the "root" user that Vault uses to manipulate dynamic & static credentials.

**Use caution:** the root user's password will not be accessible once rotated so it is highly recommended that you create
a user for Vault to utilize rather than using the actual root user.

**4. Configure a role that maps a name in Vault to an Redis Enterprise role statement to execute:**

```shell
$ vault write database/roles/redis-mydb \
  db_name=redis-mydb \
  creation_statements='{"role":"DB Member"}' \
  default_ttl=3m \
  max_ttl=5m
Success! Data written to: database/roles/my-role
```

**Note:** The creation_statements may be specified in a file and interpreted by the Vault CLI using the @ symbol:

```shell
$ vault write database/roles/redis-mydb \
    db_name=redis-mydb \
    creation_statements=@creation.json \
    default_ttl=3m \
    max_ttl=5m
    ...
```

See the [Commands](https://www.vaultproject.io/docs/commands#files) docs for more details.

## Usage

### Dynamic Credentials

After the secrets engine is configured and a user/machine has a Vault token with the proper permission,
it can generate credentials.

Generate a new credential by reading from the /creds endpoint with the name of the role:

```shell
$ vault read database/creds/redis-mydb
Key                Value
---                -----
lease_id           database/creds/redis-mydb/L7aTdIYx3CXk3DSqw76JcLor
lease_duration     3m
lease_renewable    true
password           GP-OsO9BpgtjH8PK6MKU
username           v_root_redis-mydb_bh3d1ha82e46opgaarto_1613047913
```

**Note**: As Redis Enterprise does not support automatically expiring the users created for a dynamic credential, these users may still be active if Vault is unable to communicate with Redis Enterprise when the leased secret expires as the plugin will be unable to delete the users. You can attempt to manually revoke the leased secret by using the `vault lease revoke <lease_id>`, where the `<lease_id>` will appear in the Vault logs like `2021-02-03T10:43:47.943Z [ERROR] expiration: maximum revoke attempts reached: lease_id=database/creds/mydb/cpXGOg2hJ6uE0OWJXphXhLth`. A newly elected Vault HA leader will automatically attempt to any leases that have expired but haven't yet been deleted, so will try to delete the user again.
## API

For more information on the database secrets engine's HTTP API please see the
[Database secrets engine API](https://www.vaultproject.io/api/secret/databases) page.
