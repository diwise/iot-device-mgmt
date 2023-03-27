package service

import (
	"encoding/json"
)

func MapTo[TTo any](v any) (TTo, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return *new(TTo), err
	}
	to := new(TTo)
	err = json.Unmarshal(b, to)
	if err != nil {
		return *new(TTo), err
	}
	return *to, nil
}
