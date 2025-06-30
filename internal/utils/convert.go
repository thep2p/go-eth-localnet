package utils

import (
	"encoding/hex"
	"fmt"
)

// ByteToHex converts a byte slice to a hexadecimal string prefixed with "0x".
func ByteToHex(b []byte) string {
	return "0x" + hex.EncodeToString(b)
}

// LocalAddress returns a string representing the local address for a given port.
func LocalAddress(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}
