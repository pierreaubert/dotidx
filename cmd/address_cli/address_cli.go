package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/pierreaubert/dotidx/dix"
)

func main() {
	address := flag.String("address", "", "a Polkadot address")
	flag.Parse()

	if address == nil || *address == "" {
		log.Fatal("Please provide an address")
	}

	fmt.Printf("Address: %s\n", *address)
	fmt.Printf("Base58: %s\n", dix.Base58Decode(*address))
	polkadot := dix.SS58Decode(*address, 42)

	fmt.Printf("%x\n", polkadot)
}
