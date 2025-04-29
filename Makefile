# simple build

all: fmt vet bin app

fe:
	cd cmd/dixfe && go vet
	cd cmd/dixfe && go fmt
	go build -o bin/dixfe cmd/dixfe/dixfe.go cmd/dixfe/r_*.go

live:
	cd cmd/dixlive && go vet
	cd cmd/dixlive && go fmt
	go build -o bin/dixlive cmd/dixlive/dixlive.go

mgr:
	cd cmd/dixmgr && go vet
	cd cmd/dixmgr && go fmt
	go build -o bin/dixmgr cmd/dixmgr/dixmgr.go

cli:
	go build -o bin/filter_cli cmd/filter_cli/filter_cli.go
	go build -o bin/address_cli cmd/address_cli/address_cli.go
	go build -o bin/block_cli cmd/block_cli/block_cli.go

bin: fe mgr cli live
	go build -o bin/dixbatch cmd/dixbatch/dixbatch.go
	go build -o bin/dixcron cmd/dixcron/dixcron.go

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
	cd cmd/block_cli && go vet

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
	cd cmd/block_cli && go fmt

app:
	npm run build-bulma

update_fe: fe app
	./scripts/update_fe_dev.sh



