package keeper

// Store is the single lean bonded-set KV. Do not read x/staking LastValidatorPowers.
type Store interface {
	Get(key []byte) []byte
	Set(key, value []byte)
	Delete(key []byte)
	// Iterate prefix; stop if fn returns false.
	IteratePrefix(prefix []byte, fn func(key, value []byte) bool)
}

type memStore struct {
	m map[string][]byte
}

func NewMemStore() Store {
	return &memStore{m: make(map[string][]byte)}
}

func (s *memStore) Get(key []byte) []byte {
	v, ok := s.m[string(key)]
	if !ok {
		return nil
	}
	out := make([]byte, len(v))
	copy(out, v)
	return out
}

func (s *memStore) Set(key, value []byte) {
	cp := make([]byte, len(value))
	copy(cp, value)
	s.m[string(key)] = cp
}

func (s *memStore) Delete(key []byte) {
	delete(s.m, string(key))
}

func (s *memStore) IteratePrefix(prefix []byte, fn func(key, value []byte) bool) {
	for k, v := range s.m {
		if len(k) >= len(prefix) && k[:len(prefix)] == string(prefix) {
			if !fn([]byte(k), v) {
				return
			}
		}
	}
}
