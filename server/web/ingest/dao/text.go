package dao

import "encoding/json"

func marshalLabels(in map[string]string) (any, error) {
	if len(in) == 0 {
		return nil, nil
	}
	body, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	return string(body), nil
}

func unmarshalLabels(text string) map[string]string {
	if text == "" {
		return nil
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return nil
	}
	return out
}

func nullText(value string) any {
	if value == "" {
		return nil
	}
	return value
}
