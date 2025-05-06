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
	go vet ./...
	npm run oxlint

fmt:
	go fmt ./...
	npm run format

app/dix-large.css: app/dix.scss
	npm run build-bulma-step1

app/dix.css : app/dix-large.css
	npm run build-bulma-step2

app: app/dix.css
	npm run build

update_fe: fe app
	./scripts/update_fe_dev.sh



