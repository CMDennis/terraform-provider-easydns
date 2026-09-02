package client

import (
	"encoding/json"
	"testing"
)

func TestFlexibleInt64(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		value int64
		set   bool
		ok    bool
	}{
		{input: `123`, value: 123, set: true, ok: true},
		{input: `"456"`, value: 456, set: true, ok: true},
		{input: `0`, value: 0, set: true, ok: true},
		{input: `null`, value: 0, set: false, ok: true},
		{input: `"bad"`, ok: false},
		{input: `1.5`, ok: false},
	}

	for _, test := range tests {
		var value flexibleInt64
		err := json.Unmarshal([]byte(test.input), &value)
		if (err == nil) != test.ok {
			t.Errorf("input %s error=%v, ok=%v", test.input, err, test.ok)
			continue
		}
		if test.ok && (value.Value != test.value || value.Set != test.set) {
			t.Errorf("input %s got %+v", test.input, value)
		}
	}
}

func TestFlexibleString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		value string
		set   bool
		ok    bool
	}{
		{input: `"abc"`, value: "abc", set: true, ok: true},
		{input: `123`, value: "123", set: true, ok: true},
		{input: `null`, set: false, ok: true},
		{input: `false`, ok: false},
	}

	for _, test := range tests {
		var value flexibleString
		err := json.Unmarshal([]byte(test.input), &value)
		if (err == nil) != test.ok {
			t.Errorf("input %s error=%v, ok=%v", test.input, err, test.ok)
			continue
		}
		if test.ok && (value.Value != test.value || value.Set != test.set) {
			t.Errorf("input %s got %+v", test.input, value)
		}
	}
}

func TestNullableStringAcceptsFalse(t *testing.T) {
	t.Parallel()

	var value nullableString
	if err := json.Unmarshal([]byte(`false`), &value); err != nil {
		t.Fatalf("false: %v", err)
	}
	if value.Set || value.Value != "" {
		t.Fatalf("false decoded as %+v", value)
	}
	if err := json.Unmarshal([]byte(`"2030-01-01"`), &value); err != nil {
		t.Fatalf("string: %v", err)
	}
	if !value.Set || value.Value != "2030-01-01" {
		t.Fatalf("string decoded as %+v", value)
	}
}

func TestOneOrMany(t *testing.T) {
	t.Parallel()

	for _, input := range []string{`{"value":1}`, `[{"value":1},{"value":2}]`, `null`} {
		var values oneOrMany[struct {
			Value int `json:"value"`
		}]
		if err := json.Unmarshal([]byte(input), &values); err != nil {
			t.Fatalf("input=%s error=%v", input, err)
		}
		if input != "null" && len(values) == 0 {
			t.Fatalf("input=%s values=%v", input, values)
		}
	}
}
