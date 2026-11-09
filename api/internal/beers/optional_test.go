package beers

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestOptional(t *testing.T) {
	type body struct {
		X Optional[int] `json:"x"`
	}
	decode := func(s string) (body, error) {
		var b body
		dec := json.NewDecoder(bytes.NewReader([]byte(s)))
		dec.DisallowUnknownFields()
		return b, dec.Decode(&b)
	}

	b, err := decode(`{}`)
	if err != nil || b.X.Set || b.X.Value != nil {
		t.Fatalf("{} = %+v (err %v), want unset", b.X, err)
	}
	b, err = decode(`{"x": null}`)
	if err != nil || !b.X.Set || b.X.Value != nil {
		t.Fatalf("null = %+v (err %v), want set with nil value", b.X, err)
	}
	b, err = decode(`{"x": 5}`)
	if err != nil || !b.X.Set || b.X.Value == nil || *b.X.Value != 5 {
		t.Fatalf("5 = %+v (err %v), want set to 5", b.X, err)
	}
	if _, err := decode(`{"x": "five"}`); err == nil {
		t.Fatal("wrong type decoded without error")
	}
	if _, err := decode(`{"x": 5, "y": 1}`); err == nil {
		t.Fatal("unknown field decoded without error")
	}
}
