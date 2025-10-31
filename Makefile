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

cron:
	cd cmd/dixcron && go vet
	cd cmd/dixcron && go fmt
	go build -o bin/dixcron cmd/dixcron/dixcron.go

batch:
	cd cmd/dixbatch && go vet
	cd cmd/dixbatch && go fmt
	go build -o bin/dixbatch cmd/dixbatch/dixbatch.go

watcher:
	cd cmd/dixwatcher && go vet
	cd cmd/dixwatcher && go fmt
	go build -o bin/dixwatcher ./cmd/dixwatcher

cli:
	go build -o bin/filter_cli cmd/filter_cli/filter_cli.go
	go build -o bin/block_cli cmd/block_cli/block_cli.go

bin: fe mgr cli live watcher cron batch

clean:
	./scripts/git_cleanup.sh
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

.PHONY: app

app:
	npm run build-sass
	cp app/dix-large.css app/dix.css
	npm run build

update_fe: fe app
	./scripts/dev/update_fe.sh



