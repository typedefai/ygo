package ygo

import "sort"

// MergeUpdatesV1 merges several V1 updates into one without applying them to a
// document. Equivalent to Yjs mergeUpdates: overlapping structs are
// deduplicated, clock gaps become Skip structs, adjacent GC/Skip structs are
// coalesced, and delete sets are unioned.
func MergeUpdatesV1(updates [][]uint8) ([]uint8, error) {
	if len(updates) == 1 {
		return updates[0], nil
	}
	us := make([]Update, 0, len(updates))
	for _, raw := range updates {
		u, err := DecodeUpdateV1(raw)
		if err != nil {
			return nil, err
		}
		us = append(us, u)
	}
	merged, err := mergeUpdates(us)
	if err != nil {
		return nil, err
	}
	return merged.EncodeV1()
}

// MergeUpdatesV2 is the V2 counterpart of MergeUpdatesV1.
func MergeUpdatesV2(updates [][]uint8) ([]uint8, error) {
	if len(updates) == 1 {
		return updates[0], nil
	}
	us := make([]Update, 0, len(updates))
	for _, raw := range updates {
		u, err := DecodeUpdateV2(raw)
		if err != nil {
			return nil, err
		}
		us = append(us, u)
	}
	merged, err := mergeUpdates(us)
	if err != nil {
		return nil, err
	}
	return merged.EncodeV2()
}

func mergeUpdates(us []Update) (Update, error) {
	result := NewUpdate()

	clientNodes := make(map[ClientID][]Node)
	for _, u := range us {
		for client, nodes := range u.blocks {
			clientNodes[client] = append(clientNodes[client], nodes...)
		}
	}
	for client, nodes := range clientNodes {
		merged, err := mergeClientNodes(client, nodes)
		if err != nil {
			return result, err
		}
		if len(merged) > 0 {
			result.blocks[client] = merged
		}
	}

	sets := make([]DeleteSet, 0, len(us))
	for _, u := range us {
		sets = append(sets, u.deleteSet)
	}
	result.deleteSet = MergeDeleteSets(sets)

	return result, nil
}

// mergeClientNodes builds a canonical, gap-free, non-overlapping block list for
// one client. It mirrors Yjs mergeUpdatesV2's struct stream merge.
func mergeClientNodes(client ClientID, nodes []Node) ([]Node, error) {
	sorted := make([]Node, len(nodes))
	copy(sorted, nodes)
	sort.SliceStable(sorted, func(i, j int) bool {
		ci, cj := sorted[i].Clock(), sorted[j].Clock()
		if ci != cj {
			return ci < cj
		}
		// Non-Skip wins at the same clock (Yjs comparator).
		return nodePriority(sorted[i]) < nodePriority(sorted[j])
	})

	out := make([]Node, 0, len(sorted))
	var end uint64
	for _, n := range sorted {
		start := n.Clock()
		length := n.NodeLen()
		if length == 0 {
			continue
		}
		if start < end {
			covered := end - start
			if covered >= length {
				continue // fully covered by an earlier struct
			}
			var err error
			n, err = sliceNode(n, covered)
			if err != nil {
				return nil, err
			}
			start = end
			length = n.NodeLen()
		}
		if start > end {
			// Fill the gap with a Skip, extending a trailing one if present.
			if len(out) > 0 && out[len(out)-1].IsSkip() {
				prev := out[len(out)-1]
				out[len(out)-1] = NewSkipNode(prev.NodeID(), prev.NodeLen()+(start-end))
			} else {
				out = append(out, NewSkipNode(ID{Client: client, Clock: end}, start-end))
			}
		}
		if len(out) > 0 && canMergeNodes(out[len(out)-1], n) {
			last := out[len(out)-1]
			out[len(out)-1] = mergeNodes(last, n)
		} else {
			out = append(out, n)
		}
		end = start + length
	}
	return out, nil
}

// canMergeNodes mirrors the struct-level mergeWith gate: only GC and Skip merge
// here. Item.mergeWith additionally requires linked-list adjacency, which
// decoded structs do not have — Yjs mergeUpdates does not merge Items either.
func canMergeNodes(a, b Node) bool {
	if a.IsItem() || b.IsItem() {
		return false
	}
	if a.IsGC() != b.IsGC() {
		return false
	}
	return a.Clock()+a.NodeLen() == b.Clock()
}

func mergeNodes(a, b Node) Node {
	if a.IsGC() {
		return NewGCNode(a.NodeID(), a.NodeLen()+b.NodeLen())
	}
	return NewSkipNode(a.NodeID(), a.NodeLen()+b.NodeLen())
}

func nodePriority(n Node) int {
	switch {
	case n.IsItem():
		return 0
	case n.IsGC():
		return 1
	default:
		return 2
	}
}

// sliceNode returns the right part of n starting at offset diff (Yjs
// sliceStruct).
func sliceNode(n Node, diff uint64) (Node, error) {
	switch {
	case n.IsGC():
		return NewGCNode(ID{Client: n.Client(), Clock: n.Clock() + diff}, n.NodeLen()-diff), nil
	case n.IsSkip():
		return NewSkipNode(ID{Client: n.Client(), Clock: n.Clock() + diff}, n.NodeLen()-diff), nil
	default:
		right, err := sliceItem(n.item, diff)
		if err != nil {
			return Node{}, err
		}
		return NewItemNode(right), nil
	}
}

// sliceItem returns a new Item covering [clock+diff, end) without mutating the
// original (Yjs sliceStruct).
func sliceItem(i *Item, diff uint64) (*Item, error) {
	_, rightContent, err := i.Content.Split(diff)
	if err != nil {
		return nil, err
	}
	return &Item{
		ID:          ID{Client: i.ID.Client, Clock: i.ID.Clock + diff},
		Origin:      &ID{Client: i.ID.Client, Clock: i.ID.Clock + diff - 1},
		RightOrigin: i.RightOrigin,
		Parent:      i.Parent,
		ParentSub:   i.ParentSub,
		Content:     rightContent,
		Info:        i.Info,
		parentType:  i.parentType,
	}, nil
}
