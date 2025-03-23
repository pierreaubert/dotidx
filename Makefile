# simple build

all:
	go build cmd/dixbatch/dixbatch
	go build cmd/dixfe/dixfe
	go build cmd/dixlive/dixlive
	go build cmd/dixmgr/dixmgr

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

fmt:
	go fmt
	cd cmd/dixbatch && go fmt
	cd cmd/dixlive && go fmt
	cd cmd/dixfe && go fmt


