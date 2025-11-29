LOCAL=127.0.0.1
ADDR=:12345
NAME=Vee
#Sydney

init:
	@go run ./cmd/park-node -name Ian

join:
	@go run ./cmd/park-node -name ${NAME} -bootstrap ${LOCAL}${ADDR}