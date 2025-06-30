package utils

import "encoding/hex"

func ByteToHex(b []byte) string {
	return "0x" + hex.EncodeToString(b)
}
