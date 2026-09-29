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
		// Each client's ranges are delta-encoded from zero (V2); V1 ignores it.
		encoder.ResetDsCurVal()
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

// SortAndMerge normalises every client's ranges: sorted by clock, with
// overlapping or adjacent ranges coalesced (Yjs sortAndMergeDeleteSet).
func (d *DeleteSet) SortAndMerge() {
	for client, ranges := range d.clients {
		sort.Slice(ranges, func(i, j int) bool { return ranges[i].Clock < ranges[j].Clock })
		j := 0
		for i := 1; i < len(ranges); i++ {
			left := &ranges[j]
			right := ranges[i]
			if left.Clock+left.Len >= right.Clock {
				if end := right.Clock + right.Len; end > left.Clock+left.Len {
					left.Len = end - left.Clock
				}
			} else {
				j++
				ranges[j] = right
			}
		}
		if len(ranges) > 0 {
			ranges = ranges[:j+1]
		} else {
			ranges = ranges[:0]
		}
		d.clients[client] = ranges
	}
}

// MergeDeleteSets unions several delete sets and normalises the result.
func MergeDeleteSets(sets []DeleteSet) DeleteSet {
	merged := NewDeleteSet()
	for _, ds := range sets {
		for client, ranges := range ds.clients {
			merged.clients[client] = append(merged.clients[client], ranges...)
		}
	}
	merged.SortAndMerge()
	return merged
}

func DecodeDeleteSet(decoder Decoder) (DeleteSet, error) {
	result := NewDeleteSet()
	clientCount, err := decoder.ReadVarUint()
	if err != nil {
		return result, err
	}
	for i := uint64(0); i < clientCount; i++ {
		// Ranges are delta-encoded per client (V2); V1 ignores it.
		decoder.ResetDsCurVal()
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
