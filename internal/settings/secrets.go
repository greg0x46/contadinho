package settings

import "bytes"

// Secrets holds the immutable server encryption key, independent of browser sessions.
// Production validates this key against every encrypted setting before starting HTTP or workers.
type Secrets struct{ key []byte }

func NewSecrets(key []byte) *Secrets   { return &Secrets{key: bytes.Clone(key)} }
func (s *Secrets) Key() ([]byte, bool) { return bytes.Clone(s.key), len(s.key) == 32 }
