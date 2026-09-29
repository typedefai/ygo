package ygo

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"riguz.com/ygo/internal/lib0"
)

// Any represents a tagged union of all possible value types in the yjs protocol.
type Any struct {
	Tag    AnyTag
	IntVal int32
	F32Val float32
	F64Val float64
	BigVal int64
	StrVal string
	ObjVal map[string]Any
	// ObjKeys preserves object key order as decoded from the wire. Yjs writes
	// object keys in insertion order, so byte-identical re-encoding requires
	// it; values built from Go maps leave it nil and are sorted.
	ObjKeys []string
	ArrVal  []Any
	BinVal  []uint8
}

type AnyTag uint8

const (
	AnyUndefined AnyTag = iota
	AnyNull
	AnyInteger
	AnyFloat32
	AnyFloat64
	AnyBigInt64
	AnyFalse
	AnyTrue
	AnyString
	AnyObject
	AnyArray
	AnyBinary
)

func UndefinedAny() Any              { return Any{Tag: AnyUndefined} }
func NullAny() Any                   { return Any{Tag: AnyNull} }
func FalseAny() Any                  { return Any{Tag: AnyFalse} }
func TrueAny() Any                   { return Any{Tag: AnyTrue} }
func IntegerAny(v int32) Any         { return Any{Tag: AnyInteger, IntVal: v} }
func Float32Any(v float32) Any       { return Any{Tag: AnyFloat32, F32Val: v} }
func Float64Any(v float64) Any       { return Any{Tag: AnyFloat64, F64Val: v} }
func BigInt64Any(v int64) Any        { return Any{Tag: AnyBigInt64, BigVal: v} }
func StringAny(v string) Any         { return Any{Tag: AnyString, StrVal: v} }
func ObjectAny(v map[string]Any) Any { return Any{Tag: AnyObject, ObjVal: v} }
func ArrayAny(v []Any) Any           { return Any{Tag: AnyArray, ArrVal: v} }
func BinaryAny(v []uint8) Any        { return Any{Tag: AnyBinary, BinVal: v} }
func BoolAny(v bool) Any {
	if v {
		return TrueAny()
	}
	return FalseAny()
}

func (a Any) Equal(b Any) bool {
	if a.Tag != b.Tag {
		return false
	}
	switch a.Tag {
	case AnyUndefined, AnyNull, AnyFalse, AnyTrue:
		return true
	case AnyInteger:
		return a.IntVal == b.IntVal
	case AnyFloat32:
		return a.F32Val == b.F32Val
	case AnyFloat64:
		return a.F64Val == b.F64Val
	case AnyBigInt64:
		return a.BigVal == b.BigVal
	case AnyString:
		return a.StrVal == b.StrVal
	case AnyBinary:
		if len(a.BinVal) != len(b.BinVal) {
			return false
		}
		for i := range a.BinVal {
			if a.BinVal[i] != b.BinVal[i] {
				return false
			}
		}
		return true
	case AnyArray:
		if len(a.ArrVal) != len(b.ArrVal) {
			return false
		}
		for i := range a.ArrVal {
			if !a.ArrVal[i].Equal(b.ArrVal[i]) {
				return false
			}
		}
		return true
	case AnyObject:
		if len(a.ObjVal) != len(b.ObjVal) {
			return false
		}
		for k, v := range a.ObjVal {
			bv, ok := b.ObjVal[k]
			if !ok || !v.Equal(bv) {
				return false
			}
		}
		return true
	}
	return false
}

// ReadAny decodes one tag-based Any value from the update stream.
func ReadAny(decoder Decoder) (Any, error) {
	return decoder.ReadAnyValue()
}

// WriteAny encodes one tag-based Any value into the update stream.
func WriteAny(encoder Encoder, a Any) error {
	return encoder.WriteAnyValue(a)
}

// readAnyValueFrom decodes lib0's tagged-union wire format from a raw reader
// (the V1 stream or the V2 "rest" stream).
func readAnyValueFrom(r lib0.Read) (Any, error) {
	index, err := r.ReadUint8()
	if err != nil {
		return UndefinedAny(), err
	}
	tag := 127 - index
	switch tag {
	case 0:
		return UndefinedAny(), nil
	case 1:
		return NullAny(), nil
	case 2:
		v, err := r.ReadVarInt()
		if err != nil {
			return UndefinedAny(), err
		}
		return IntegerAny(int32(v)), nil
	case 3:
		v, err := r.ReadFloat32()
		if err != nil {
			return UndefinedAny(), err
		}
		return Float32Any(v), nil
	case 4:
		v, err := r.ReadFloat64()
		if err != nil {
			return UndefinedAny(), err
		}
		return Float64Any(v), nil
	case 5:
		v, err := r.ReadInt64()
		if err != nil {
			return UndefinedAny(), err
		}
		return BigInt64Any(v), nil
	case 6:
		return FalseAny(), nil
	case 7:
		return TrueAny(), nil
	case 8:
		v, err := r.ReadVarString()
		if err != nil {
			return UndefinedAny(), err
		}
		return StringAny(v), nil
	case 9:
		length, err := r.ReadVarUint()
		if err != nil {
			return UndefinedAny(), err
		}
		obj := make(map[string]Any, length)
		keys := make([]string, 0, length)
		for i := uint64(0); i < length; i++ {
			key, err := r.ReadVarString()
			if err != nil {
				return UndefinedAny(), err
			}
			val, err := readAnyValueFrom(r)
			if err != nil {
				return UndefinedAny(), err
			}
			obj[key] = val
			keys = append(keys, key)
		}
		out := ObjectAny(obj)
		out.ObjKeys = keys
		return out, nil
	case 10:
		length, err := r.ReadVarUint()
		if err != nil {
			return UndefinedAny(), err
		}
		arr := make([]Any, length)
		for i := uint64(0); i < length; i++ {
			val, err := readAnyValueFrom(r)
			if err != nil {
				return UndefinedAny(), err
			}
			arr[i] = val
		}
		return ArrayAny(arr), nil
	case 11:
		buf, err := r.ReadVarUint8Array()
		if err != nil {
			return UndefinedAny(), err
		}
		return BinaryAny(buf), nil
	default:
		return UndefinedAny(), nil
	}
}

// writeAnyValueTo encodes lib0's tagged-union wire format onto a raw writer.
func writeAnyValueTo(w lib0.Write, a Any) error {
	switch a.Tag {
	case AnyUndefined:
		return w.WriteUint8(127)
	case AnyNull:
		return w.WriteUint8(126)
	case AnyInteger:
		if err := w.WriteUint8(125); err != nil {
			return err
		}
		return w.WriteVarInt32(a.IntVal)
	case AnyFloat32:
		if err := w.WriteUint8(124); err != nil {
			return err
		}
		return w.WriteFloat32(a.F32Val)
	case AnyFloat64:
		if err := w.WriteUint8(123); err != nil {
			return err
		}
		return w.WriteFloat64(a.F64Val)
	case AnyBigInt64:
		if err := w.WriteUint8(122); err != nil {
			return err
		}
		return w.WriteInt64(a.BigVal)
	case AnyFalse:
		return w.WriteUint8(121)
	case AnyTrue:
		return w.WriteUint8(120)
	case AnyString:
		if err := w.WriteUint8(119); err != nil {
			return err
		}
		return w.WriteVarString(&a.StrVal)
	case AnyObject:
		if err := w.WriteUint8(118); err != nil {
			return err
		}
		if err := w.WriteVarUint64(uint64(len(a.ObjVal))); err != nil {
			return err
		}
		for _, k := range orderedObjKeys(a) {
			if err := w.WriteVarString(&k); err != nil {
				return err
			}
			if err := writeAnyValueTo(w, a.ObjVal[k]); err != nil {
				return err
			}
		}
		return nil
	case AnyArray:
		if err := w.WriteUint8(117); err != nil {
			return err
		}
		if err := w.WriteVarUint64(uint64(len(a.ArrVal))); err != nil {
			return err
		}
		for _, v := range a.ArrVal {
			if err := writeAnyValueTo(w, v); err != nil {
				return err
			}
		}
		return nil
	case AnyBinary:
		if err := w.WriteUint8(116); err != nil {
			return err
		}
		return w.WriteVarUint8Array(a.BinVal)
	default:
		return fmt.Errorf("unknown Any tag: %d", a.Tag)
	}
}

func ReadMultipleAny(decoder Decoder) ([]Any, error) {
	length, err := decoder.ReadLen()
	if err != nil {
		return nil, err
	}
	result := make([]Any, length)
	for i := uint64(0); i < length; i++ {
		val, err := decoder.ReadAnyValue()
		if err != nil {
			return nil, err
		}
		result[i] = val
	}
	return result, nil
}

func WriteMultipleAny(encoder Encoder, anys []Any) error {
	if err := encoder.WriteLen(uint64(len(anys))); err != nil {
		return err
	}
	for _, a := range anys {
		if err := encoder.WriteAnyValue(a); err != nil {
			return err
		}
	}
	return nil
}

// orderedObjKeys returns object keys in their preserved wire order when
// available, otherwise sorted (Go maps have no order). Byte-identical encoding
// against yjs requires the former.
func orderedObjKeys(a Any) []string {
	if len(a.ObjKeys) == len(a.ObjVal) {
		valid := true
		for _, k := range a.ObjKeys {
			if _, ok := a.ObjVal[k]; !ok {
				valid = false
				break
			}
		}
		if valid {
			return a.ObjKeys
		}
	}
	keys := make([]string, 0, len(a.ObjVal))
	for k := range a.ObjVal {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// anyToJSON serialises an Any value to JSON text, used by the legacy V1
// embed/format content (JSON.stringify in yjs). Written by hand rather than
// via encoding/json so object key order matches the reference and no HTML
// escaping is applied.
func anyToJSON(a Any) (string, error) {
	var b strings.Builder
	if err := writeAnyJSON(&b, a); err != nil {
		return "", err
	}
	return b.String(), nil
}

func writeAnyJSON(b *strings.Builder, a Any) error {
	switch a.Tag {
	case AnyUndefined, AnyNull:
		// JSON.stringify(undefined) is undefined, which yjs cannot write as a
		// string either; emit null rather than invalid JSON.
		b.WriteString("null")
	case AnyTrue:
		b.WriteString("true")
	case AnyFalse:
		b.WriteString("false")
	case AnyInteger:
		b.WriteString(strconv.FormatInt(int64(a.IntVal), 10))
	case AnyFloat32:
		b.WriteString(formatJSONFloat(float64(a.F32Val)))
	case AnyFloat64:
		b.WriteString(formatJSONFloat(a.F64Val))
	case AnyBigInt64:
		b.WriteString(strconv.FormatInt(a.BigVal, 10))
	case AnyString:
		b.WriteString(strconv.Quote(a.StrVal))
	case AnyBinary:
		// JSON.stringify(Uint8Array) yields an object with numeric keys.
		b.WriteByte('{')
		for i, v := range a.BinVal {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(strconv.Quote(strconv.Itoa(i)))
			b.WriteByte(':')
			b.WriteString(strconv.FormatUint(uint64(v), 10))
		}
		b.WriteByte('}')
	case AnyArray:
		b.WriteByte('[')
		for i, e := range a.ArrVal {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := writeAnyJSON(b, e); err != nil {
				return err
			}
		}
		b.WriteByte(']')
	case AnyObject:
		b.WriteByte('{')
		for i, k := range orderedObjKeys(a) {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(strconv.Quote(k))
			b.WriteByte(':')
			if err := writeAnyJSON(b, a.ObjVal[k]); err != nil {
				return err
			}
		}
		b.WriteByte('}')
	default:
		return fmt.Errorf("unknown Any tag: %d", a.Tag)
	}
	return nil
}

func formatJSONFloat(f float64) string {
	// JSON.stringify(Infinity|NaN) is "null".
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return "null"
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// anyFromJSON parses JSON text (V1 embed/format) into an Any value, preserving
// object key order via a token stream.
func anyFromJSON(s string) (Any, error) {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	return anyFromJSONToken(dec)
}

func anyFromJSONToken(dec *json.Decoder) (Any, error) {
	tok, err := dec.Token()
	if err != nil {
		return UndefinedAny(), err
	}
	return anyFromJSONValue(dec, tok)
}

func anyFromJSONValue(dec *json.Decoder, tok json.Token) (Any, error) {
	switch t := tok.(type) {
	case nil:
		return NullAny(), nil
	case bool:
		return BoolAny(t), nil
	case string:
		return StringAny(t), nil
	case json.Number:
		if i, err := t.Int64(); err == nil && i >= math.MinInt32 && i <= math.MaxInt32 {
			return IntegerAny(int32(i)), nil
		}
		f, err := t.Float64()
		if err != nil {
			return UndefinedAny(), err
		}
		return Float64Any(f), nil
	case json.Delim:
		switch t {
		case '[':
			arr := []Any{}
			for dec.More() {
				e, err := anyFromJSONToken(dec)
				if err != nil {
					return UndefinedAny(), err
				}
				arr = append(arr, e)
			}
			if _, err := dec.Token(); err != nil { // consume ']'
				return UndefinedAny(), err
			}
			return ArrayAny(arr), nil
		case '{':
			obj := map[string]Any{}
			keys := []string{}
			for dec.More() {
				keyTok, err := dec.Token()
				if err != nil {
					return UndefinedAny(), err
				}
				key, _ := keyTok.(string)
				val, err := anyFromJSONToken(dec)
				if err != nil {
					return UndefinedAny(), err
				}
				obj[key] = val
				keys = append(keys, key)
			}
			if _, err := dec.Token(); err != nil { // consume '}'
				return UndefinedAny(), err
			}
			out := ObjectAny(obj)
			out.ObjKeys = keys
			return out, nil
		}
	}
	return UndefinedAny(), fmt.Errorf("unsupported JSON token %v", tok)
}
