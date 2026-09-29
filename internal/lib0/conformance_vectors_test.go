package lib0_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"gotest.tools/assert"
	"riguz.com/ygo/internal/lib0"
)

// Reference wire vectors generated from lib0@0.2.99 by
// testutil/gen_lib0_vectors.js. Run `bun run testutil/gen_lib0_vectors.js`
// to regenerate after a lib0 version bump.
const vectorsPath = "../../testutil/vectors/lib0_vectors.json"

type valueVector struct {
	Value string `json:"value"`
	Hex   string `json:"hex"`
}

type anyVector struct {
	Name   string          `json:"name"`
	Go     json.RawMessage `json:"go"`
	SkipGo bool            `json:"skipGo"`
	Hex    string          `json:"hex"`
}

type lib0Vectors struct {
	Varuint       []valueVector `json:"varuint"`
	Varint        []valueVector `json:"varint"`
	Varstring     []valueVector `json:"varstring"`
	Varuint8Array []valueVector `json:"varuint8array"`
	Any           []anyVector   `json:"any"`
}

func loadVectors(t *testing.T) lib0Vectors {
	t.Helper()
	raw, err := os.ReadFile(filepath.FromSlash(vectorsPath))
	if err != nil {
		t.Fatalf("read %s: %v (run `bun run testutil/gen_lib0_vectors.js`)", vectorsPath, err)
	}
	var v lib0Vectors
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		t.Fatalf("parse %s: %v", vectorsPath, err)
	}
	return v
}

func TestConformance_VarUint(t *testing.T) {
	for _, v := range loadVectors(t).Varuint {
		t.Run(v.Value, func(t *testing.T) {
			n, err := strconv.ParseUint(v.Value, 10, 64)
			assert.NilError(t, err)
			w := lib0.NewBufferWrite()
			assert.NilError(t, w.WriteVarUint64(n))
			assert.Equal(t, v.Hex, hex.EncodeToString(w.ToBytes()))
		})
	}
}

func TestConformance_VarInt(t *testing.T) {
	for _, v := range loadVectors(t).Varint {
		t.Run(v.Value, func(t *testing.T) {
			w := lib0.NewBufferWrite()
			if v.Value == "-0" {
				// lib0's writeVarInt distinguishes -0 (0x40) from +0 (0x00);
				// int64 cannot express -0, so exercise the dedicated helper.
				assert.NilError(t, w.WriteNegVarUint(0))
			} else {
				n, err := strconv.ParseInt(v.Value, 10, 64)
				assert.NilError(t, err)
				assert.NilError(t, w.WriteVarInt64(n))
			}
			assert.Equal(t, v.Hex, hex.EncodeToString(w.ToBytes()))
		})
	}
}

func TestConformance_VarString(t *testing.T) {
	for _, v := range loadVectors(t).Varstring {
		t.Run(v.Value, func(t *testing.T) {
			w := lib0.NewBufferWrite()
			s := v.Value
			assert.NilError(t, w.WriteVarString(&s))
			assert.Equal(t, v.Hex, hex.EncodeToString(w.ToBytes()))
		})
	}
}

func TestConformance_VarUint8Array(t *testing.T) {
	for _, v := range loadVectors(t).Varuint8Array {
		t.Run(v.Value, func(t *testing.T) {
			buf, err := hex.DecodeString(v.Value)
			assert.NilError(t, err)
			w := lib0.NewBufferWrite()
			assert.NilError(t, w.WriteVarUint8Array(buf))
			assert.Equal(t, v.Hex, hex.EncodeToString(w.ToBytes()))
		})
	}
}

// TestConformance_Any replays each reference lib0 writeAny vector through our
// encoder. Vectors marked skipGo describe JavaScript values with no Go
// equivalent (-0 as an integer; 2^53+1, which a JS Number cannot hold).
func TestConformance_Any(t *testing.T) {
	for _, v := range loadVectors(t).Any {
		if v.SkipGo {
			t.Logf("skip (no Go equivalent): %s", v.Name)
			continue
		}
		t.Run(v.Name, func(t *testing.T) {
			val := decodeAnyValue(t, v.Go)
			w := lib0.NewBufferWrite()
			assert.NilError(t, w.WriteAny(val))
			assert.Equal(t, v.Hex, hex.EncodeToString(w.ToBytes()))
		})
	}
}

func decodeAnyValue(t *testing.T, raw json.RawMessage) any {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		t.Fatalf("decode any descriptor %s: %v", raw, err)
	}
	return anyFromDescriptor(t, m)
}

func anyFromDescriptor(t *testing.T, m map[string]any) any {
	t.Helper()
	switch m["t"] {
	case "undefined":
		return lib0.Undefined{}
	case "null":
		return nil
	case "true":
		return true
	case "false":
		return false
	case "int":
		n, err := m["v"].(json.Number).Int64()
		assert.NilError(t, err)
		return n
	case "number":
		f, err := m["v"].(json.Number).Float64()
		assert.NilError(t, err)
		return f
	case "bigint":
		n, err := m["v"].(json.Number).Int64()
		assert.NilError(t, err)
		return lib0.BigInt(n)
	case "string":
		return m["v"].(string)
	case "bytes":
		buf, err := hex.DecodeString(m["hex"].(string))
		assert.NilError(t, err)
		return buf
	case "array":
		in := m["v"].([]any)
		out := make([]any, len(in))
		for i, e := range in {
			out[i] = anyFromDescriptor(t, e.(map[string]any))
		}
		return out
	case "object":
		in := m["v"].(map[string]any)
		out := make(map[string]any, len(in))
		for k, e := range in {
			out[k] = anyFromDescriptor(t, e.(map[string]any))
		}
		return out
	default:
		t.Fatalf("unknown any descriptor tag %q", m["t"])
		return nil
	}
}
