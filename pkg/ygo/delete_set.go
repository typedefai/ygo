package ygo

import "sort"

type DeleteRange struct {
	Clock uint64
	Len   uint64
}

type DeleteSet struct {
	clients map[ClientID][]DeleteRange
}

func NewDeleteSet() DeleteSet {
	return DeleteSet{clients: make(map[ClientID][]DeleteRange)}
}

func (d *DeleteSet) Add(client ClientID, clock uint64, length uint64) {
	if length == 0 {
		return
	}
	ranges := d.clients[client]
	ranges = append(ranges, DeleteRange{Clock: clock, Len: length})
	d.clients[client] = ranges
}

func (d DeleteSet) Get(client ClientID) []DeleteRange {
	r := d.clients[client]
	out := make([]DeleteRange, len(r))
	copy(out, r)
	return out
}

func (d DeleteSet) Len() int {
	return len(d.clients)
}

func (d DeleteSet) Encode(encoder Encoder) error {
	clients := make([]ClientID, 0, len(d.clients))
	for c := range d.clients {
		clients = append(clients, c)
	}
	sort.Slice(clients, func(i, j int) bool { return clients[i] < clients[j] })

	if err := encoder.WriteVarUint64(uint64(len(clients))); err != nil {
		return err
	}
	for _, client := range clients {
		ranges := d.clients[client]
		if err := encoder.WriteVarUint64(uint64(client)); err != nil {
			return err
		}
		if err := encoder.WriteVarUint64(uint64(len(ranges))); err != nil {
			return err
		}
		for _, r := range ranges {
			if err := encoder.WriteDsClock(r.Clock); err != nil {
				return err
			}
			if err := encoder.WriteDsLen(r.Len); err != nil {
				return err
			}
		}
	}
	return nil
}

func DecodeDeleteSet(decoder Decoder) (DeleteSet, error) {
	result := NewDeleteSet()
	clientCount, err := decoder.ReadVarUint()
	if err != nil {
		return result, err
	}
	for i := uint64(0); i < clientCount; i++ {
		clientRaw, err := decoder.ReadVarUint()
		if err != nil {
			return result, err
		}
		client := ClientID(clientRaw)
		rangeCount, err := decoder.ReadVarUint()
		if err != nil {
			return result, err
		}
		for j := uint64(0); j < rangeCount; j++ {
			clock, err := decoder.ReadDsClock()
			if err != nil {
				return result, err
			}
			length, err := decoder.ReadDsLen()
			if err != nil {
				return result, err
			}
			result.Add(client, clock, length)
		}
	}
	return result, nil
}
