# simple build

all: fmt vet bin app

fe:
	cd cmd/dixfe && go vet
	cd cmd/dixfe && go fmt
	go build cmd/dixfe/dixfe.go cmd/dixfe/r_*.go

mgr:
	cd cmd/dixmgr && go vet
	cd cmd/dixmgr && go fmt
	go build cmd/dixmgr/dixmgr.go

cli:
	go build cmd/filter_cli/filter_cli.go
	go build cmd/address_cli/address_cli.go

bin: fe mgr cli
	go build cmd/dixbatch/dixbatch.go
	go build cmd/dixlive/dixlive.go
	go build cmd/dixmgr/dixmgr.go
	go build cmd/dixcron/dixcron.go

clean:
	./scripts/cleanup.sh
	go clean

test:
	go test -v
	npm run test

vet:
	go vet
	cd cmd/dixbatch && go vet
	cd cmd/dixlive && go vet
	cd cmd/dixcron && go vet
	cd cmd/dixfe && go vet
	npm run oxlint
	cd cmd/filter_cli && go vet
	cd cmd/address_cli && go vet

fmt:
	go fmt
	cd cmd/dixbatch && go fmt
	cd cmd/dixfe && go fmt
	cd cmd/dixlive && go fmt
	cd cmd/dixcron && go fmt
	cd cmd/dixmgr && go fmt
	npm run format
	cd cmd/filter_cli && go fmt
	cd cmd/address_cli && go fmt

app:
	npm run build-bulma

update_fe: fe app
	./scripts/update_fe_dev.sh



