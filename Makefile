LOCAL=127.0.0.1
ADDR=:12345
NAME=Ian
#Vee
#Sydney
TEST_DIR=./internal/dht

run:
	@go run ./cmd/park-node -name ${NAME} -data ~/.p2p-park-${NAME}

bootstrap:
	@go run ./cmd/park-node -name ${NAME} -bootstrap ${LOCAL}${ADDR}

race:
	GOFLAGS=-race go test ./...

racedir:
	GOFLAGS=-race go test -count=1 ${TEST_DIR} -v

cleancache:
	@go clean -cache