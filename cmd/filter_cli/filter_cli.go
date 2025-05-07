package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/pierreaubert/dotidx/dix"
)

func readJSONFile(filePath string) (map[string]interface{}, error) {
	// Read file content
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	// Parse JSON
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	if err != nil {
		return nil, fmt.Errorf("error parsing JSON: %w", err)
	}

	return result, nil
}

func main() {
	inputFile := flag.String("input", "", "input File")
	address := flag.String("address", "", "a Polkadot address")
	method := flag.String("method", "", "a Polkadot runtime pallet method")
	pallet := flag.String("pallet", "", "a Polkadot runtime pallet name")

	flag.Parse()

	matcher := &dix.Matcher{
		Address: *address,
		Method:  *method,
		Pallet:  *pallet,
	}

	log.Printf("Matching %s in %s:%s", matcher.Address, matcher.Pallet, matcher.Method)

	data, err := readJSONFile(*inputFile)
	if err != nil {
		log.Fatalf("can't open %s", *inputFile)
	}

	filtered, found, err := dix.EventProcess(data, *matcher)
	if err != nil {
		log.Fatalf("can't process")
	}
	if found {
		fmt.Print("We did find matches")
	}

	output, err := json.MarshalIndent(filtered, "", "  ")
	if err != nil {
		log.Fatalf("Can't marshal: %v\n", err)
	}

	fmt.Println(string(output))
}
