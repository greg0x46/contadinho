package settings

import "context"

// KeyBrapiToken stores the optional brapi.dev API token the brapi provider
// in internal/marketdata sends as an Authorization: Bearer header. Encrypted like
// the Pluggy credentials it sits next to in Configurações.
const KeyBrapiToken = "quotes.brapi_token"

// GetBrapiToken reads the brapi token, decrypting it with unlockKey. ok is
// false when no token has ever been saved — internal/quotes treats that the
// same as an empty token, sending requests unauthenticated.
func GetBrapiToken(ctx context.Context, q Querier, unlockKey []byte) (token string, ok bool, err error) {
	return Get(ctx, q, KeyBrapiToken, unlockKey)
}

// SetBrapiToken stores the brapi token encrypted under unlockKey. An empty
// token is a valid value to store: it is how a previously configured token
// gets cleared.
func SetBrapiToken(ctx context.Context, q Querier, token string, unlockKey []byte) error {
	return Set(ctx, q, KeyBrapiToken, token, true, unlockKey)
}
