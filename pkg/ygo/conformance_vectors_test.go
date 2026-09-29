package ygo_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/assert"
	"riguz.com/ygo/pkg/ygo"
)

// RLE vector half of the reference corpus generated from lib0@0.2.99 by
// testutil/gen_lib0_vectors.js. The primitive (varint/any) half lives in
// internal/lib0/conformance_vectors_test.go.
const rleVectorsPath = "../../testutil/vectors/lib0_vectors.json"

type rleVectors struct {
	Rle []struct {
		Value []uint8 `json:"value"`
		Hex   string  `json:"hex"`
	} `json:"rle"`
	Uintopt []struct {
		Value []uint64 `json:"value"`
		Hex   string   `json:"hex"`
	} `json:"uintopt"`
	Intdiff []struct {
		Value []int64 `json:"value"`
		Hex   string  `json:"hex"`
	} `json:"intdiff"`
	StringEncoder []struct {
		Value []string `json:"value"`
		Hex   string   `json:"hex"`
	} `json:"stringencoder"`
}

func loadRLEVectors(t *testing.T) rleVectors {
	t.Helper()
	raw, err := os.ReadFile(filepath.FromSlash(rleVectorsPath))
	if err != nil {
		t.Fatalf("read %s: %v (run `bun run testutil/gen_lib0_vectors.js`)", rleVectorsPath, err)
	}
	var v rleVectors
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		t.Fatalf("parse %s: %v", rleVectorsPath, err)
	}
	return v
}

func TestConformance_RleEncoder(t *testing.T) {
	for i, v := range loadRLEVectors(t).Rle {
		t.Run(fmt.Sprintf("case%d", i), func(t *testing.T) {
			e := ygo.NewRleEncoder()
			for _, x := range v.Value {
				assert.NilError(t, e.Write(x))
			}
			assert.Equal(t, v.Hex, hex.EncodeToString(e.ToBytes()))
		})
	}
}

func TestConformance_UIntOptRleEncoder(t *testing.T) {
	for i, v := range loadRLEVectors(t).Uintopt {
		t.Run(fmt.Sprintf("case%d", i), func(t *testing.T) {
			e := ygo.NewUIntOptRleEncoder()
			for _, x := range v.Value {
				assert.NilError(t, e.Write(x))
			}
			b, err := e.ToBytes()
			assert.NilError(t, err)
			assert.Equal(t, v.Hex, hex.EncodeToString(b))
		})
	}
}

func TestConformance_IntDiffOptRleEncoder(t *testing.T) {
	for i, v := range loadRLEVectors(t).Intdiff {
		t.Run(fmt.Sprintf("case%d", i), func(t *testing.T) {
			e := ygo.NewIntDiffOptRleEncoder()
			for _, x := range v.Value {
				// Write takes uint32; converting preserves the two's-complement
				// bits the encoder interprets as int32.
				assert.NilError(t, e.Write(uint32(x)))
			}
			b, err := e.ToBytes()
			assert.NilError(t, err)
			assert.Equal(t, v.Hex, hex.EncodeToString(b))
		})
	}
}

func TestConformance_StringEncoder(t *testing.T) {
	for i, v := range loadRLEVectors(t).StringEncoder {
		t.Run(fmt.Sprintf("case%d", i), func(t *testing.T) {
			e := ygo.NewStringEncoder()
			for _, x := range v.Value {
				s := x
				assert.NilError(t, e.Write(&s))
			}
			b, err := e.ToBytes()
			assert.NilError(t, err)
			assert.Equal(t, v.Hex, hex.EncodeToString(b))
		})
	}
}
