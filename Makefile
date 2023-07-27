dev: 
	nodemon -e go --watch './**/*.go' --signal SIGTERM --exec 'go' run ./cmd/tunex.go

.PHONY: dev
