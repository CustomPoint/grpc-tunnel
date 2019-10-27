# This Makefile builds
# Output will be in: ./out

YELLOW=\e[33m
GREEN=\e[32m
BLUE=\e[34m
DEFAULT=\e[0m

SERVER_LOG?="./out/server.log"
CLIENT_LOG?="./out/client.log"

all: clean build

.PHONY: all clean build prebuild regen
clean:
	@echo "${GREEN} = Cleaning up the build environment...${DEFAULT}"
	rm -rf ./out
	@echo "${GREEN} === DONE Cleaning === ${DEFAULT}"

regen:
	@echo "${GREEN} = Regenerating hijack...${DEFAULT}"
	protoc -I hijack/ hijack/hijack.proto --go_out=plugins=grpc:hijack
	@echo "${GREEN} === DONE Regen === ${DEFAULT}"

prebuild:
	mkdir -p ./out
	@echo "${GREEN} === DONE === ${DEFAULT}"

build: prebuild regen
	@echo "${YELLOW} = Building ... ${DEFAULT}"
	go build -o ./out/grpc_client ./grpc_client/
	go build -o ./out/grpc_server ./grpc_server/
	@echo "${GREEN} === DONE === ${DEFAULT}"

run_server:
	@echo "${YELLOW} = Attempting to run server ... ${DEFAULT}"
	./out/grpc_server &> ${SERVER_LOG} &

run_client:
	@echo "${YELLOW} = Attempting to run client ... ${DEFAULT}"
	./out/grpc_client

run: run_server run_client
	@echo "${GREEN} === DONE === ${DEFAULT}"

kill:
	@echo "${YELLOW} = Killing processes... ${DEFAULT}"
	ps -ef | grep grpc | awk '{print $$2}' | xargs kill -9
	@echo "${GREEN} === DONE === ${DEFAULT}"

go_check:
	@echo "${YELLOW} = Checking go code for formatting smells...${DEFAULT}"
	gofmt -l -e -d -s ./.
	@echo "${YELLOW} = Checking go code for suspicious constructs...${DEFAULT}"
	go vet ./...
	@echo "${GREEN} === DONE Checking === ${DEFAULT}"

go_correct:
	@echo "${YELLOW} = Correcting go code for formatting smells...${DEFAULT}"
	gofmt -s -w ./.
	@echo "${GREEN} === DONE Correcting === ${DEFAULT}"
