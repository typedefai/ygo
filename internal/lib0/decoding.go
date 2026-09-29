package lib0

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

type Read interface {
	ReadUint8Array(len uint) ([]uint8, error)
	ReadUint8() (uint8, error)
	ReadUint16() (uint16, error)
	ReadUint32() (uint32, error)
	ReadUint32BigEndian() (uint32, error)
	ReadUint64() (uint64, error)
	ReadFloat32() (float32, error)
	ReadFloat64() (float64, error)
	ReadInt64() (int64, error)
	ReadVarUint8Array() ([]uint8, error)
	ReadVarUint() (uint64, error)
	ReadVarInt() (int64, error)
	ReadVarString() (string, error)
	ReadAny() (any, error)
}

var _ Read = &BufferRead{}

type BufferRead struct {
	reader *bufio.Reader
}

func NewBufferRead(reader io.Reader) BufferRead {
	return BufferRead{
		reader: bufio.NewReader(reader),
	}
}

func (r *BufferRead) ReadUint8Array(len uint) ([]uint8, error) {
	buf := make([]uint8, len)
	if _, err := io.ReadFull(r.reader, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func (r *BufferRead) ReadUint8() (uint8, error) {
	buf, err := r.ReadUint8Array(1)
	if err != nil {
		return 0, err
	}
	return buf[0], nil
}

func (r *BufferRead) ReadUint16() (uint16, error) {
	var value uint16
	err := binary.Read(r.reader, binary.LittleEndian, &value)
	return value, err
}

func (r *BufferRead) ReadUint32() (uint32, error) {
	var value uint32
	err := binary.Read(r.reader, binary.LittleEndian, &value)
	return value, err
}

func (r *BufferRead) ReadUint32BigEndian() (uint32, error) {
	var value uint32
	err := binary.Read(r.reader, binary.BigEndian, &value)
	return value, err
}

func (r *BufferRead) ReadUint64() (uint64, error) {
	var value uint64
	err := binary.Read(r.reader, binary.BigEndian, &value)
	return value, err
}

func (r *BufferRead) ReadFloat32() (float32, error) {
	var value float32
	err := binary.Read(r.reader, binary.BigEndian, &value)
	return value, err
}

func (r *BufferRead) ReadFloat64() (float64, error) {
	var value float64
	err := binary.Read(r.reader, binary.BigEndian, &value)
	return value, err
}

func (r *BufferRead) ReadInt64() (int64, error) {
	var value int64
	err := binary.Read(r.reader, binary.BigEndian, &value)
	return value, err
}

func (r *BufferRead) ReadVarUint() (uint64, error) {
	return binary.ReadUvarint(r.reader)
}

func (r *BufferRead) ReadVarUint8Array() ([]uint8, error) {
	len, err := r.ReadVarUint()
	if err != nil {
		return nil, err
	}
	return r.ReadUint8Array(uint(len))
}

// ReadVarIntWithSign decodes lib0's sign-magnitude VarInt, returning the
// magnitude and sign bit separately. ReadVarInt collapses the sign into an
// int64 and therefore cannot represent -0, which lib0 emits for run markers;
// run-length decoders need this variant.
func (r *BufferRead) ReadVarIntWithSign() (uint64, bool, error) {
	firstByte, err := r.ReadUint8()
	if err != nil {
		return 0, false, err
	}
	mag := uint64(firstByte & 0b0011_1111)
	neg := firstByte&uint8(0b0100_0000) != 0
	if firstByte&uint8(0b1000_0000) == 0 {
		return mag, neg, nil
	}
	shift := 6
	for {
		b, err := r.ReadUint8()
		if err != nil {
			return 0, false, err
		}
		mag |= (uint64(b) & 0b0111_1111) << shift
		shift += 7
		if b < uint8(0b1000_0000) {
			return mag, neg, nil
		}
		if shift > 70 {
			return 0, false, errors.New("varint size exceeded length of 70 bits")
		}
	}
}

func (r *BufferRead) ReadVarInt() (int64, error) {
	mag, neg, err := r.ReadVarIntWithSign()
	if err != nil {
		return 0, err
	}
	if neg {
		return -int64(mag), nil
	}
	return int64(mag), nil
}

// HasContent reports whether at least one more byte can be read. Used by the
// RLE decoders, whose final run has no count and extends to the end of the
// column.
func (r *BufferRead) HasContent() bool {
	_, err := r.reader.Peek(1)
	return err == nil
}

// ReadRemaining drains the reader. Used to split a length-prefixed column from
// the raw trailing bytes of a V2 update.
func (r *BufferRead) ReadRemaining() ([]byte, error) {
	return io.ReadAll(r.reader)
}

func (r *BufferRead) ReadVarString() (string, error) {
	buf, err := r.ReadVarUint8Array()
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

func (r *BufferRead) ReadAny() (any, error) {
	t, err := r.ReadUint8()
	if err != nil {
		return nil, err
	}
	switch t {
	case 127:
		return Undefined{}, nil
	case 126:
		return nil, nil
	case 125:
		return r.ReadVarInt()
	case 124:
		return r.ReadFloat32()
	case 123:
		return r.ReadFloat64()
	case 122:
		v, err := r.ReadInt64()
		if err != nil {
			return nil, err
		}
		return BigInt(v), nil
	case 121:
		return false, nil
	case 120:
		return true, nil
	case 119:
		return r.ReadVarString()
	case 118:
		n, err := r.ReadVarUint()
		if err != nil {
			return nil, err
		}
		obj := make(map[string]any, n)
		for range n {
			key, err := r.ReadVarString()
			if err != nil {
				return nil, err
			}
			val, err := r.ReadAny()
			if err != nil {
				return nil, err
			}
			obj[key] = val
		}
		return obj, nil
	case 117:
		n, err := r.ReadVarUint()
		if err != nil {
			return nil, err
		}
		arr := make([]any, n)
		for i := range n {
			val, err := r.ReadAny()
			if err != nil {
				return nil, err
			}
			arr[i] = val
		}
		return arr, nil
	case 116:
		return r.ReadVarUint8Array()
	default:
		return nil, fmt.Errorf("unknown any type: %v", t)
	}
}
