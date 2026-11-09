package beers

import (
	"bytes"
	"encoding/json"
)

// Optional is a PATCH field that tells "absent" from "null" (research R2): Set is false when
// the field was not sent, and Value is nil when it was sent as null. encoding/json calls
// UnmarshalJSON only for fields present in the body, including explicit nulls.
type Optional[T any] struct {
	Set   bool
	Value *T
}

// UnmarshalJSON marks o set and decodes data into Value, leaving Value nil for null.
func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	o.Set = true
	if bytes.Equal(data, []byte("null")) {
		o.Value = nil
		return nil
	}
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	o.Value = &v
	return nil
}
