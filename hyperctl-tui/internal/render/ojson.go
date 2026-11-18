package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Obj is a JSON object that remembers key order, so generic tables keep the
// column order the compositor chose.
type Obj struct {
	Keys []string
	Vals map[string]any
}

// Decode parses JSON into nil/bool/json.Number/string/[]any/*Obj.
func Decode(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := decodeValue(dec)
	if err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, fmt.Errorf("trailing data after JSON value")
	}
	return v, nil
}

func decodeValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			o := &Obj{Vals: map[string]any{}}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				k := kt.(string)
				v, err := decodeValue(dec)
				if err != nil {
					return nil, err
				}
				if _, dup := o.Vals[k]; !dup {
					o.Keys = append(o.Keys, k)
				}
				o.Vals[k] = v
			}
			_, err := dec.Token()
			return o, err
		case '[':
			arr := []any{}
			for dec.More() {
				v, err := decodeValue(dec)
				if err != nil {
					return nil, err
				}
				arr = append(arr, v)
			}
			_, err := dec.Token()
			return arr, err
		}
		return nil, fmt.Errorf("unexpected delimiter %v", t)
	default:
		return t, nil
	}
}

// Get returns a value by key, or nil. Safe on a nil receiver.
func (o *Obj) Get(k string) any {
	if o == nil {
		return nil
	}
	return o.Vals[k]
}

// Str formats a scalar value; nested values become compact JSON-ish text.
func Str(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case json.Number:
		return trimFloat(t.String())
	case []any:
		parts := make([]string, len(t))
		for i, e := range t {
			parts[i] = Str(e)
		}
		return strings.Join(parts, ", ")
	case *Obj:
		parts := make([]string, 0, len(t.Keys))
		for _, k := range t.Keys {
			parts = append(parts, k+"="+Str(t.Vals[k]))
		}
		return "{" + strings.Join(parts, " ") + "}"
	}
	return fmt.Sprint(v)
}

// trimFloat turns "60.00000" into "60" and "1.25000" into "1.25".
func trimFloat(s string) string {
	if !strings.Contains(s, ".") || strings.ContainsAny(s, "eE") {
		return s
	}
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, ".")
}

func (o *Obj) S(k string) string { return Str(o.Get(k)) }

func (o *Obj) B(k string) bool {
	switch t := o.Get(k).(type) {
	case bool:
		return t
	case string:
		return t == "true"
	}
	return false
}

func (o *Obj) N(k string) float64 {
	if n, ok := o.Get(k).(json.Number); ok {
		f, _ := n.Float64()
		return f
	}
	return 0
}

func (o *Obj) O(k string) *Obj {
	v, _ := o.Get(k).(*Obj)
	return v
}

func (o *Obj) A(k string) []any {
	v, _ := o.Get(k).([]any)
	return v
}

func isScalar(v any) bool {
	switch v.(type) {
	case *Obj, []any:
		return false
	}
	return true
}
