LOCAL=127.0.0.1
ADDR=:12345
NAME=Ian
#Vee
#Sydney

run:
	@go run ./cmd/park-node -name ${NAME}

bootstrap:
	@go run ./cmd/park-node -name ${NAME} -bootstrap ${LOCAL}${ADDR}

race:
	GOFLAGS=-race go test ./...