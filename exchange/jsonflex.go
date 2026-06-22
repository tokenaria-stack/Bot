package exchange

import (
	"encoding/json"
	"strconv"
)

// flexString unmarshals JSON numbers or strings into a decimal string.
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		*f = ""
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*f = flexString(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	*f = flexString(n.String())
	return nil
}

func (f flexString) Float64() (float64, error) {
	if f == "" {
		return 0, strconv.ErrSyntax
	}
	return strconv.ParseFloat(string(f), 64)
}
