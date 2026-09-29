package ygo

import (
	"unicode/utf16"

	"riguz.com/ygo/internal/lib0"
)

type Encode interface {
	Encode(encoder *Encoder)
	EncodeV1() ([]uint8, error)
	EncodeV2() ([]uint8, error)
}

// Encoder is the version-agnostic update writer used by content and struct
// encoding. It intentionally does NOT embed lib0.Write: in V1 the raw methods
// and the whole stream coincide, but in V2 they route to different columns
// (e.g. strings go to the string column, while Any payloads go to "rest").
type Encoder interface {
	WriteVarUint64(num uint64) error
	WriteVarUint8Array(buf []uint8) error
	WriteVarString(str *string) error
	ResetDsCurVal()
	WriteDsClock(clock uint64) error
	WriteDsLen(len uint64) error
	WriteLeftId(id ID) error
	WriteRightId(id ID) error
	WriteClient(client ClientID) error
	WriteInfo(info uint8) error
	WriteParentInfo(isYKey bool) error
	WriteTypeRef(info uint8) error
	WriteLen(len uint64) error
	WriteJson(a Any) error
	WriteKey(key *string) error
	WriteAnyValue(a Any) error
}

var _ Encoder = &EncoderV1{}

type EncoderV1 struct {
	buf lib0.BufferWrite
}

func NewEncoderV1() EncoderV1 {
	w := lib0.NewBufferWrite()
	return EncoderV1{
		buf: w,
	}
}

// Raw lib0 forwards. These implement the concrete V1 stream and are also used
// directly by tests; they are not part of the Encoder interface.
func (e *EncoderV1) WriteUint8Array(buf []uint8) error     { return e.buf.WriteUint8Array(buf) }
func (e *EncoderV1) WriteUint8(num uint8) error            { return e.buf.WriteUint8(num) }
func (e *EncoderV1) WriteUint16(num uint16) error          { return e.buf.WriteUint16(num) }
func (e *EncoderV1) WriteUint32(num uint32) error          { return e.buf.WriteUint32(num) }
func (e *EncoderV1) WriteUint32BigEndian(num uint32) error { return e.buf.WriteUint32BigEndian(num) }
func (e *EncoderV1) WriteUint64(num uint64) error          { return e.buf.WriteUint64(num) }
func (e *EncoderV1) WriteFloat32(num float32) error        { return e.buf.WriteFloat32(num) }
func (e *EncoderV1) WriteFloat64(num float64) error        { return e.buf.WriteFloat64(num) }
func (e *EncoderV1) WriteInt64(num int64) error            { return e.buf.WriteInt64(num) }
func (e *EncoderV1) WriteVarUint(num uint) error           { return e.buf.WriteVarUint(num) }
func (e *EncoderV1) WriteVarUint8(num uint8) error         { return e.buf.WriteVarUint8(num) }
func (e *EncoderV1) WriteVarUint16(num uint16) error       { return e.buf.WriteVarUint16(num) }
func (e *EncoderV1) WriteVarUint32(num uint32) error       { return e.buf.WriteVarUint32(num) }
func (e *EncoderV1) WriteVarInt(num int) error             { return e.buf.WriteVarInt(num) }
func (e *EncoderV1) WriteVarInt8(num int8) error           { return e.buf.WriteVarInt8(num) }
func (e *EncoderV1) WriteVarInt16(num int16) error         { return e.buf.WriteVarInt16(num) }
func (e *EncoderV1) WriteVarInt32(num int32) error         { return e.buf.WriteVarInt32(num) }
func (e *EncoderV1) WriteVarInt64(num int64) error         { return e.buf.WriteVarInt64(num) }
func (e *EncoderV1) WriteAny(a any) error                  { return e.buf.WriteAny(a) }
func (e *EncoderV1) ToBytes() []uint8                      { return e.buf.ToBytes() }

// WriteId writes an ID as two lib0 VarUints. Yjs UpdateEncoderV1.writeLeftId /
// writeRightId use writeVarUint for both client and clock.
func (e *EncoderV1) WriteId(id ID) error {
	if err := e.buf.WriteVarUint64(uint64(id.Client)); err != nil {
		return err
	}
	return e.buf.WriteVarUint64(id.Clock)
}

func (e *EncoderV1) ResetDsCurVal() {}

func (e *EncoderV1) WriteDsClock(clock uint64) error {
	return e.buf.WriteVarUint64(clock)
}

func (e *EncoderV1) WriteDsLen(length uint64) error {
	return e.buf.WriteVarUint64(length)
}

func (e *EncoderV1) WriteLeftId(id ID) error {
	return e.WriteId(id)
}

func (e *EncoderV1) WriteRightId(id ID) error {
	return e.WriteId(id)
}

func (e *EncoderV1) WriteClient(client ClientID) error {
	return e.buf.WriteVarUint64(uint64(client))
}

func (e *EncoderV1) WriteInfo(info uint8) error {
	return e.buf.WriteUint8(info)
}

func (e *EncoderV1) WriteParentInfo(isYKey bool) error {
	var i uint32 = 0
	if isYKey {
		i = 1
	}
	return e.buf.WriteVarUint32(i)
}

func (e *EncoderV1) WriteTypeRef(info uint8) error {
	return e.buf.WriteVarUint64(uint64(info))
}

func (e *EncoderV1) WriteLen(length uint64) error {
	return e.buf.WriteVarUint64(length)
}

func (e *EncoderV1) WriteVarString(str *string) error     { return e.buf.WriteVarString(str) }
func (e *EncoderV1) WriteVarUint64(num uint64) error      { return e.buf.WriteVarUint64(num) }
func (e *EncoderV1) WriteVarUint8Array(buf []uint8) error { return e.buf.WriteVarUint8Array(buf) }

// WriteJson encodes with legacy V1 semantics: JSON text as a VarString.
func (e *EncoderV1) WriteJson(a Any) error {
	s, err := anyToJSON(a)
	if err != nil {
		return err
	}
	return e.buf.WriteVarString(&s)
}

func (e *EncoderV1) WriteKey(key *string) error {
	return e.buf.WriteVarString(key)
}

func (e *EncoderV1) WriteAnyValue(a Any) error {
	return writeAnyValueTo(&e.buf, a)
}

type IntDiffOptRleEncoder struct {
	buf   lib0.BufferWrite
	last  uint32
	count uint32
	diff  int32
}

func NewIntDiffOptRleEncoder() IntDiffOptRleEncoder {
	return IntDiffOptRleEncoder{
		buf:   lib0.NewBufferWrite(),
		last:  0,
		count: 0,
		diff:  0,
	}
}

func (i *IntDiffOptRleEncoder) ToBytes() ([]uint8, error) {
	if err := i.flush(); err != nil {
		return nil, err
	}
	return i.buf.ToBytes(), nil
}

func (i *IntDiffOptRleEncoder) Write(value uint32) error {
	var diff int32 = int32(value) - int32(i.last)
	if i.diff == diff {
		i.last = value
		i.count += 1
	} else {
		if err := i.flush(); err != nil {
			return err
		}
		i.count = 1
		i.diff = diff
		i.last = value
	}
	return nil
}

func (i *IntDiffOptRleEncoder) flush() error {
	if i.count > 0 {
		var encodeDiff int32 = i.diff << 1
		if i.count == 1 {
			encodeDiff |= 0
		} else {
			encodeDiff |= 1
		}
		if err := i.buf.WriteVarInt64(int64(encodeDiff)); err != nil {
			return err
		}
		if i.count > 1 {
			if err := i.buf.WriteVarUint32(i.count - 2); err != nil {
				return err
			}
		}
	}
	return nil
}

type UIntOptRleEncoder struct {
	buf   lib0.BufferWrite
	last  uint64
	count uint32
}

func NewUIntOptRleEncoder() UIntOptRleEncoder {
	return UIntOptRleEncoder{
		buf:   lib0.NewBufferWrite(),
		last:  0,
		count: 0,
	}
}

func (u *UIntOptRleEncoder) ToBytes() ([]uint8, error) {
	if err := u.flush(); err != nil {
		return nil, err
	}
	return u.buf.ToBytes(), nil
}

func (u *UIntOptRleEncoder) Write(value uint64) error {
	if u.last == value {
		u.count += 1
	} else {
		if err := u.flush(); err != nil {
			return err
		}
		u.count = 1
		u.last = value
	}
	return nil
}

func (u *UIntOptRleEncoder) flush() error {
	if u.count > 0 {
		if u.count == 1 {
			return u.buf.WriteVarInt64(int64(u.last))
		}
		// A run is signalled by writing -value. This must use the
		// sign-magnitude helper rather than WriteVarInt64(-int64(last)): for a
		// run of zeros, -0 (0x40) is distinct from +0 (0x00) on the wire and
		// collapsing them makes the decoder read the count as the next value.
		if err := u.buf.WriteNegVarUint(u.last); err != nil {
			return err
		}
		return u.buf.WriteVarUint32(u.count - 2)
	}
	return nil
}

// RleEncoder run-length-encodes a byte sequence as [value, VarUint(count-1)]
// pairs; the final run's count is omitted (the decoder infers it from the end
// of the column).
type RleEncoder struct {
	buf   lib0.BufferWrite
	last  *uint8
	count uint32
}

func NewRleEncoder() RleEncoder {
	return RleEncoder{
		buf:   lib0.NewBufferWrite(),
		last:  nil,
		count: 0,
	}
}

func (r *RleEncoder) ToBytes() []uint8 { return r.buf.ToBytes() }

func (r *RleEncoder) Write(value uint8) error {
	if r.last != nil && *r.last == value {
		r.count += 1
	} else {
		if r.count > 0 {
			if err := r.buf.WriteVarUint32(r.count - 1); err != nil {
				return err
			}
		}
		r.count = 1
		if err := r.buf.WriteUint8(value); err != nil {
			return err
		}
		r.last = &value
	}
	return nil
}

type StringEncoder struct {
	buf        lib0.BufferWrite
	str        string
	lenEncoder UIntOptRleEncoder
}

func NewStringEncoder() StringEncoder {
	return StringEncoder{
		buf:        lib0.NewBufferWrite(),
		str:        "",
		lenEncoder: NewUIntOptRleEncoder(),
	}
}

func (s *StringEncoder) ToBytes() ([]uint8, error) {
	lengths, err := s.lenEncoder.ToBytes()
	if err != nil {
		return nil, err
	}
	writer := lib0.NewBufferWrite()
	if err := writer.WriteVarString(&s.str); err != nil {
		return nil, err
	}
	if err := writer.WriteUint8Array(lengths); err != nil {
		return nil, err
	}
	return writer.ToBytes(), nil
}

func (s *StringEncoder) Write(str *string) error {
	utf16Units := utf16.Encode([]rune(*str))
	utf16Len := len(utf16Units)
	s.str += *str
	return s.lenEncoder.Write(uint64(utf16Len))
}

// ── V2 ────────────────────────────────────────────────────────────────────────

var _ Encoder = &EncoderV2{}

// EncoderV2 implements the column-oriented Yjs V2 update format: optimized
// fields go into dedicated RLE columns, while strings, Any payloads and most
// scalar values go into the trailing "rest" stream.
type EncoderV2 struct {
	rest      lib0.BufferWrite
	keyClock  uint32
	dsCurrVal uint64

	keyClockEncoder   IntDiffOptRleEncoder
	clientEncoder     UIntOptRleEncoder
	leftClockEncoder  IntDiffOptRleEncoder
	rightClockEncoder IntDiffOptRleEncoder
	infoEncoder       RleEncoder
	stringEncoder     StringEncoder
	parentInfoEncoder RleEncoder
	typeRefEncoder    UIntOptRleEncoder
	lenEncoder        UIntOptRleEncoder
}

func NewEncoderV2() *EncoderV2 {
	return &EncoderV2{
		rest:              lib0.NewBufferWrite(),
		keyClockEncoder:   NewIntDiffOptRleEncoder(),
		clientEncoder:     NewUIntOptRleEncoder(),
		leftClockEncoder:  NewIntDiffOptRleEncoder(),
		rightClockEncoder: NewIntDiffOptRleEncoder(),
		infoEncoder:       NewRleEncoder(),
		stringEncoder:     NewStringEncoder(),
		parentInfoEncoder: NewRleEncoder(),
		typeRefEncoder:    NewUIntOptRleEncoder(),
		lenEncoder:        NewUIntOptRleEncoder(),
	}
}

func (e *EncoderV2) ToBytes() ([]uint8, error) {
	keyClock, err := e.keyClockEncoder.ToBytes()
	if err != nil {
		return nil, err
	}
	client, err := e.clientEncoder.ToBytes()
	if err != nil {
		return nil, err
	}
	leftClock, err := e.leftClockEncoder.ToBytes()
	if err != nil {
		return nil, err
	}
	rightClock, err := e.rightClockEncoder.ToBytes()
	if err != nil {
		return nil, err
	}
	info := e.infoEncoder.ToBytes()
	str, err := e.stringEncoder.ToBytes()
	if err != nil {
		return nil, err
	}
	parentInfo := e.parentInfoEncoder.ToBytes()
	typeRef, err := e.typeRefEncoder.ToBytes()
	if err != nil {
		return nil, err
	}
	lengths, err := e.lenEncoder.ToBytes()
	if err != nil {
		return nil, err
	}
	rest := e.rest.ToBytes()

	writer := lib0.NewBufferWrite()
	if err := writer.WriteVarUint(0); err != nil { // feature flag
		return nil, err
	}
	// The first nine columns are length-prefixed; "rest" is appended raw.
	for _, arr := range [][]uint8{keyClock, client, leftClock, rightClock,
		info, str, parentInfo, typeRef, lengths} {
		if err := writer.WriteVarUint8Array(arr); err != nil {
			return nil, err
		}
	}
	if err := writer.WriteUint8Array(rest); err != nil {
		return nil, err
	}
	return writer.ToBytes(), nil
}

// WriteVarString writes to the string column (parent names, parentSub, text
// content, and format/xml keys all share it, in write order).
func (e *EncoderV2) WriteVarString(str *string) error { return e.stringEncoder.Write(str) }

// WriteVarUint64 and WriteVarUint8Array target the rest stream.
func (e *EncoderV2) WriteVarUint64(num uint64) error      { return e.rest.WriteVarUint64(num) }
func (e *EncoderV2) WriteVarUint8Array(buf []uint8) error { return e.rest.WriteVarUint8Array(buf) }

func (e *EncoderV2) ResetDsCurVal() { e.dsCurrVal = 0 }

func (e *EncoderV2) WriteDsClock(clock uint64) error {
	diff := clock - e.dsCurrVal
	e.dsCurrVal = clock
	return e.rest.WriteVarUint64(diff)
}

func (e *EncoderV2) WriteDsLen(length uint64) error {
	if length == 0 {
		return NewInvalidDeleteSetLenError()
	}
	if err := e.rest.WriteVarUint64(length - 1); err != nil {
		return err
	}
	e.dsCurrVal += length
	return nil
}

func (e *EncoderV2) WriteLeftId(id ID) error {
	if err := e.clientEncoder.Write(uint64(id.Client)); err != nil {
		return err
	}
	return e.leftClockEncoder.Write(uint32(id.Clock))
}

func (e *EncoderV2) WriteRightId(id ID) error {
	if err := e.clientEncoder.Write(uint64(id.Client)); err != nil {
		return err
	}
	return e.rightClockEncoder.Write(uint32(id.Clock))
}

func (e *EncoderV2) WriteClient(client ClientID) error {
	return e.clientEncoder.Write(uint64(client))
}

func (e *EncoderV2) WriteInfo(info uint8) error { return e.infoEncoder.Write(info) }

func (e *EncoderV2) WriteParentInfo(isYKey bool) error {
	var b uint8 = 0
	if isYKey {
		b = 1
	}
	return e.parentInfoEncoder.Write(b)
}

func (e *EncoderV2) WriteTypeRef(info uint8) error { return e.typeRefEncoder.Write(uint64(info)) }

func (e *EncoderV2) WriteLen(length uint64) error { return e.lenEncoder.Write(length) }

// WriteJson and WriteAnyValue write lib0 Any payloads to the rest stream.
func (e *EncoderV2) WriteJson(a Any) error     { return writeAnyValueTo(&e.rest, a) }
func (e *EncoderV2) WriteAnyValue(a Any) error { return writeAnyValueTo(&e.rest, a) }

// WriteKey mirrors Yjs UpdateEncoderV2.writeKey. The keyMap optimisation is
// disabled upstream (the cache is never populated), so every key emits a fresh
// keyClock followed by its string.
func (e *EncoderV2) WriteKey(key *string) error {
	if err := e.keyClockEncoder.Write(e.keyClock); err != nil {
		return err
	}
	e.keyClock++
	return e.stringEncoder.Write(key)
}
