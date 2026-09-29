package ygo

import (
	"bytes"
	"io"

	"riguz.com/ygo/internal/lib0"
)

type Decode interface {
	Decode(decoder *Decoder)
	DecodeV1([]uint8) error
	DecodeV2([]uint8) error
}

// Decoder is the version-agnostic update reader. As with Encoder, it does not
// embed lib0.Read: in V2 the raw scalar methods target "rest", while strings
// come from the string column.
type Decoder interface {
	ReadVarUint() (uint64, error)
	ReadVarUint8Array() ([]uint8, error)
	ReadVarString() (string, error)
	ResetDsCurVal()
	ReadDsClock() (uint64, error)
	ReadDsLen() (uint64, error)
	ReadLeftId() (ID, error)
	ReadRightId() (ID, error)
	ReadClient() (ClientID, error)
	ReadInfo() (uint8, error)
	ReadParentInfo() (bool, error)
	ReadTypeRef() (uint8, error)
	ReadLen() (uint64, error)
	ReadKey() (*string, error)
	ReadJson() (Any, error)
	ReadAnyValue() (Any, error)
}

var _ Decoder = &DecoderV1{}

type DecoderV1 struct {
	cursor lib0.BufferRead
}

func NewDecoderV1(reader io.Reader) DecoderV1 {
	w := lib0.NewBufferRead(reader)
	return DecoderV1{
		cursor: w,
	}
}

// Raw lib0 forwards, used directly by tests. Not part of the Decoder interface.
func (d *DecoderV1) ReadUint8Array(len uint) ([]uint8, error) { return d.cursor.ReadUint8Array(len) }
func (d *DecoderV1) ReadUint8() (uint8, error)                { return d.cursor.ReadUint8() }
func (d *DecoderV1) ReadUint16() (uint16, error)              { return d.cursor.ReadUint16() }
func (d *DecoderV1) ReadUint32() (uint32, error)              { return d.cursor.ReadUint32() }
func (d *DecoderV1) ReadUint32BigEndian() (uint32, error)     { return d.cursor.ReadUint32BigEndian() }
func (d *DecoderV1) ReadUint64() (uint64, error)              { return d.cursor.ReadUint64() }
func (d *DecoderV1) ReadFloat32() (float32, error)            { return d.cursor.ReadFloat32() }
func (d *DecoderV1) ReadFloat64() (float64, error)            { return d.cursor.ReadFloat64() }
func (d *DecoderV1) ReadInt64() (int64, error)                { return d.cursor.ReadInt64() }
func (d *DecoderV1) ReadVarInt() (int64, error)               { return d.cursor.ReadVarInt() }
func (d *DecoderV1) ReadAny() (any, error)                    { return d.cursor.ReadAny() }

func (d *DecoderV1) ReadVarUint() (uint64, error)        { return d.cursor.ReadVarUint() }
func (d *DecoderV1) ReadVarUint8Array() ([]uint8, error) { return d.cursor.ReadVarUint8Array() }
func (d *DecoderV1) ReadVarString() (string, error)      { return d.cursor.ReadVarString() }

func (d *DecoderV1) readId() (ID, error) {
	client, err := d.cursor.ReadVarUint()
	if err != nil {
		return ID{}, err
	}
	clock, err := d.cursor.ReadVarUint()
	if err != nil {
		return ID{}, err
	}
	return ID{
		Client: ClientID(client),
		Clock:  clock,
	}, nil
}

func (d *DecoderV1) ResetDsCurVal() {}

func (d *DecoderV1) ReadDsClock() (uint64, error) {
	return d.cursor.ReadVarUint()
}

func (d *DecoderV1) ReadDsLen() (uint64, error) {
	return d.cursor.ReadVarUint()
}

func (d *DecoderV1) ReadLeftId() (ID, error) {
	return d.readId()
}

func (d *DecoderV1) ReadRightId() (ID, error) {
	return d.readId()
}

func (d *DecoderV1) ReadClient() (ClientID, error) {
	id, err := d.cursor.ReadVarUint()
	return ClientID(id), err
}

func (d *DecoderV1) ReadInfo() (uint8, error) {
	return d.cursor.ReadUint8()
}

func (d *DecoderV1) ReadParentInfo() (bool, error) {
	v, err := d.cursor.ReadVarUint()
	if err != nil {
		return false, err
	}
	return v == 1, nil
}

func (d *DecoderV1) ReadTypeRef() (uint8, error) {
	v, err := d.cursor.ReadVarUint()
	return uint8(v), err
}

func (d *DecoderV1) ReadLen() (uint64, error) {
	return d.cursor.ReadVarUint()
}

func (d *DecoderV1) ReadKey() (*string, error) {
	str, err := d.cursor.ReadVarString()
	return &str, err
}

func (d *DecoderV1) ReadJson() (Any, error) {
	s, err := d.cursor.ReadVarString()
	if err != nil {
		return UndefinedAny(), err
	}
	return anyFromJSON(s)
}

func (d *DecoderV1) ReadAnyValue() (Any, error) {
	return readAnyValueFrom(&d.cursor)
}

// ── V2 ────────────────────────────────────────────────────────────────────────

var _ Decoder = &DecoderV2{}

// DecoderV2 reads the column-oriented Yjs V2 update format.
type DecoderV2 struct {
	rest      *lib0.BufferRead
	dsCurrVal uint64
	keys      []string

	keyClockDecoder   *IntDiffOptRleDecoder
	clientDecoder     *UIntOptRleDecoder
	leftClockDecoder  *IntDiffOptRleDecoder
	rightClockDecoder *IntDiffOptRleDecoder
	infoDecoder       *RleDecoder
	stringDecoder     *StringDecoder
	parentInfoDecoder *RleDecoder
	typeRefDecoder    *UIntOptRleDecoder
	lenDecoder        *UIntOptRleDecoder
}

func NewDecoderV2(data []uint8) (*DecoderV2, error) {
	r := lib0.NewBufferRead(bytes.NewReader(data))
	if _, err := r.ReadVarUint(); err != nil { // feature flag
		return nil, err
	}
	cols := make([][]uint8, 9)
	for i := range cols {
		c, err := r.ReadVarUint8Array()
		if err != nil {
			return nil, err
		}
		cols[i] = c
	}
	rest, err := r.ReadRemaining()
	if err != nil {
		return nil, err
	}
	sd, err := NewStringDecoder(cols[5])
	if err != nil {
		return nil, err
	}
	restReader := lib0.NewBufferRead(bytes.NewReader(rest))
	return &DecoderV2{
		rest:              &restReader,
		keyClockDecoder:   NewIntDiffOptRleDecoder(cols[0]),
		clientDecoder:     NewUIntOptRleDecoder(cols[1]),
		leftClockDecoder:  NewIntDiffOptRleDecoder(cols[2]),
		rightClockDecoder: NewIntDiffOptRleDecoder(cols[3]),
		infoDecoder:       NewRleDecoder(cols[4]),
		stringDecoder:     sd,
		parentInfoDecoder: NewRleDecoder(cols[6]),
		typeRefDecoder:    NewUIntOptRleDecoder(cols[7]),
		lenDecoder:        NewUIntOptRleDecoder(cols[8]),
	}, nil
}

func (d *DecoderV2) ReadVarUint() (uint64, error)        { return d.rest.ReadVarUint() }
func (d *DecoderV2) ReadVarUint8Array() ([]uint8, error) { return d.rest.ReadVarUint8Array() }
func (d *DecoderV2) ReadVarString() (string, error)      { return d.stringDecoder.Read() }

func (d *DecoderV2) ResetDsCurVal() { d.dsCurrVal = 0 }

func (d *DecoderV2) ReadDsClock() (uint64, error) {
	diff, err := d.rest.ReadVarUint()
	if err != nil {
		return 0, err
	}
	d.dsCurrVal += diff
	return d.dsCurrVal, nil
}

func (d *DecoderV2) ReadDsLen() (uint64, error) {
	diff, err := d.rest.ReadVarUint()
	if err != nil {
		return 0, err
	}
	length := diff + 1
	d.dsCurrVal += length
	return length, nil
}

func (d *DecoderV2) ReadLeftId() (ID, error) {
	client, err := d.clientDecoder.Read()
	if err != nil {
		return ID{}, err
	}
	clock, err := d.leftClockDecoder.Read()
	if err != nil {
		return ID{}, err
	}
	return ID{Client: ClientID(client), Clock: uint64(clock)}, nil
}

func (d *DecoderV2) ReadRightId() (ID, error) {
	client, err := d.clientDecoder.Read()
	if err != nil {
		return ID{}, err
	}
	clock, err := d.rightClockDecoder.Read()
	if err != nil {
		return ID{}, err
	}
	return ID{Client: ClientID(client), Clock: uint64(clock)}, nil
}

func (d *DecoderV2) ReadClient() (ClientID, error) {
	client, err := d.clientDecoder.Read()
	return ClientID(client), err
}

func (d *DecoderV2) ReadInfo() (uint8, error) { return d.infoDecoder.Read() }

func (d *DecoderV2) ReadParentInfo() (bool, error) {
	b, err := d.parentInfoDecoder.Read()
	return b == 1, err
}

func (d *DecoderV2) ReadTypeRef() (uint8, error) {
	v, err := d.typeRefDecoder.Read()
	return uint8(v), err
}

func (d *DecoderV2) ReadLen() (uint64, error) { return d.lenDecoder.Read() }

func (d *DecoderV2) ReadKey() (*string, error) {
	clock, err := d.keyClockDecoder.Read()
	if err != nil {
		return nil, err
	}
	if clock >= 0 && int(clock) < len(d.keys) {
		s := d.keys[clock]
		return &s, nil
	}
	s, err := d.stringDecoder.Read()
	if err != nil {
		return nil, err
	}
	d.keys = append(d.keys, s)
	return &s, nil
}

func (d *DecoderV2) ReadJson() (Any, error)     { return readAnyValueFrom(d.rest) }
func (d *DecoderV2) ReadAnyValue() (Any, error) { return readAnyValueFrom(d.rest) }
