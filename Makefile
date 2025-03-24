# simple build

all: fmt vet bin

bin:
	go build cmd/dixbatch/dixbatch.go
	go build cmd/dixfe/dixfe.go cmd/dixfe/r_*.go
	go build cmd/dixlive/dixlive.go
	go build cmd/dixmgr/dixmgr.go
	go build cmd/dixcron/dixcron.go

clean:
	./scripts/cleanup.sh
	go clean

test:
	go test -v

vet:
	go vet
	cd cmd/dixbatch && go vet
	cd cmd/dixlive && go vet
	cd cmd/dixfe && go vet
	cd cmd/dixcron && go vet

fmt:
	go fmt
	cd cmd/dixbatch && go fmt
	cd cmd/dixlive && go fmt
	cd cmd/dixfe && go fmt
	cd cmd/dixcron && go fmt


