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

func (u *Update) EncodeV1() ([]uint8, error) {
	enc := NewEncoderV1()
	clients := make([]ClientID, 0, len(u.blocks))
	for c := range u.blocks {
		clients = append(clients, c)
	}
	sort.Slice(clients, func(i, j int) bool { return clients[i] < clients[j] })

	if err := enc.WriteVarUint64(uint64(len(clients))); err != nil {
		return nil, err
	}
	for _, client := range clients {
		nodes := u.blocks[client]
		if err := enc.WriteClient(client); err != nil {
			return nil, err
		}
		if err := enc.WriteLen(uint64(len(nodes))); err != nil {
			return nil, err
		}
		if len(nodes) > 0 {
			if err := enc.WriteVarUint64(nodes[0].Clock()); err != nil {
				return nil, err
			}
			for _, n := range nodes {
				if err := n.WriteNode(&enc); err != nil {
					return nil, err
				}
			}
		}
	}

	if err := u.deleteSet.Encode(&enc); err != nil {
		return nil, err
	}

	return enc.ToBytes(), nil
}

func DecodeUpdateV1(data []uint8) (Update, error) {
	dec := NewDecoderV1(bytes.NewReader(data))
	result := NewUpdate()

	clientCount, err := dec.ReadVarUint()
	if err != nil {
		return result, err
	}

	for i := uint64(0); i < clientCount; i++ {
		client, err := dec.ReadClient()
		if err != nil {
			return result, err
		}
		nodeCount, err := dec.ReadLen()
		if err != nil {
			return result, err
		}
		if nodeCount == 0 {
			continue
		}

		clock, err := dec.ReadVarUint()
		if err != nil {
			return result, err
		}
		for j := uint64(0); j < nodeCount; j++ {
			node, err := ReadNode(&dec, ID{Client: client, Clock: clock})
			if err != nil {
				return result, err
			}
			result.AddNode(node)
			clock += node.NodeLen()
		}
	}

	ds, err := DecodeDeleteSet(&dec)
	if err != nil {
		return result, err
	}
	result.deleteSet = ds

	return result, nil
}
