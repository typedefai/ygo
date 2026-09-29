package ygo

import (
	"bytes"
	"sort"

	"riguz.com/ygo/internal/lib0"
)

// DecodeStateVector parses a lib0 state vector: VarUint(count) followed by
// (client, clock) VarUint pairs.
func DecodeStateVector(data []uint8) (StateVector, error) {
	sv := NewStateVector()
	r := lib0.NewBufferRead(bytes.NewReader(data))
	count, err := r.ReadVarUint()
	if err != nil {
		return sv, err
	}
	for i := uint64(0); i < count; i++ {
		client, err := r.ReadVarUint()
		if err != nil {
			return sv, err
		}
		clock, err := r.ReadVarUint()
		if err != nil {
			return sv, err
		}
		sv.SetMax(ClientID(client), clock)
	}
	return sv, nil
}

// EncodeStateVectorV1 encodes a state vector in the canonical layout.
func EncodeStateVectorV1(sv StateVector) ([]uint8, error) {
	enc := NewEncoderV1()
	if err := sv.Encode(&enc); err != nil {
		return nil, err
	}
	return enc.ToBytes(), nil
}

// EncodeStateVectorFromUpdateV1 computes the state vector an update produces
// without applying it (Yjs encodeStateVectorFromUpdate). Clock ranges after a
// gap or Skip do not count.
func EncodeStateVectorFromUpdateV1(update []uint8) (StateVector, error) {
	u, err := DecodeUpdateV1(update)
	if err != nil {
		return NewStateVector(), err
	}
	return stateVectorFromUpdate(u), nil
}

// EncodeStateVectorFromUpdateV2 is the V2 counterpart.
func EncodeStateVectorFromUpdateV2(update []uint8) (StateVector, error) {
	u, err := DecodeUpdateV2(update)
	if err != nil {
		return NewStateVector(), err
	}
	return stateVectorFromUpdate(u), nil
}

func stateVectorFromUpdate(u Update) StateVector {
	sv := NewStateVector()
	for client, nodes := range u.blocks {
		if len(nodes) == 0 {
			continue
		}
		sorted := make([]Node, len(nodes))
		copy(sorted, nodes)
		sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Clock() < sorted[j].Clock() })

		stop := sorted[0].Clock() != 0
		var clock uint64
		for _, n := range sorted {
			if n.IsSkip() {
				stop = true
			}
			if !stop {
				clock = n.Clock() + n.NodeLen()
			}
		}
		if clock != 0 {
			sv.SetMax(client, clock)
		}
	}
	return sv
}
