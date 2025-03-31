package dotidx

import (
	"encoding/json"
	"fmt"
	"log"

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
		`extrinsics.#(method.pallet=="utility").events.#(data.#(%%"%s"))#|#(method.pallet=="balances")#`,
		eb.address)
	patternMultisig := fmt.Sprintf(
		`extrinsics.#(method.pallet=="multisig").events.#(data.#(%%"%s"))#|#(method.pallet=="balances")#`,
		eb.address)
	patternStaking := fmt.Sprintf(
		`extrinsics.#(method.pallet=="staking").events.#(data.#(%%"%s"))#|#(method.pallet=="balances")#`,
		eb.address)

	// add transactionPayment ?

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
	resultsMultisig := gjson.Get(sextrinsics, patternMultisig).String()
	if resultsMultisig == "" {
		resultsMultisig = "[]"
	}
	resultsStaking := gjson.Get(sextrinsics, patternStaking).String()
	if resultsStaking == "" {
		resultsStaking = "[]"
	}

	results := fmt.Sprintf(`{"balances": %s, "utility": %s, "multisig": %s, "staking": %s}`,
		resultsBalances, resultsUtility, resultsMultisig, resultsStaking,
	)

	log.Printf("%s", results)

	return json.RawMessage(results), nil
}

type EventsStaking struct {
	address string
}

func NewEventsStaking(address string) *EventsStaking {
	return &EventsStaking{
		address: address,
	}
}

func (es *EventsStaking) Process(extrinsics json.RawMessage) (filtered json.RawMessage, err error) {

	// in pallet staking takes all events that match address and balances
	patternStaking := fmt.Sprintf(
		`extrinsics.#(method.pallet=="staking").events.#(data.#(%%"%s"))#`,
		es.address)

	// in pallet utility takes all events that match address and staking
	patternUtility := fmt.Sprintf(
		`extrinsics.#(method.pallet="utility").events.#(data.#(%%"%s"))#|#(method.pallet=="staking")#`,
		es.address)

	// expensive ...
	sextrinsics := fmt.Sprintf(`{"extrinsics": %s}`, string(extrinsics))

	// log.Printf("%s", sextrinsics)
	// log.Printf("%d %s", len(sextrinsics), patternBalances)

	resultsStaking := gjson.Get(sextrinsics, patternStaking).String()
	if resultsStaking == "" {
		resultsStaking = "[]"
	}

	resultsUtility := gjson.Get(sextrinsics, patternUtility).String()
	if resultsUtility == "" {
		resultsUtility = "[]"
	}

	results := fmt.Sprintf(`{"staking": %s, "utility": %s}`, resultsStaking, resultsUtility)

	// log.Printf("%s", results)

	return json.RawMessage(results), nil
}
