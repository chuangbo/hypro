## 🧚 hypro

[![GoDoc](https://pkg.go.dev/badge/github.com/chuangbo/hypro?utm_source=godoc)](https://pkg.go.dev/github.com/chuangbo/hypro)
[![Go Report Card](https://goreportcard.com/badge/github.com/chuangbo/hypro)](https://goreportcard.com/report/github.com/chuangbo/hypro)

*HYpertext-transfer-protocol PROxy*

Hypro is a simple HTTP tunnel powered by go and gRPC.

```sh
 HTTP Server
     |
Reverse Proxy (Server)
     |
 gRPC Stream
     |
Reverse Proxy (Client)
     |
   Target
```

### Install

```sh
# Server
go install github.com/chuangbo/hypro/cmd/hypro-server
# Client
go install github.com/chuangbo/hypro/cmd/hypro
```

### Usage

1. DNS

    Add a DNS A record to your server:

    `example.com => YOUR_SERVER_IP`

    And then add a wildcard DNS A record to your server:

    `*.example.com => YOUR_SERVER_IP`

1. Server

    ```
    hypro-server
    ```

1. Client

    ```
    hypro -server example.com -insecure -domain myapp.example.com -target http://localhost:8080
    ```

### Secure Connection

1. Create self-sign certificate

    ```sh
    # Generate CA key:
    openssl ecparam -name prime256v1 -noout -genkey -out ca.key

    # Generate CA certificate:
    openssl req -new -x509 -days 3650 -key ca.key -out ca.crt -subj "/C=NZ/ST=AKL/L=Auckland/O=HyproCompany/OU=HyproApp/CN=HyproRootCA"

    # Generate server key:
    openssl ecparam -name prime256v1 -noout -genkey -out server.key

    # Generate server signing request:
    openssl req -new -key server.key -out server.csr -subj "/C=NZ/ST=AKL/L=Auckland/O=HyproCompany/OU=HyproApp/CN=$(SERVER_NAME)"

    # Self-sign server certificate:
    openssl x509 -req -days 365 -in server.csr -CA ca.crt -CAkey ca.key -set_serial 01 -out server.crt
    ```

1. Server

    ```sh
    hypro-server -cert server.crt -key server.key
    ```

1. Client

    ```sh
    hypro -server example.com -cert server.crt -domain myapp.example.com -target http://localhost:8080
    ```

### Documentation

<https://godoc.org/github.com/chuangbo/hypro>

### Build from source

```sh
go install google.golang.org/protobuf/cmd/protoc-gen-go
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc
make
```

### License

[MIT](http://opensource.org/licenses/MIT)

Copyright (c) 2018-present, Chuangbo Li

### Todos

* Tests
* Benchmarks
* Reuse connections
* Reconnect to the server
* 12-factor
* Graceful reload
* Max connections
* Record / Redo requests
* Cluster
