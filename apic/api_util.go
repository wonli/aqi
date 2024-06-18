package apic

import "encoding/json"

var marshal = func(a any) (string, error) {
	data, err := json.Marshal(a)
	if err != nil {
		return "", err
	}

	return string(data), nil
}
