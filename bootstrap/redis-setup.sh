#!/usr/bin/env bash

# Simple script to configure the redislabs/redis image to make it easy to run the tests against it

# Exit on error. Append || true if you expect an error.
set -o errexit
# Exit on error inside any functions or subshells.
set -o errtrace
# Do not allow use of undefined vars. Use ${VAR:-} to use an undefined VAR
set -o nounset
# Catch the error in case mysqldump fails (but gzip succeeds) in `mysqldump |gzip`
set -o pipefail
# Turn on traces, useful while debugging but commented out by default
#set -o xtrace

username="admin"
password="xyzzyxyzzy"
db_name="mydb"

while test $# -gt 0; do
  case "$1" in
  -h|--help)
    echo "Configure the Redis Enterprise Software within redislabs/redis to make it easier to run the tests"
    echo ""
    echo "options:"
    echo "-u [username]  username to use as the Administrator of the cluster and database"
    echo "-p [password]  password to use for the Administrator of the cluster and database"
    echo "-db [name]     name of the initial database within the cluster"
    exit
    ;;
  -u)
    shift
    username=$1
    shift
    ;;
  -p)
    shift
    password=$1
    shift
    ;;
  -db)
    shift
    db_name=$1
    shift
    ;;
  esac
done

while [[ "$(curl -k -s -o /dev/null -w '%{http_code}' https://localhost:9443/v1/bootstrap)" != "200" ]]; do
  echo "waiting for the container to finish starting up"
  sleep 5;
done

# Initialise the cluster and wait for it to finish being created
jq --null-input --arg username "$username" --arg password "$password" \
  '{action:"create_cluster",cluster:{nodes:[],name:"host.docker.internal"},credentials:{username:$username,password:$password}}' | \
  curl -fks -X POST "https://localhost:9443/v1/bootstrap/create_cluster" -H "Content-Type: application/json" -H  "accept: application/json" -d @- -o /dev/null

while [ "$(curl -fks -u "$username:$password" 'https://localhost:9443/v1/bootstrap' | jq -r .bootstrap_status.state)" != "completed" ]; do
  echo "waiting for cluster bootstrap"
  sleep 1;
done

# Create the initial database and wait for it to finish being set up
acl_id=$(curl -fks -u "$username:$password" https://localhost:9443/v1/redis_acls | jq '.[] | select(.name=="Not Dangerous").uid')
role_id=$(curl -fks -u "$username:$password" https://localhost:9443/v1/roles | jq '.[] | select(.name=="DB Member").uid')
jq --null-input --arg acl_id "$acl_id" --arg role_id "$role_id" --arg db_name "$db_name" --arg password "$password" \
  '{name:$db_name,type:"redis",memory_size:104857600,port:12000,authentication_redis_pass: $password,roles_permissions : [{role_uid : $role_id | tonumber,redis_acl_uid : $acl_id | tonumber}]}' | \
  curl -fks -u "$username:$password" -X POST -H "Content-Type: application/json" -d @- https://localhost:9443/v1/bdbs -o /dev/null

while [ "$(curl -fks -u "$username:$password" https://localhost:9443/v1/bdbs | jq --arg db_name "${db_name}" -r '.[] | select(.name == $db_name) | .status')" != "active" ]; do
  echo "waiting for database setup"
  sleep 1;
done

echo "done"
