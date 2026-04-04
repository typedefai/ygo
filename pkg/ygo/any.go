package ygo

import (
	"fmt"
)

// Any represents a tagged union of all possible value types in the yjs protocol.
type Any struct {
	Tag      AnyTag
	IntVal   int32
	F32Val   float32
	F64Val   float64
	BigVal   int64
	StrVal   string
	ObjVal   map[string]Any
	ArrVal   []Any
	BinVal   []uint8
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

func UndefinedAny() Any  { return Any{Tag: AnyUndefined} }
func NullAny() Any       { return Any{Tag: AnyNull} }
func FalseAny() Any      { return Any{Tag: AnyFalse} }
func TrueAny() Any       { return Any{Tag: AnyTrue} }
func IntegerAny(v int32) Any { return Any{Tag: AnyInteger, IntVal: v} }
func Float32Any(v float32) Any { return Any{Tag: AnyFloat32, F32Val: v} }
func Float64Any(v float64) Any { return Any{Tag: AnyFloat64, F64Val: v} }
func BigInt64Any(v int64) Any  { return Any{Tag: AnyBigInt64, BigVal: v} }
func StringAny(v string) Any   { return Any{Tag: AnyString, StrVal: v} }
func ObjectAny(v map[string]Any) Any { return Any{Tag: AnyObject, ObjVal: v} }
func ArrayAny(v []Any) Any     { return Any{Tag: AnyArray, ArrVal: v} }
func BinaryAny(v []uint8) Any  { return Any{Tag: AnyBinary, BinVal: v} }
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

func ReadAny(decoder Decoder) (Any, error) {
	index, err := decoder.ReadUint8()
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
		v, err := decoder.ReadVarInt()
		if err != nil {
			return UndefinedAny(), err
		}
		return IntegerAny(int32(v)), nil
	case 3:
		v, err := decoder.ReadFloat32()
		if err != nil {
			return UndefinedAny(), err
		}
		return Float32Any(v), nil
	case 4:
		v, err := decoder.ReadFloat64()
		if err != nil {
			return UndefinedAny(), err
		}
		return Float64Any(v), nil
	case 5:
		v, err := decoder.ReadInt64()
		if err != nil {
			return UndefinedAny(), err
		}
		return BigInt64Any(v), nil
	case 6:
		return FalseAny(), nil
	case 7:
		return TrueAny(), nil
	case 8:
		v, err := decoder.ReadVarString()
		if err != nil {
			return UndefinedAny(), err
		}
		return StringAny(v), nil
	case 9:
		length, err := decoder.ReadVarUint()
		if err != nil {
			return UndefinedAny(), err
		}
		obj := make(map[string]Any, length)
		for i := uint64(0); i < length; i++ {
			key, err := decoder.ReadVarString()
			if err != nil {
				return UndefinedAny(), err
			}
			val, err := ReadAny(decoder)
			if err != nil {
				return UndefinedAny(), err
			}
			obj[key] = val
		}
		return ObjectAny(obj), nil
	case 10:
		length, err := decoder.ReadVarUint()
		if err != nil {
			return UndefinedAny(), err
		}
		arr := make([]Any, length)
		for i := uint64(0); i < length; i++ {
			val, err := ReadAny(decoder)
			if err != nil {
				return UndefinedAny(), err
			}
			arr[i] = val
		}
		return ArrayAny(arr), nil
	case 11:
		buf, err := decoder.ReadVarUint8Array()
		if err != nil {
			return UndefinedAny(), err
		}
		return BinaryAny(buf), nil
	default:
		return UndefinedAny(), nil
	}
}

func WriteAny(encoder Encoder, a Any) error {
	switch a.Tag {
	case AnyUndefined:
		return encoder.WriteUint8(127)
	case AnyNull:
		return encoder.WriteUint8(126)
	case AnyInteger:
		if err := encoder.WriteUint8(125); err != nil {
			return err
		}
		return encoder.WriteVarInt32(a.IntVal)
	case AnyFloat32:
		if err := encoder.WriteUint8(124); err != nil {
			return err
		}
		return encoder.WriteFloat32(a.F32Val)
	case AnyFloat64:
		if err := encoder.WriteUint8(123); err != nil {
			return err
		}
		return encoder.WriteFloat64(a.F64Val)
	case AnyBigInt64:
		if err := encoder.WriteUint8(122); err != nil {
			return err
		}
		return encoder.WriteInt64(a.BigVal)
	case AnyFalse:
		return encoder.WriteUint8(121)
	case AnyTrue:
		return encoder.WriteUint8(120)
	case AnyString:
		if err := encoder.WriteUint8(119); err != nil {
			return err
		}
		return encoder.WriteVarString(&a.StrVal)
	case AnyObject:
		if err := encoder.WriteUint8(118); err != nil {
			return err
		}
		if err := encoder.WriteVarUint64(uint64(len(a.ObjVal))); err != nil {
			return err
		}
		for k, v := range a.ObjVal {
			if err := encoder.WriteVarString(&k); err != nil {
				return err
			}
			if err := WriteAny(encoder, v); err != nil {
				return err
			}
		}
		return nil
	case AnyArray:
		if err := encoder.WriteUint8(117); err != nil {
			return err
		}
		if err := encoder.WriteVarUint64(uint64(len(a.ArrVal))); err != nil {
			return err
		}
		for _, v := range a.ArrVal {
			if err := WriteAny(encoder, v); err != nil {
				return err
			}
		}
		return nil
	case AnyBinary:
		if err := encoder.WriteUint8(116); err != nil {
			return err
		}
		return encoder.WriteVarUint8Array(a.BinVal)
	default:
		return fmt.Errorf("unknown Any tag: %d", a.Tag)
	}
}

func ReadMultipleAny(decoder Decoder) ([]Any, error) {
	length, err := decoder.ReadVarUint()
	if err != nil {
		return nil, err
	}
	result := make([]Any, length)
	for i := uint64(0); i < length; i++ {
		val, err := ReadAny(decoder)
		if err != nil {
			return nil, err
		}
		result[i] = val
	}
	return result, nil
}

func WriteMultipleAny(encoder Encoder, anys []Any) error {
	if err := encoder.WriteVarUint64(uint64(len(anys))); err != nil {
		return err
	}
	for _, a := range anys {
		if err := WriteAny(encoder, a); err != nil {
			return err
		}
	}
	return nil
}
