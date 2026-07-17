package marketplace

import (
	"encoding/json"
	"fmt"
)

func SetRawField(document map[string]json.RawMessage, name string, value interface{}) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	document[name] = raw
	return nil
}

func NestedRawObject(document map[string]json.RawMessage, name string) (map[string]json.RawMessage, error) {
	var nested map[string]json.RawMessage
	if raw, ok := document[name]; ok && string(raw) != "null" {
		if err := json.Unmarshal(raw, &nested); err != nil {
			return nil, fmt.Errorf("subscription field %q is not an object: %w", name, err)
		}
	}
	if nested == nil {
		nested = make(map[string]json.RawMessage)
	}
	return nested, nil
}

func StoreNestedRawObject(document map[string]json.RawMessage, name string, nested map[string]json.RawMessage) error {
	raw, err := json.Marshal(nested)
	if err != nil {
		return err
	}
	document[name] = raw
	return nil
}
