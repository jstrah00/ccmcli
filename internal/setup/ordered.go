package setup

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// obj es un objeto JSON que conserva el orden de sus claves: settings.json es
// del usuario y reescribirlo con las claves ordenadas le ensucia el diff.
type obj []kv

type kv struct {
	K string
	V any
}

func (o obj) get(k string) (any, bool) {
	for _, e := range o {
		if e.K == k {
			return e.V, true
		}
	}
	return nil, false
}

func (o *obj) set(k string, v any) {
	for i, e := range *o {
		if e.K == k {
			(*o)[i].V = v
			return
		}
	}
	*o = append(*o, kv{k, v})
}

func parse(b []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	v, err := decodeValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err == nil {
		return nil, errors.New("trailing data after JSON value")
	}
	return v, nil
}

func decodeValue(dec *json.Decoder) (any, error) {
	t, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := t.(type) {
	case json.Delim:
		switch t {
		case '{':
			o := obj{}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				v, err := decodeValue(dec)
				if err != nil {
					return nil, err
				}
				o = append(o, kv{kt.(string), v})
			}
			_, err := dec.Token()
			return o, err
		case '[':
			a := []any{}
			for dec.More() {
				v, err := decodeValue(dec)
				if err != nil {
					return nil, err
				}
				a = append(a, v)
			}
			_, err := dec.Token()
			return a, err
		}
		return nil, fmt.Errorf("unexpected %v", t)
	default:
		return t, nil
	}
}

func encode(v any) ([]byte, error) {
	var compact bytes.Buffer
	if err := encodeValue(&compact, v); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := json.Indent(&out, compact.Bytes(), "", "  "); err != nil {
		return nil, err
	}
	out.WriteByte('\n')
	return out.Bytes(), nil
}

func encodeValue(b *bytes.Buffer, v any) error {
	switch v := v.(type) {
	case obj:
		b.WriteByte('{')
		for i, e := range v {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := scalar(b, e.K); err != nil {
				return err
			}
			b.WriteByte(':')
			if err := encodeValue(b, e.V); err != nil {
				return err
			}
		}
		b.WriteByte('}')
	case []any:
		b.WriteByte('[')
		for i, e := range v {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := encodeValue(b, e); err != nil {
				return err
			}
		}
		b.WriteByte(']')
	default:
		return scalar(b, v)
	}
	return nil
}

// scalar sin escapar HTML: los hooks suelen tener && y > en los comandos.
func scalar(b *bytes.Buffer, v any) error {
	enc := json.NewEncoder(b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return err
	}
	b.Truncate(b.Len() - 1)
	return nil
}
