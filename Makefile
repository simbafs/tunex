tcpipfw:
	nodemon -e go --watch './**/*.go' --signal SIGTERM --exec 'go' run ./tcpipfw

http:
	nodemon -e go --watch './**/*.go' --signal SIGTERM --exec 'go' run ./http

dev: 
	nodemon -e go --watch './**/*.go' --signal SIGTERM --exec 'go' run .
.PHONY: dev tcpipfw http
