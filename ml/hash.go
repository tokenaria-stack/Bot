package ml

import (
	"encoding/binary"
	"hash"
	"math"
)

func hashPutU32(h hash.Hash, v uint32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	_, _ = h.Write(b[:])
}

func hashPutU64(h hash.Hash, v uint64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], v)
	_, _ = h.Write(b[:])
}

func hashPutBytes(h hash.Hash, p []byte) {
	hashPutU32(h, uint32(len(p)))
	_, _ = h.Write(p)
}

func hashPutString(h hash.Hash, s string) {
	hashPutBytes(h, []byte(s))
}

func hashPutF64(h hash.Hash, v float64) {
	hashPutU64(h, math.Float64bits(v))
}
