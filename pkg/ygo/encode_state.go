package ygo

import "sort"

// EncodeStateAsUpdateV1 serialises a document as a V1 update. If sv is
// non-nil, only the structs a peer at that state vector is missing are written
// (Yjs encodeStateAsUpdate).
func EncodeStateAsUpdateV1(doc *Doc, sv *StateVector) ([]uint8, error) {
	enc := NewEncoderV1()
	if err := writeStateAsUpdate(&enc, doc, sv); err != nil {
		return nil, err
	}
	return enc.ToBytes(), nil
}

// EncodeStateAsUpdateV2 is the V2 counterpart.
func EncodeStateAsUpdateV2(doc *Doc, sv *StateVector) ([]uint8, error) {
	enc := NewEncoderV2()
	if err := writeStateAsUpdate(enc, doc, sv); err != nil {
		return nil, err
	}
	return enc.ToBytes()
}

func writeStateAsUpdate(enc Encoder, doc *Doc, sv *StateVector) error {
	if err := writeClientsStructs(enc, doc, sv); err != nil {
		return err
	}
	ds := createDeleteSetFromStructStore(doc.store)
	if doc.pendingDs != nil {
		ds = MergeDeleteSets([]DeleteSet{ds, *doc.pendingDs})
	}
	return ds.Encode(enc)
}

// writeClientsStructs mirrors Yjs writeClientsStructs: clients descending,
// each starting at max(sv[client], firstKnownClock), with the first struct
// written at an offset when the target state vector cuts into it.
func writeClientsStructs(enc Encoder, doc *Doc, sv *StateVector) error {
	clientNodes := map[ClientID][]Node{}
	for client, entry := range doc.store.clients {
		clientNodes[client] = append(clientNodes[client], entry.list...)
	}
	for client, nodes := range doc.pending {
		clientNodes[client] = append(clientNodes[client], nodes...)
	}

	clients := make([]ClientID, 0, len(clientNodes))
	for client, nodes := range clientNodes {
		merged, err := mergeClientNodes(client, nodes)
		if err != nil {
			return err
		}
		if len(merged) == 0 {
			continue
		}
		if sv != nil {
			state := merged[len(merged)-1].Clock() + merged[len(merged)-1].NodeLen()
			if state <= sv.Get(client) {
				continue
			}
		}
		clientNodes[client] = merged
		clients = append(clients, client)
	}
	sort.Slice(clients, func(i, j int) bool { return clients[i] > clients[j] })

	if err := enc.WriteVarUint64(uint64(len(clients))); err != nil {
		return err
	}
	for _, client := range clients {
		nodes := clientNodes[client]
		clock := uint64(0)
		if sv != nil {
			clock = sv.Get(client)
		}
		start := findIndex(nodes, clock)
		if start >= len(nodes) {
			continue
		}
		if nodes[start].Clock() > clock {
			clock = nodes[start].Clock()
		}
		if err := enc.WriteVarUint64(uint64(len(nodes) - start)); err != nil {
			return err
		}
		if err := enc.WriteClient(client); err != nil {
			return err
		}
		if err := enc.WriteVarUint64(clock); err != nil {
			return err
		}
		for idx := start; idx < len(nodes); idx++ {
			offset := uint64(0)
			if idx == start {
				offset = clock - nodes[idx].Clock()
			}
			if err := nodes[idx].WriteNode(enc, offset); err != nil {
				return err
			}
		}
	}
	return nil
}
