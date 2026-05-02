package utils

import (
	"encoding/base64"

	"github.com/cespare/xxhash/v2"
)

// Xxh64 return a base64-encoded checksum of a resource using Xxh64 algorithm
//
// Encoded using Base64 URLSafe
func HashXxh64(content []byte) string {
	h := xxhash.New()
	h.Write(content)
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
