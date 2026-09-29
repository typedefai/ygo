package ygo

import (
	"bytes"
	"sort"
)

type Update struct {
	blocks    map[ClientID][]Node
	deleteSet DeleteSet
}

func NewUpdate() Update {
	return Update{
		blocks:    make(map[ClientID][]Node),
		deleteSet: NewDeleteSet(),
	}
}

func (u *Update) AddNode(node Node) {
	u.blocks[node.Client()] = append(u.blocks[node.Client()], node)
}

func (u *Update) AddDelete(client ClientID, clock uint64, length uint64) {
	u.deleteSet.Add(client, clock, length)
}

func (u *Update) Blocks(client ClientID) []Node {
	list := u.blocks[client]
	out := make([]Node, len(list))
	copy(out, list)
	return out
}

func (u *Update) DeleteSet() DeleteSet {
	return u.deleteSet
}

func (u *Update) StateVector() StateVector {
	sv := NewStateVector()
	for client, list := range u.blocks {
		if len(list) == 0 {
			continue
		}
		last := list[len(list)-1]
		sv.SetMax(client, last.Clock()+last.NodeLen())
	}
	return sv
}

// encode writes the update using the version-agnostic Encoder. The logical
// call order mirrors Yjs writeClientsStructs/writeStructs:
//
//	VarUint(numClients)
//	  VarUint(numStructs) client VarUint(clock) structs...   (per client)
//	deleteSet
//
// The per-field routing to columns (V2) vs the single stream (V1) is handled by
// the Encoder implementation.
func (u *Update) encode(enc Encoder) error {
	clients := make([]ClientID, 0, len(u.blocks))
	for c := range u.blocks {
		clients = append(clients, c)
	}
	// Yjs sorts descending ("higher client ids first ... heavily improves the
	// conflict algorithm").
	sort.Slice(clients, func(i, j int) bool { return clients[i] > clients[j] })

	if err := enc.WriteVarUint64(uint64(len(clients))); err != nil {
		return err
	}
	for _, client := range clients {
		nodes := u.blocks[client]
		if err := enc.WriteVarUint64(uint64(len(nodes))); err != nil {
			return err
		}
		if err := enc.WriteClient(client); err != nil {
			return err
		}
		if len(nodes) == 0 {
			return NewIncompleteDocumentError("empty client struct block")
		}
		if err := enc.WriteVarUint64(nodes[0].Clock()); err != nil {
			return err
		}
		for _, n := range nodes {
			if err := n.WriteNode(enc, 0); err != nil {
				return err
			}
		}
	}
	return u.deleteSet.Encode(enc)
}

func (u *Update) EncodeV1() ([]uint8, error) {
	enc := NewEncoderV1()
	if err := u.encode(&enc); err != nil {
		return nil, err
	}
	return enc.ToBytes(), nil
}

func (u *Update) EncodeV2() ([]uint8, error) {
	enc := NewEncoderV2()
	if err := u.encode(enc); err != nil {
		return nil, err
	}
	return enc.ToBytes()
}

// UpdateV1ToV2 re-encodes a V1 update in the V2 wire format without applying
// it to a document (equivalent to Yjs convertUpdateFormatV1ToV2).
func UpdateV1ToV2(v1 []uint8) ([]uint8, error) {
	u, err := DecodeUpdateV1(v1)
	if err != nil {
		return nil, err
	}
	return u.EncodeV2()
}

// UpdateV2ToV1 re-encodes a V2 update in the V1 wire format without applying
// it to a document (equivalent to Yjs convertUpdateFormatV2ToV1).
func UpdateV2ToV1(v2 []uint8) ([]uint8, error) {
	u, err := DecodeUpdateV2(v2)
	if err != nil {
		return nil, err
	}
	return u.EncodeV1()
}

func DecodeUpdateV1(data []uint8) (Update, error) {
	dec := NewDecoderV1(bytes.NewReader(data))
	return decodeUpdate(&dec)
}

func DecodeUpdateV2(data []uint8) (Update, error) {
	dec, err := NewDecoderV2(data)
	if err != nil {
		return NewUpdate(), err
	}
	return decodeUpdate(dec)
}

func decodeUpdate(dec Decoder) (Update, error) {
	result := NewUpdate()

	clientCount, err := dec.ReadVarUint()
	if err != nil {
		return result, err
	}
	for i := uint64(0); i < clientCount; i++ {
		// Yjs writeStructs emits numberOfStructs, client, clock in that order
		// (src/utils/encoding.js), with clients ordered descending.
		nodeCount, err := dec.ReadVarUint()
		if err != nil {
			return result, err
		}
		client, err := dec.ReadClient()
		if err != nil {
			return result, err
		}
		clock, err := dec.ReadVarUint()
		if err != nil {
			return result, err
		}
		if nodeCount == 0 {
			continue
		}
		for j := uint64(0); j < nodeCount; j++ {
			node, err := ReadNode(dec, ID{Client: client, Clock: clock})
			if err != nil {
				return result, err
			}
			result.AddNode(node)
			clock += node.NodeLen()
		}
	}

	ds, err := DecodeDeleteSet(dec)
	if err != nil {
		return result, err
	}
	result.deleteSet = ds

	return result, nil
}
