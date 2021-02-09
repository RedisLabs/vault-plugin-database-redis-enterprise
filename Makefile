TEST_USERNAME?=admin
TEST_PASSWORD?=xyzzyxyzzy
TEST_DB_NAME?=mydb
TEST_DB_URL?=https://localhost:9443

go_files := $(shell find . -path '*/testdata' -prune -o -type f -name '*.go' -print)

.DEFAULT_GOAL := all
.PHONY := all start-docker configure-docker stop-docker test test-acc build fmtcheck vet

vet: $(go_files)
	go vet  ./...

fmt:
	@go run golang.org/x/tools/cmd/goimports -w $(go_files)

fmtcheck: $(go_files)
	# Checking format of Go files...
	@GOIMPORTS=$$(go run golang.org/x/tools/cmd/goimports -l $(go_files)) && \
	if [ "$$GOIMPORTS" != "" ]; then \
		go run golang.org/x/tools/cmd/goimports -d $(go_files); \
		exit 1; \
	fi

test:
	RS_API_URL=https://localhost:9443 RS_USERNAME=go-vcr RS_PASSWORD=unused RS_DB=mydb go test -v ./...

test-acc:
	RS_DISABLE_FIXTURES=true RS_API_URL=$(TEST_DB_URL) RS_USERNAME=$(TEST_USERNAME) RS_PASSWORD=$(TEST_PASSWORD) RS_DB=$(TEST_DB_NAME) go test -v ./...

bin/vault-plugin-database-redisenterprise: $(go_files)
	go build -trimpath -o ./bin/vault-plugin-database-redisenterprise ./cmd/vault-plugin-database-redisenterprise

bin/vault-plugin-database-redisenterprise_linux_amd64: $(go_files)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o ./bin/vault-plugin-database-redisenterprise_linux_amd64 ./cmd/vault-plugin-database-redisenterprise

build: vet fmtcheck bin/vault-plugin-database-redisenterprise_linux_amd64 bin/vault-plugin-database-redisenterprise

start-docker:
	cd bootstrap && docker-compose up --detach
	./bootstrap/redis-setup.sh -u $(TEST_USERNAME) -p $(TEST_PASSWORD) -db $(TEST_DB_NAME)

configure-docker: bin/vault-plugin-database-redisenterprise_linux_amd64
	cd bootstrap && docker-compose exec -e VAULT_ADDR=http://localhost:8200 v vault login root
	cd bootstrap && docker-compose exec -e VAULT_ADDR=http://localhost:8200 v vault write sys/plugins/catalog/database/redisenterprise-database-plugin command=vault-plugin-database-redisenterprise_linux_amd64 sha256=$(shell shasum -a 256 ./bin/vault-plugin-database-redisenterprise_linux_amd64 | awk '{print $$1}')
	cd bootstrap && docker-compose exec -e VAULT_ADDR=http://localhost:8200 v vault secrets enable database
	cd bootstrap && docker-compose exec -e VAULT_ADDR=http://localhost:8200 v vault write database/config/redis-mydb plugin_name="redisenterprise-database-plugin" url="https://rp:9443" allowed_roles="*" database=$(TEST_DB_NAME) username=$(TEST_USERNAME) password=$(TEST_PASSWORD)
	cd bootstrap && docker-compose exec -e VAULT_ADDR=http://localhost:8200 v vault write database/roles/mydb db_name=redis-mydb creation_statements='{"role":"DB Member"}' default_ttl=3m max_ttl=5m
	cd bootstrap && docker-compose exec -e VAULT_ADDR=http://localhost:8200 v vault read database/creds/mydb

stop-docker:
	cd bootstrap && docker-compose down

all: test build
