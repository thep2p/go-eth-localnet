package model

import (
	"encoding/json"
	"fmt"
)

type HexInt int

// UnmarshalJSON parses a JSON-encoded string representing a hexadecimal number and stores the result in the HexInt.
func (h *HexInt) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	_, err := fmt.Sscanf(s, "0x%x", (*int)(h))
	return err
}
