package dotidx

import (
	"encoding/json"
	"fmt"
	// "log"

	"github.com/tidwall/gjson"
)

type Filter interface {
	Process(extrinsics json.RawMessage) (filtered json.RawMessage, err error)
}

type EventsBalance struct {
	address string
}

func NewEventsBalance(address string) *EventsBalance {
	return &EventsBalance{
		address: address,
	}
}

func (eb *EventsBalance) Process(extrinsics json.RawMessage) (filtered json.RawMessage, err error) {
	patternBalances := fmt.Sprintf(
		`extrinsics.#(method.pallet=="balances").events.#(data.#(%%"%s"))#`,
		eb.address)
	patternUtility := fmt.Sprintf(
		`extrinsics.#(method.pallet=="utility").events.#(data.#(%%"%s"))#`,
		eb.address)
	patternStaking := fmt.Sprintf(
		`extrinsics.#(method.pallet=="staking").events.#(data.#(%%"%s"))#|#(method.pallet=="balances")#`,
		eb.address)

	// expensive ...
	sextrinsics := fmt.Sprintf(`{"extrinsics": %s}`, string(extrinsics))

	// log.Printf("%s", sextrinsics)
	// log.Printf("%d %s", len(sextrinsics), patternBalances)

	resultsBalances := gjson.Get(sextrinsics, patternBalances).String()
	if resultsBalances == "" {
		resultsBalances = "[]"
	}
	resultsUtility := gjson.Get(sextrinsics, patternUtility).String()
	if resultsUtility == "" {
		resultsUtility = "[]"
	}
	resultsStaking := gjson.Get(sextrinsics, patternStaking).String()
	if resultsStaking == "" {
		resultsStaking = "[]"
	}

	results := fmt.Sprintf(`{"balances": %s, "utility": %s, "staking": %s}`,
		resultsBalances, resultsUtility, resultsStaking,
	)

	// log.Printf("%s", results)

	return json.RawMessage(results), nil
}
