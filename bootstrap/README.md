# Bootstrapping a single-node cluster for testing

## Boostrapping a cluster in a container

```
docker run --cap-add sys_resource --rm -p 9443:9443 -p 8443:8443 -p 12000-12100:12000-12100 redislabs/redis:6.0.12-49
```

The ports:

 * 8443 - the admin UI
 * 9443 - the REST API
 * 12000-12100 - a port reserved for a databases (you may add more of these)

## Bootstrap the node

The bootstrap the cluster request:

```
cat << EOF > bootstrap.json
{
   "action" : "create_cluster",
   "cluster" : {
      "nodes" : [],
      "name" : "host.docker.internal"
   },
   "credentials" : {
      "username": "admin",
      "password": "xyzzyxyzzy"
   }
}
EOF
```

Bootstrap the cluster in the container:

```
curl -vk -X POST -H "Content-Type: application/json" -d @bootstrap.json https://localhost:9443/v1/bootstrap/create_cluster
```

Check the cluster creation:

```
curl -vk -u admin:xyzzyxyzzy https://localhost:9443/v1/bootstrap | jq .bootstrap_status.state
```

## Setup a database

If you want to replicate:

```YAML
apiVersion: app.redislabs.com/v1alpha1
kind: RedisEnterpriseDatabase
metadata:
  labels:
    app: redis-enterprise
  name: mydb
spec:
  memorySize: 100MB
  rolesPermissions:
  - type: redis-enterprise
    role: "DB Member"
    acl: "Not Dangerous"
```

Find the role and ACL uid values by name:

```
export ROLE_ID=`curl -ks -u admin:xyzzyxyzzy https://localhost:9443/v1/roles | jq '.[] | select(.name=="DB Member").uid'`
export ACL_ID=`curl -ks -u admin:xyzzyxyzzy https://localhost:9443/v1/redis_acls | jq '.[] | select(.name=="Not Dangerous").uid'`
```

Use these values to create a database creation request:

```
cat << EOF > bdb.json
{
   "name" : "mydb",
   "type" : "redis",
   "memory_size" : 104857600,
   "port" : 12000,
   "authentication_redis_pass" : "xyzzyxyzzy",
   "roles_permissions" : [
      {
         "role_uid" : $ROLE_ID,
         "redis_acl_uid" : $ACL_ID
      }
   ]
}
EOF
```

Create the database:

```
curl -vk -u admin:xyzzyxyzzy -X POST -H "Content-Type: application/json" -d @bdb.json https://localhost:9443/v1/bdbs
```

To check for existence (filtering for names):

```
curl -k -u  admin:xyzzyxyzzy https://localhost:9443/v1/bdbs | jq ".[].name"
```

To delete (by id only):

```
DBID=`curl -ks -u admin:xyzzyxyzzy https://localhost:9443/v1/bdbs | jq '.[] | select(.name="mydb").uid'`
curl -k -u admin:xyzzyxyzzy -X DELETE https://localhost:9443/v1/bdbs/$DBID
```
