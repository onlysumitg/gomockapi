package jsonutils

import (
	"encoding/json"
	"fmt"
)

// JsonToMapPlaceholder replaces every leaf value of the JSON with the
// token {{FLATKEY}}, using the same keys as JsonToFlatMap.
// The top level may be an object or an array.
func JsonToMapPlaceholder(stringJson string) (any, error) {
	var parsedJson any

	err := json.Unmarshal([]byte(stringJson), &parsedJson)
	if err != nil {
		return nil, err
	}

	return setPlaceholders(parsedJson, ""), nil
}

// -----------------------------------------------------------
//
// -----------------------------------------------------------
func setPlaceholders(value any, keyChain string) any {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			childKey := key
			if keyChain != "" {
				childKey = fmt.Sprintf("%s.%s", keyChain, key)
			}
			v[key] = setPlaceholders(child, childKey)
		}
		return v

	case []any:
		for i, child := range v {
			childKey := fmt.Sprintf("%s[%d]", keyChain, i) // "[0]" at the top level
			v[i] = setPlaceholders(child, childKey)
		}
		return v

	default:
		return fmt.Sprintf("{{%s}}", keyChain)
	}
}
