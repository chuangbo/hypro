GOBIN = hypro hypro-server
SERVER_NAME = localhost
PROTOS = protos/hypro.pb.go
GOFILES=$(wildcard *.go) $(wildcard */*.go)  $(wildcard cmd/*/*.go)

all: $(GOBIN)

hypro: $(PROTOS) ${GOFILES}
	go build ./cmd/hypro

hypro-server: $(PROTOS) ${GOFILES}
	go build ./cmd/hypro-server

protos: $(PROTOS)

protos/%.pb.go:protos/%.proto
	protoc -I protos/ $< --go_out=plugins=grpc:protos

clean:
	rm $(GOBIN)

certs:
	# Generate CA key:
	@openssl ecparam -name prime256v1 -noout -genkey -out certs/ca.key

	# Generate CA certificate:
	@openssl req -new -x509 -days 3650 -key certs/ca.key -out certs/ca.crt -subj "/C=NZ/ST=AKL/L=Auckland/O=HyproCompany/OU=HyproApp/CN=HyproRootCA"

	# Generate server key:
	@openssl ecparam -name prime256v1 -noout -genkey -out certs/server.key

	# Generate server signing request:
	@openssl req -new -key certs/server.key -out certs/server.csr -subj "/C=NZ/ST=AKL/L=Auckland/O=HyproCompany/OU=HyproApp/CN=$(SERVER_NAME)"

	# Self-sign server certificate:
	@openssl x509 -req -days 365 -in certs/server.csr -CA certs/ca.crt -CAkey certs/ca.key -set_serial 01 -out certs/server.crt

.PHONY: certs protos
