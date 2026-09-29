package lib0

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"sort"
)

type Undefined struct{}

// BigInt represents lib0's writeAny tag 122: a signed 64-bit integer that is
// not representable as a JavaScript Number. lib0 encodes it big-endian and
// decodes it back to a JS BigInt.
type BigInt int64

type Write interface {
	WriteUint8Array(buf []uint8) error
	WriteUint8(num uint8) error
	WriteUint16(num uint16) error
	WriteUint32(num uint32) error
	WriteUint32BigEndian(num uint32) error
	WriteUint64(num uint64) error
	WriteFloat32(num float32) error
	WriteFloat64(num float64) error
	WriteInt64(num int64) error
	WriteVarUint(num uint) error
	WriteVarUint8(num uint8) error
	WriteVarUint16(num uint16) error
	WriteVarUint32(num uint32) error
	WriteVarUint64(num uint64) error
	WriteVarInt(num int) error
	WriteVarInt8(num int8) error
	WriteVarInt16(num int16) error
	WriteVarInt32(num int32) error
	WriteVarInt64(num int64) error
	WriteVarUint8Array(buf []uint8) error
	WriteVarString(str *string) error
	WriteAny(a any) error
	ToBytes() []byte
}

var _ Write = &BufferWrite{}

type BufferWrite struct {
	buffer *bytes.Buffer
}

func NewBufferWrite() BufferWrite {
	writer := BufferWrite{
		buffer: &bytes.Buffer{},
	}
	return writer
}

func (w *BufferWrite) ToBytes() []byte {
	return w.buffer.Bytes()
}

func (w *BufferWrite) WriteUint8Array(buf []uint8) error {
	_, err := w.buffer.Write(buf)
	return err
}

func (w *BufferWrite) WriteUint8(value uint8) error {
	_, err := w.buffer.Write([]byte{value})
	return err
}

func (w *BufferWrite) WriteUint16(num uint16) error {
	return binary.Write(w.buffer, binary.LittleEndian, num)
}

func (w *BufferWrite) WriteUint32(num uint32) error {
	return binary.Write(w.buffer, binary.LittleEndian, num)
}

func (w *BufferWrite) WriteUint32BigEndian(num uint32) error {
	return binary.Write(w.buffer, binary.BigEndian, num)
}

func (w *BufferWrite) WriteUint64(num uint64) error {
	return binary.Write(w.buffer, binary.BigEndian, num)
}

func (w *BufferWrite) WriteInt64(num int64) error {
	return binary.Write(w.buffer, binary.BigEndian, num)
}

func (w *BufferWrite) WriteFloat32(num float32) error {
	return binary.Write(w.buffer, binary.BigEndian, num)
}

func (w *BufferWrite) WriteFloat64(num float64) error {
	return binary.Write(w.buffer, binary.BigEndian, num)
}

func (w *BufferWrite) WriteVarUint(num uint) error {
	return w.WriteVarUint64(uint64(num))
}

func (w *BufferWrite) WriteVarUint8(num uint8) error {
	return w.WriteVarUint64(uint64(num))
}

func (w *BufferWrite) WriteVarUint16(num uint16) error {
	return w.WriteVarUint64(uint64(num))
}

func (w *BufferWrite) WriteVarUint32(num uint32) error {
	return w.WriteVarUint64(uint64(num))
}

func (w *BufferWrite) WriteVarUint64(num uint64) error {
	buf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(buf, uint64(num))
	return w.WriteUint8Array(buf[:n])
}

func (w *BufferWrite) WriteVarInt(num int) error {
	return w.WriteVarInt64(int64(num))
}

func (w *BufferWrite) WriteVarInt8(num int8) error {
	return w.WriteVarInt64(int64(num))
}

func (w *BufferWrite) WriteVarInt16(num int16) error {
	return w.WriteVarInt64(int64(num))
}

func (w *BufferWrite) WriteVarInt32(num int32) error {
	return w.WriteVarInt64(int64(num))
}

func (w *BufferWrite) WriteVarInt64(num int64) error {
	isNegative := num < 0
	if isNegative {
		num = -num
	}
	firstByte := uint8(int64(0b0011_1111) & num)
	if num > int64(0b0011_1111) {
		firstByte |= uint8(0b1000_0000) // continue reading or not
	}
	if isNegative {
		firstByte |= uint8(0b0100_0000) // is negative or not
	}
	if err := w.WriteUint8(firstByte); err != nil {
		return err
	}

	num >>= 6
	for num > 0 {
		var b uint8 = 0
		if num > int64(0b0111_1111) {
			b |= uint8(0b1000_0000)
		}
		b |= uint8(int64(0b0111_1111) & num)
		if err := w.WriteUint8(b); err != nil {
			return err
		}
		num >>= 7
	}
	return nil
}

// WriteNegVarUint writes -(v) using lib0's sign-magnitude VarInt format.
// Unlike WriteVarInt64(-int64(v)), it can represent negative zero: lib0
// distinguishes -0 (0x40) from +0 (0x00) because its writeVarInt uses
// math.isNegativeZero. Run-length encoders (UIntOptRleEncoder) rely on this
// to mark a negative value that carries a repeat count.
func (w *BufferWrite) WriteNegVarUint(v uint64) error {
	const (
		sign = uint8(0b0100_0000)
	)
	if v < 1<<6 {
		return w.WriteUint8(sign | uint8(v))
	}
	if err := w.WriteUint8(0b1000_0000 | sign | uint8(v&0b0011_1111)); err != nil {
		return err
	}
	v >>= 6
	for v >= 1<<7 {
		if err := w.WriteUint8(0b1000_0000 | uint8(v&0b0111_1111)); err != nil {
			return err
		}
		v >>= 7
	}
	return w.WriteUint8(uint8(v))
}

func (w *BufferWrite) WriteVarUint8Array(buf []uint8) error {
	if err := w.WriteVarUint(uint(len(buf))); err != nil {
		return err
	}
	return w.WriteUint8Array(buf)
}

func (w *BufferWrite) WriteVarString(str *string) error {
	return w.WriteVarUint8Array([]byte(*str))
}

func (w *BufferWrite) WriteAny(a any) error {
	switch t := a.(type) {
	case string:
		return w.writeString(t)
	case float32:
		return w.writeFloat64(float64(t))
	case float64:
		return w.writeFloat64(t)
	case int8:
		return w.writeAnyInt(int64(t))
	case int16:
		return w.writeAnyInt(int64(t))
	case int32:
		return w.writeAnyInt(int64(t))
	case int:
		return w.writeAnyInt(int64(t))
	case int64:
		return w.writeAnyInt(t)
	case uint:
		return w.writeAnyUint(uint64(t))
	case uint8:
		return w.writeAnyUint(uint64(t))
	case uint16:
		return w.writeAnyUint(uint64(t))
	case uint32:
		return w.writeAnyUint(uint64(t))
	case uint64:
		return w.writeAnyUint(t)
	case BigInt:
		if err := w.WriteUint8(122); err != nil {
			return err
		}
		return w.WriteInt64(int64(t))
	case bool:
		return w.writeBool(t)
	case []uint8:
		return w.writeUint8Array(t)
	case []any:
		return w.writeArray(t)
	case map[string]any:
		return w.writeObject(t)
	case nil:
		return w.WriteUint8(126)
	case Undefined:
		return w.WriteUint8(127)
	default:
		return fmt.Errorf("unrecognized any payload type:%v", reflect.TypeOf(a))
	}
}

func (w *BufferWrite) writeVarInt(num int32) error {
	if err := w.WriteUint8(125); err != nil {
		return err
	}
	return w.WriteVarInt32(num)
}

func (w *BufferWrite) writeFloat32(num float32) error {
	if err := w.WriteUint8(124); err != nil {
		return err
	}
	return w.WriteFloat32(num)
}

// maxFallbackInt is 2^53, the largest magnitude at which every integer is
// exactly representable as a float64 (JavaScript's Number). Whole int64 values
// beyond it are emitted as BigInt (tag 122) so precision survives the wire.
const maxFallbackInt = int64(1) << 53
const minFallbackInt = -maxFallbackInt

// lib0's writeAny writes an integer as tag-125 VarInt when
// `number.isInteger(data) && Math.abs(data) <= BITS31` (BITS31 = 2^31-1).
// The bound is on the absolute value, so -2^31 is NOT an int32-range integer:
// it takes the float path and is encoded as tag 124.
const maxAnyInt = int64(math.MaxInt32)
const minAnyInt = int64(math.MinInt32) + 1 // -2147483647

// writeFloat64 encodes a number using lib0's value-based writeAny dispatch
// (JavaScript has a single number type, so float32/float64/int are not
// distinguished on the wire):
//
//   - integral value in int32 range → tag 125 (VarInt)
//   - value exactly representable as float32 → tag 124 (4 bytes)
//   - otherwise → tag 123 (8 bytes)
func (w *BufferWrite) writeFloat64(num float64) error {
	if math.Trunc(num) == num && num >= float64(minAnyInt) && num <= float64(maxAnyInt) {
		return w.writeVarInt(int32(num))
	}
	if isFloat32Lossless(num) {
		return w.writeFloat32(float32(num))
	}
	if err := w.WriteUint8(123); err != nil {
		return err
	}
	return w.WriteFloat64(num)
}

// writeAnyInt encodes an integer using lib0's writeAny number dispatch.
// Values outside int32 but within float64's exact integer range take the float
// path (tags 124/123); values beyond it use tag 122 (BigInt), which is how a
// Go int64 can round-trip through a JS peer without precision loss.
func (w *BufferWrite) writeAnyInt(v int64) error {
	if v >= minAnyInt && v <= maxAnyInt {
		return w.writeVarInt(int32(v))
	}
	if v >= minFallbackInt && v <= maxFallbackInt {
		return w.writeFloat64(float64(v))
	}
	if err := w.WriteUint8(122); err != nil {
		return err
	}
	return w.WriteInt64(v)
}

// writeAnyUint routes a uint64 through the signed path when it fits, and falls
// back to float64 (with documented precision loss) above MaxInt64, matching
// lib0's behaviour for Numbers in that range.
func (w *BufferWrite) writeAnyUint(v uint64) error {
	if v <= math.MaxInt64 {
		return w.writeAnyInt(int64(v))
	}
	if err := w.WriteUint8(123); err != nil {
		return err
	}
	return w.WriteFloat64(float64(v))
}

// isFloat32Lossless reports whether num round-trips through float32 exactly,
// matching lib0's isFloat32 check. NaN and ±Inf intentionally fall through to
// the float64 tag, exactly as they do in lib0.
func isFloat32Lossless(num float64) bool {
	return float64(float32(num)) == num
}

func (w *BufferWrite) writeBool(val bool) error {
	var t uint8 = 121
	if val {
		t = 120
	}

	return w.WriteUint8(t)
}

func (w *BufferWrite) writeString(str string) error {
	if err := w.WriteUint8(119); err != nil {
		return err
	}
	return w.WriteVarString(&str)
}

func (w *BufferWrite) writeObject(obj map[string]any) error {
	if err := w.WriteUint8(118); err != nil {
		return err
	}
	if err := w.WriteVarUint(uint(len(obj))); err != nil {
		return err
	}
	// JS object key order follows insertion; Go maps have none, so sort keys
	// for deterministic output. The decoded object is identical either way.
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := w.WriteVarString(&k); err != nil {
			return err
		}
		if err := w.WriteAny(obj[k]); err != nil {
			return err
		}
	}
	return nil
}

func (w *BufferWrite) writeArray(arr []any) error {
	if err := w.WriteUint8(117); err != nil {
		return err
	}
	if err := w.WriteVarUint(uint(len(arr))); err != nil {
		return err
	}
	for _, a := range arr {
		if err := w.WriteAny(a); err != nil {
			return err
		}
	}
	return nil
}

func (w *BufferWrite) writeUint8Array(arr []byte) error {
	if err := w.WriteUint8(116); err != nil {
		return err
	}
	return w.WriteVarUint8Array(arr)
}
