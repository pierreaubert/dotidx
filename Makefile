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
