package service

import (
	"encoding/json"
)

func MapTo[T any](v any) (T, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return *new(T), err
	}
	to := new(T)
	err = json.Unmarshal(b, to)
	if err != nil {
		return *new(T), err
	}
	return *to, nil
}

func MapToArr[T any](arr []any) ([]T, error) {
	result := *new([]T)

	for _, v := range arr {
		to, err := MapTo[T](v)
		if err != nil {
			return *new([]T), err
		}
		result = append(result, to)
	}

	return result, nil
}
