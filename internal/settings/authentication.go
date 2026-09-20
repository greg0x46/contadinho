package settings

import (
	"context"
	"fmt"
)

const KeyAuthenticationEnabled = "authentication.enabled"

// AuthenticationEnabled defaults to the secure behavior for databases that
// predate the preference. Invalid persisted values are errors so callers can
// fail closed instead of accidentally opening the API.
func AuthenticationEnabled(ctx context.Context, q Querier) (bool, error) {
	value, ok, err := Get(ctx, q, KeyAuthenticationEnabled, nil)
	if err != nil {
		return false, err
	}
	if !ok {
		return true, nil
	}
	switch value {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("valor inválido para %s", KeyAuthenticationEnabled)
	}
}

func SetAuthenticationEnabled(ctx context.Context, q Querier, enabled bool) error {
	value := "false"
	if enabled {
		value = "true"
	}
	return Set(ctx, q, KeyAuthenticationEnabled, value, false, nil)
}
