package http

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type jsonInt64 int64

func (v *jsonInt64) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	if text == "" || text == "null" {
		*v = 0
		return nil
	}
	if strings.HasPrefix(text, `"`) {
		var raw string
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
		text = strings.TrimSpace(raw)
	}
	if text == "" {
		*v = 0
		return nil
	}
	value, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid int64 value %q: %w", text, err)
	}
	*v = jsonInt64(value)
	return nil
}

func (v jsonInt64) Int64() int64 {
	return int64(v)
}
