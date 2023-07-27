# Tunex

Tunex is a pure Golang alternative to https://serveo.net. It allows you to expose a local HTTP server to the internet without the need for any additional installations (only openSSH is required).

# Usage

## Client

Assuming you have a local HTTP server running on localhost:4000, you can use the following command to expose it to the internet:k

```
ssh -NR <name>:80:localhost:4000 server.host
```

## Server

To set up the server, follow these steps:

1. Clone this repository.
2. Run the following command

```
make dev
```
