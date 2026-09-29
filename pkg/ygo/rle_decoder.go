package ygo

import (
	"bytes"
	"unicode/utf8"

	"riguz.com/ygo/internal/lib0"
)

// RleDecoder is the inverse of RleEncoder: [value, VarUint(count-1)] pairs with
// the final run's count omitted (it runs to the end of the column).
type RleDecoder struct {
	r     *lib0.BufferRead
	state uint8
	count int // 0 = next run; -1 = final run (unbounded)
}

func NewRleDecoder(data []byte) *RleDecoder {
	r := lib0.NewBufferRead(bytes.NewReader(data))
	return &RleDecoder{r: &r}
}

func (d *RleDecoder) Read() (uint8, error) {
	if d.count == 0 {
		b, err := d.r.ReadUint8()
		if err != nil {
			return 0, err
		}
		d.state = b
		if d.r.HasContent() {
			cnt, err := d.r.ReadVarUint()
			if err != nil {
				return 0, err
			}
			d.count = int(cnt) + 1
		} else {
			d.count = -1
		}
	}
	if d.count > 0 {
		d.count--
	}
	return d.state, nil
}

// UIntOptRleDecoder is the inverse of UIntOptRleEncoder. A negative sign (which
// lib0 encodes as the sign-magnitude bit, including -0) marks a run, followed
// by VarUint(count-2).
type UIntOptRleDecoder struct {
	r     *lib0.BufferRead
	state uint64
	count int
}

func NewUIntOptRleDecoder(data []byte) *UIntOptRleDecoder {
	r := lib0.NewBufferRead(bytes.NewReader(data))
	return &UIntOptRleDecoder{r: &r}
}

func (d *UIntOptRleDecoder) Read() (uint64, error) {
	if d.count == 0 {
		mag, neg, err := d.r.ReadVarIntWithSign()
		if err != nil {
			return 0, err
		}
		d.state = mag
		d.count = 1
		if neg {
			cnt, err := d.r.ReadVarUint()
			if err != nil {
				return 0, err
			}
			d.count = int(cnt) + 2
		}
	}
	d.count--
	return d.state, nil
}

// IntDiffOptRleDecoder is the inverse of IntDiffOptRleEncoder. The LSB of the
// VarInt diff marks whether a count follows.
type IntDiffOptRleDecoder struct {
	r     *lib0.BufferRead
	state int64
	diff  int64
	count int
}

func NewIntDiffOptRleDecoder(data []byte) *IntDiffOptRleDecoder {
	r := lib0.NewBufferRead(bytes.NewReader(data))
	return &IntDiffOptRleDecoder{r: &r}
}

func (d *IntDiffOptRleDecoder) Read() (int64, error) {
	if d.count == 0 {
		encoded, err := d.r.ReadVarInt()
		if err != nil {
			return 0, err
		}
		hasCount := encoded & 1
		d.diff = encoded >> 1 // arithmetic shift
		d.count = 1
		if hasCount != 0 {
			cnt, err := d.r.ReadVarUint()
			if err != nil {
				return 0, err
			}
			d.count = int(cnt) + 2
		}
	}
	d.state += d.diff
	d.count--
	return d.state, nil
}

// StringDecoder is the inverse of StringEncoder: the column is a VarString
// holding all strings concatenated, followed by UIntOptRle-encoded UTF-16
// lengths. Reads slice the pool sequentially.
type StringDecoder struct {
	str     string
	bytePos int
	lens    *UIntOptRleDecoder
}

func NewStringDecoder(data []byte) (*StringDecoder, error) {
	d := &StringDecoder{lens: NewUIntOptRleDecoder(nil)}
	if len(data) == 0 {
		return d, nil
	}
	r := lib0.NewBufferRead(bytes.NewReader(data))
	str, err := r.ReadVarString()
	if err != nil {
		return nil, err
	}
	rest, err := r.ReadRemaining()
	if err != nil {
		return nil, err
	}
	d.str = str
	d.lens = NewUIntOptRleDecoder(rest)
	return d, nil
}

func (d *StringDecoder) Read() (string, error) {
	units, err := d.lens.Read()
	if err != nil {
		return "", err
	}
	start := d.bytePos
	d.bytePos = advanceUTF16(d.str, start, int(units))
	return d.str[start:d.bytePos], nil
}

// advanceUTF16 returns the byte offset reached by advancing `units` UTF-16 code
// units forward from byteStart, scanning only the requested span.
func advanceUTF16(s string, byteStart, units int) int {
	bytePos := byteStart
	counted := 0
	for counted < units && bytePos < len(s) {
		r, size := utf8.DecodeRuneInString(s[bytePos:])
		if r >= 0x10000 {
			counted += 2
		} else {
			counted++
		}
		bytePos += size
	}
	return bytePos
}
