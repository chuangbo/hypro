## 🧚 hypro
[![GoDoc](https://godoc.org/github.com/chuangbo/hypro?status.svg)](https://godoc.org/github.com/chuangbo/hypro)

*HYpertext-transfer-protocol PROxy*

Hypro is a simple HTTP tunnel powered by go and gRPC.

```
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

```
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

    ```
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

    ```
    hypro-server -cert server.crt -key server.key
    ```

1. Client

    ```
    hypro -server example.com -cert server.crt -domain myapp.example.com -target http://localhost:8080
    ```

### Documentation

https://godoc.org/github.com/chuangbo/hypro


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
