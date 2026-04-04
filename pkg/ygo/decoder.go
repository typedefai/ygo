package ygo

import (
	"io"

	"riguz.com/ygo/internal/lib0"
)

type Decode interface {
	Decode(decoder *Decoder)
	DecodeV1([]uint8) error
	DecodeV2([]uint8) error
}

type Decoder interface {
	lib0.Read
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
}

var _ Decoder = &DecoderV1{}

type DecoderV1 struct {
	cursor lib0.Read
}

func NewDecoderV1(reader io.Reader) DecoderV1 {
	w := lib0.NewBufferRead(reader)
	return DecoderV1{
		cursor: &w,
	}
}

func (d *DecoderV1) ReadUint8Array(len uint) ([]uint8, error) { return d.cursor.ReadUint8Array(len) }
func (d *DecoderV1) ReadUint8() (uint8, error)                { return d.cursor.ReadUint8() }
func (d *DecoderV1) ReadUint16() (uint16, error)              { return d.cursor.ReadUint16() }
func (d *DecoderV1) ReadUint32() (uint32, error)              { return d.cursor.ReadUint32() }
func (d *DecoderV1) ReadUint32BigEndian() (uint32, error)     { return d.cursor.ReadUint32BigEndian() }
func (d *DecoderV1) ReadUint64() (uint64, error)              { return d.cursor.ReadUint64() }
func (d *DecoderV1) ReadFloat32() (float32, error)            { return d.cursor.ReadFloat32() }
func (d *DecoderV1) ReadFloat64() (float64, error)            { return d.cursor.ReadFloat64() }
func (d *DecoderV1) ReadInt64() (int64, error)                { return d.cursor.ReadInt64() }
func (d *DecoderV1) ReadVarUint8Array() ([]uint8, error)      { return d.cursor.ReadVarUint8Array() }
func (d *DecoderV1) ReadVarUint() (uint64, error)             { return d.cursor.ReadVarUint() }
func (d *DecoderV1) ReadVarInt() (int64, error)               { return d.cursor.ReadVarInt() }
func (d *DecoderV1) ReadVarString() (string, error)           { return d.cursor.ReadVarString() }
func (d *DecoderV1) ReadAny() (any, error)                    { return d.cursor.ReadAny() }

func (d *DecoderV1) readId() (ID, error) {
	client, err := d.ReadVarUint()
	if err != nil {
		return ID{}, err
	}
	clock, err := d.ReadVarUint()
	if err != nil {
		return ID{}, err
	}
	return ID{
		Client: ClientID(client),
		Clock:  clock,
	}, nil
}

func (d *DecoderV1) ResetDsCurVal() {
	/* no op */
}

func (d *DecoderV1) ReadDsClock() (uint64, error) {
	return d.ReadVarUint()
}

func (d *DecoderV1) ReadDsLen() (uint64, error) {
	return d.ReadVarUint()
}

func (d *DecoderV1) ReadLeftId() (ID, error) {
	return d.readId()
}

func (d *DecoderV1) ReadRightId() (ID, error) {
	return d.readId()
}

func (d *DecoderV1) ReadClient() (ClientID, error) {
	id, err := d.ReadVarUint()
	return ClientID(id), err
}

func (d *DecoderV1) ReadInfo() (uint8, error) {
	return d.ReadUint8()
}

func (d *DecoderV1) ReadParentInfo() (bool, error) {
	v, err := d.ReadVarUint()
	if err != nil {
		return false, err
	}
	return v == 1, nil
}

func (d *DecoderV1) ReadTypeRef() (uint8, error) {
	return d.ReadUint8()
}

func (d *DecoderV1) ReadLen() (uint64, error) {
	return d.ReadVarUint()
}

func (d *DecoderV1) ReadKey() (*string, error) {
	str, err := d.ReadVarString()
	return &str, err
}
