package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type flexibleInt64 struct {
	Value int64
	Set   bool
}

func (value *flexibleInt64) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) || len(data) == 0 {
		*value = flexibleInt64{}
		return nil
	}

	var number json.Number
	if data[0] == '"' {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		if isNullSentinel(text) {
			*value = flexibleInt64{}
			return nil
		}
		parsed, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid integer string %q: %w", text, err)
		}
		value.Value = parsed
		value.Set = true
		return nil
	}

	if err := json.Unmarshal(data, &number); err != nil {
		return err
	}
	parsed, err := number.Int64()
	if err != nil {
		return fmt.Errorf("invalid integer %q: %w", number.String(), err)
	}
	value.Value = parsed
	value.Set = true
	return nil
}

type flexibleString struct {
	Value string
	Set   bool
}

func (value *flexibleString) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) || len(data) == 0 {
		*value = flexibleString{}
		return nil
	}

	if data[0] == '"' {
		if err := json.Unmarshal(data, &value.Value); err != nil {
			return err
		}
		value.Set = true
		return nil
	}

	var number json.Number
	if err := json.Unmarshal(data, &number); err != nil {
		return fmt.Errorf("expected string or number: %w", err)
	}
	if _, err := number.Float64(); err != nil {
		return fmt.Errorf("expected string or number: %w", err)
	}
	value.Value = number.String()
	value.Set = true
	return nil
}

type nullableString struct {
	Value string
	Set   bool
}

type oneOrMany[T any] []T

func (values *oneOrMany[T]) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*values = nil
		return nil
	}
	if data[0] == '[' {
		return json.Unmarshal(data, (*[]T)(values))
	}
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*values = []T{value}
	return nil
}

func (value *nullableString) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) || bytes.Equal(data, []byte("false")) || len(data) == 0 {
		*value = nullableString{}
		return nil
	}

	if err := json.Unmarshal(data, &value.Value); err != nil {
		return fmt.Errorf("expected string, null, or false: %w", err)
	}
	if isNullSentinel(value.Value) {
		*value = nullableString{}
		return nil
	}
	value.Set = true
	return nil
}

// isNullSentinel reports the spellings EasyDNS uses for "no value" in fields
// that are otherwise integers or strings. Observed in the sandbox: a domain
// with no subscription block returns sub_block "NONE", and one that is not
// cloned returns cloned_to "NONE".
func isNullSentinel(value string) bool {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "", "NONE", "NULL", "N/A":
		return true
	default:
		return false
	}
}

// flexibleBool decodes the several spellings EasyDNS uses for a boolean:
// true/false, 1/0, "1"/"0", and "Y"/"N". An absent or null value stays unset
// so callers can tell "false" apart from "not reported".
type flexibleBool struct {
	Value bool
	Set   bool
}

func (value *flexibleBool) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) || len(data) == 0 {
		*value = flexibleBool{}
		return nil
	}

	if bytes.Equal(data, []byte("true")) || bytes.Equal(data, []byte("false")) {
		var parsed bool
		if err := json.Unmarshal(data, &parsed); err != nil {
			return err
		}
		value.Value = parsed
		value.Set = true
		return nil
	}

	if data[0] == '"' {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		switch text {
		case "1", "Y", "y", "true", "TRUE", "yes", "YES":
			value.Value = true
		case "0", "N", "n", "false", "FALSE", "no", "NO", "":
			value.Value = false
		default:
			return fmt.Errorf("invalid boolean string %q", text)
		}
		value.Set = true
		return nil
	}

	var number json.Number
	if err := json.Unmarshal(data, &number); err != nil {
		return fmt.Errorf("expected boolean, string, or number: %w", err)
	}
	parsed, err := number.Int64()
	if err != nil {
		return fmt.Errorf("invalid boolean number %q: %w", number.String(), err)
	}
	value.Value = parsed != 0
	value.Set = true
	return nil
}
