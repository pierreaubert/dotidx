# simple build

all:
	go build
	cd cmd/dixbatch && go build
	cd cmd/dixlive && go build
	cd cmd/dixfe && go build

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


