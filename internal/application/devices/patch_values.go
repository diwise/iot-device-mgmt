package devices

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var errInvalidPatch = errors.New("invalid patch")

func patchString(field string, value any) (string, error) {
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%w: field %q must be a string", errInvalidPatch, field)
	}

	return text, nil
}

func patchBool(field string, value any) (bool, error) {
	switch current := value.(type) {
	case bool:
		return current, nil
	case string:
		parsed, err := strconv.ParseBool(current)
		if err != nil {
			return false, fmt.Errorf("%w: field %q must be a boolean", errInvalidPatch, field)
		}
		return parsed, nil
	default:
		return false, fmt.Errorf("%w: field %q must be a boolean", errInvalidPatch, field)
	}
}

func patchFloat(field string, value any) (float64, error) {
	switch current := value.(type) {
	case float64:
		return current, nil
	case float32:
		return float64(current), nil
	case int:
		return float64(current), nil
	case int64:
		return float64(current), nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(current), 64)
		if err != nil {
			return 0, fmt.Errorf("%w: field %q must be numeric", errInvalidPatch, field)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("%w: field %q must be numeric", errInvalidPatch, field)
	}
}

func patchInt(field string, value any) (int, error) {
	switch current := value.(type) {
	case int:
		return current, nil
	case int64:
		return int(current), nil
	case float64:
		if current != float64(int(current)) {
			return 0, fmt.Errorf("%w: field %q must be an integer", errInvalidPatch, field)
		}
		return int(current), nil
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(current))
		if err != nil {
			return 0, fmt.Errorf("%w: field %q must be an integer", errInvalidPatch, field)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("%w: field %q must be an integer", errInvalidPatch, field)
	}
}

func patchStringSlice(field string, value any) ([]string, error) {
	switch current := value.(type) {
	case []string:
		return append([]string(nil), current...), nil
	case []any:
		result := make([]string, 0, len(current))
		for _, item := range current {
			text, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("%w: field %q must contain strings", errInvalidPatch, field)
			}
			result = append(result, text)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("%w: field %q must be a list of strings", errInvalidPatch, field)
	}
}
