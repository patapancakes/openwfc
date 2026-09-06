package gamespy

import (
	"slices"
	"strings"
)

type KeyValues []KV

type KV struct {
	K, V string
}

func KeyValuesFromString(s string) KeyValues {
	s = strings.TrimPrefix(s, `\`)

	split := strings.Split(s, `\`)
	if len(split)%2 != 0 {
		return nil
	}

	var kv KeyValues
	for split := range slices.Chunk(split, 2) {
		kv.Set(split[0], split[1])
	}

	return kv
}

func (kvs KeyValues) Get(k string) string {
	for _, kv := range kvs {
		if kv.K != k {
			continue
		}

		return kv.V
	}

	return ""
}

func (kvs KeyValues) Has(k string) bool {
	for _, kv := range kvs {
		if kv.K != k {
			continue
		}

		return true
	}

	return false
}

func (kvs *KeyValues) Set(k string, v string) {
	i := -1
	for vi, kv := range *kvs {
		if kv.K != k {
			continue
		}

		i = vi
		break
	}

	// key doesn't exist
	if i == -1 {
		*kvs = append(*kvs, KV{K: k, V: v})
		return
	}

	(*kvs)[i].V = v
}

func (kvs KeyValues) Encode() string {
	var s strings.Builder
	for _, kv := range kvs {
		s.WriteString(`\`)
		s.WriteString(kv.K)
		s.WriteString(`\`)
		s.WriteString(kv.V)
	}

	return s.String()
}
