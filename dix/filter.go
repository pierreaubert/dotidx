package dix

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/tidwall/gjson"
)

type Filter interface {
	ProcessGJson(extrinsics json.RawMessage) (filtered json.RawMessage, err error)
	Process(extrinsics json.RawMessage) (filtered json.RawMessage, found bool, err error)
}

type EventsBalance struct {
	address string
}

func NewEventsBalance(address string) *EventsBalance {
	return &EventsBalance{
		address: address,
	}
}

func (eb *EventsBalance) ProcessGJson(extrinsics json.RawMessage) (filtered json.RawMessage, err error) {
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

func (es *EventsStaking) ProcessGJson(extrinsics json.RawMessage) (filtered json.RawMessage, err error) {

	// in pallet staking takes all events that match address and balances
	patternStaking := fmt.Sprintf(
		`extrinsics.#(method.pallet=="staking").events.#(data.#(%%"%s"))#`,
		es.address)

	// in pallet utility takes all events that match address and staking
	patternUtility := fmt.Sprintf(
		`extrinsics.#(method.pallet=="utility").events.#(data.#(%%"%s"))#|#(method.pallet=="staking")#`,
		es.address)

	// expensive ...
	sextrinsics := fmt.Sprintf(`{"extrinsics": %s}`, string(extrinsics))

	// log.Printf("%s", sextrinsics)
	// log.Printf("%d %s", len(sextrinsics), patternBalances)

	staking := gjson.Get(sextrinsics, patternStaking)
	resultsStaking := staking.String()
	if resultsStaking == "" || resultsStaking == "[]" {
		resultsStaking = "[]"
	}

	utility := gjson.Get(sextrinsics, patternUtility)
	resultsUtility := utility.String()
	if resultsUtility == "" || resultsUtility == "[]" {
		resultsUtility = "[]"
	}
	results := fmt.Sprintf(`{"staking": %s, "utility": %s}`, resultsStaking, resultsUtility)
	// log.Printf("%s", results)
	return json.RawMessage(results), nil
}

type Matcher struct {
	Address string
	Method  string
	Pallet  string
}

func eventMatchMethodData(data interface{}, matcher Matcher) bool {
	switch v := data.(type) {
	case map[string]interface{}:
		if v["data"] == nil || v["method"] == nil {
			return false
		}
		method := v["method"]
		switch m := method.(type) {
		case map[string]interface{}:
			if (matcher.Method != "" && m["method"] != matcher.Method) ||
				(matcher.Pallet != "" && m["pallet"] != matcher.Pallet) {
				return false
			}
			data := v["data"]
			switch d := data.(type) {
			case []interface{}:
				for _, item := range d {
					switch adr := item.(type) {
					case string:
						a := string(adr)
						if matcher.Address == "" || a == matcher.Address {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

func eventMatchMethodTargetID(data interface{}, matcher Matcher) bool {
	switch v := data.(type) {
	case map[string]interface{}:
		if v["args"] == nil || v["method"] == nil {
			return false
		}
		method := v["method"]
		switch m := method.(type) {
		case map[string]interface{}:
			if (matcher.Method != "" && m["method"] != matcher.Method) || (matcher.Pallet != "" && m["pallet"] != matcher.Pallet) {
				return false
			}
			args := v["args"]
			switch d := args.(type) {
			case map[string]interface{}:
				target := d["target"]
				switch t := target.(type) {
				case map[string]interface{}:
					for key, value := range t {
						if key == "id" {
							switch adr := value.(type) {
							case string:
								a := string(adr)
								if matcher.Address == "" || matcher.Address == a {
									return true
								}
							}
						}
					}
				}
			}
		}
	}
	return false
}

func eventMatch(data interface{}, matcher Matcher) bool {
	switch matcher.Pallet {
	case "staking":
		return eventMatchMethodData(data, matcher)
	case "balances":
		return eventMatchMethodData(data, matcher)
	case "vesting":
		return eventMatchMethodTargetID(data, matcher)
	}
	return false
}

func EventProcess(data interface{}, matcher Matcher) (interface{}, bool, error) {

	switch v := data.(type) {
	case map[string]interface{}:
		w := make(map[string]interface{})
		found := false
		for key, value := range v {
			if eventMatch(value, matcher) {
				w[key] = value
				found = true
			} else {
				processed, foundDown, err := EventProcess(value, matcher)
				if err != nil {
					return nil, false, err
				}
				if foundDown {
					w[key] = processed
					found = true
				}
			}
		}
		return w, found, nil
	case []interface{}:
		var w []interface{}
		found := false
		for _, item := range v {
			if eventMatch(item, matcher) {
				w = append(w, item)
				found = true
			} else {
				processed, foundDown, err := EventProcess(item, matcher)
				if err != nil {
					return nil, false, err
				}
				if foundDown {
					w = append(w, processed)
					found = true
				}
			}
		}
		return w, found, nil
	default:
		return v, false, nil
	}
}

func (eb *EventsBalance) Process(extrinsics json.RawMessage) (filtered json.RawMessage, found bool, err error) {
	matcher := &Matcher{
		Address: eb.address,
		Method:  "",
		Pallet:  "balances",
	}

	var e []interface{}
	err = json.Unmarshal([]byte(extrinsics), &e)
	if err != nil {
		return nil, false, err
	}
	selection, found, err := EventProcess(e, *matcher)
	if err != nil {
		log.Printf("Error processing balances extrinsics: %v", err)
		return nil, false, err
	}

	filtered, err = json.MarshalIndent(selection, "", "  ")
	if err != nil {
		log.Printf("Error marshalling balances extrinsics: %v", err)
		return nil, false, err
	}
	// log.Printf("Debug: %v\n", string(filtered))
	return filtered, found, nil
}

func (es *EventsStaking) Process(extrinsics json.RawMessage) (filtered json.RawMessage, found bool, err error) {
	matcher := &Matcher{
		Address: es.address,
		Method:  "",
		Pallet:  "staking",
	}
	var e []interface{}
	err = json.Unmarshal([]byte(extrinsics), &e)
	if err != nil {
		return nil, false, err
	}
	selection, found, err := EventProcess(e, *matcher)
	if err != nil {
		log.Printf("Error processing staking extrinsics: %v", err)
		return nil, false, err
	}

	filtered, err = json.MarshalIndent(selection, "", "  ")
	if err != nil {
		log.Printf("Error marshalling staking extrinsics: %v", err)
		return nil, false, err
	}
	// log.Printf("Debug: %v\n", string(filtered))
	return filtered, found, nil
}
