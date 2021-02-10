# Vault Database Secrets Engine - Redis Enterprise Plugin

A Redis Enterprise plugin for the HashiCorp Vault Database Secrets Engine.

This is a standalone backend plugin for use with Hashicorp Vault. 
This plugin generates credentials for use with Redis Enterprise Database clusters.

## Quick Links
- [Redis Enterprise Docs](https://redislabs.com/redis-enterprise-software/overview)
- [Vault Website](https://www.vaultproject.io)
- [Vault Github Project](https://www.github.com/hashicorp/vault)

## Development

If you wish to work on this plugin, you'll first need [Go](https://www.golang.org) installed on your machine 
(version 1.15+ is *required*).

Make sure Go is properly installed, including setting up a [GOPATH](https://golang.org/doc/code.html#GOPATH).

To run the tests locally you will need to have [Docker](https://docs.docker.com/get-docker) installed on your machine.

Clone this repository:

```sh
$ git clone https://github.com/RedisLabs/vault-plugin-database-redis-enterprise
$ cd vault-plugin-database-redis-enterprise
```

or use `go get github.com/RedisLabs/vault-plugin-database-redis-enterprise`

### Building

To compile this plugin, run `make` or `make build`.  This will put the plugin binary in a local `bin` directory.
By default this will generate a binary for your local platform and a binary for `linux`/`amd64`.

```sh
$ make build
```

### Testing

Testing this plugin takes two forms.  The first is the execution of the tests against a set of recorded fixtures
and the second executes tests against a running Redis Enterprise Cluster.

To execute the tests against the recorded fixtures run `make test`.  
Fixtures have been included to allow the tests to execute quickly within the plugin's CI process. 

```sh
$ make test
```

To execute the tests against an existing live Redis Enterprise cluster run `make test-acc`.
The plugin's makefile contains default values to locate a Redis Enterprise cluster provisioned through 
the makefile.  The following example shows how these values can be overridden to locate your own cluster.

```sh
$ export TEST_USERNAME=admin
$ export TEST_PASSWORD=xyzzyxyzzy
$ export TEST_DB_NAME=mydb
$ export TEST_DB_URL=https://localhost:9443

$ make test-acc
```

## Running Vault + Plugin + Redis Enterprise

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
Once setup is complete run `make test-acc` to execute the acceptance test against the local containers.

```sh
$ make test-acc
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
and these steps are coordinated through the repo's GitHub Action workflows.  To test changes to the gotrelease 
configuration or to build the different binaries locally the following command can be executed.

```sh
$ goreleaser --snapshot --skip-publish --rm-dist
```

A `dist` directory will be created and there will be one binary for each OS/Arch combination defined in the root
`goreleaser.yml` file.  A SHA256SUMS file will also be produced with entries for each binary.  
The SHA values can be used when installing the plugin to a running Vault server. 

**Note:**  If you are running Vault via docker the plugin architecture if likely to be `linux/amd64`.
This binary is also produced through the `make build`
