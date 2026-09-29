package ygo

import "sort"

// integrateStructs integrates a decoded update's structs into the store,
// honouring client ordering and cross-client dependencies. Structs that cannot
// integrate yet are returned so the caller can retry them on a later apply.
func integrateStructs(txn *Transaction, blocks map[ClientID][]Node) ([]Node, error) {
	type queue struct {
		nodes []Node
		i     int
	}
	queues := map[ClientID]*queue{}
	ids := make([]ClientID, 0, len(blocks))
	for client, ns := range blocks {
		sorted := make([]Node, len(ns))
		copy(sorted, ns)
		sort.SliceStable(sorted, func(a, b int) bool { return sorted[a].Clock() < sorted[b].Clock() })
		queues[client] = &queue{nodes: sorted}
		ids = append(ids, client)
	}
	sort.Slice(ids, func(a, b int) bool { return ids[a] < ids[b] }) // highest processed first

	var pending []Node
	parked := func(idx int) {
		q := queues[ids[idx]]
		pending = append(pending, q.nodes[q.i:]...)
		ids = append(ids[:idx], ids[idx+1:]...)
	}

	for len(ids) > 0 {
		progressed := false
		for idx := len(ids) - 1; idx >= 0; idx-- {
			client := ids[idx]
			q := queues[client]
			if q.i >= len(q.nodes) {
				ids = append(ids[:idx], ids[idx+1:]...)
				continue
			}
			node := q.nodes[q.i]
			if node.IsSkip() {
				q.i++
				progressed = true
				continue
			}
			state := txn.doc.store.GetState(client)
			if node.Clock() > state {
				parked(idx) // own-client gap
				continue
			}
			if node.Clock()+node.NodeLen() <= state {
				q.i++
				progressed = true
				continue
			}
			if dep, ok := nodeMissingDep(txn, node); ok {
				if dq, exists := queues[dep]; exists && dq.i < len(dq.nodes) {
					continue // process the dependency's client first
				}
				parked(idx)
				continue
			}
			if err := integrateNode(txn, node, state-node.Clock()); err != nil {
				return pending, err
			}
			q.i++
			progressed = true
		}
		if !progressed {
			for _, c := range append([]ClientID(nil), ids...) {
				idx := indexOfID(ids, c)
				if idx >= 0 {
					parked(idx)
				}
			}
			break
		}
	}
	return pending, nil
}

// nodeMissingDep reports a cross-client dependency whose client has not been
// integrated far enough yet.
func nodeMissingDep(txn *Transaction, node Node) (ClientID, bool) {
	if !node.IsItem() {
		return 0, false
	}
	i := node.AsItem()
	store := txn.doc.store
	if i.Origin != nil && i.Origin.Client != i.ID.Client && i.Origin.Clock >= store.GetState(i.Origin.Client) {
		return i.Origin.Client, true
	}
	if i.RightOrigin != nil && i.RightOrigin.Client != i.ID.Client && i.RightOrigin.Clock >= store.GetState(i.RightOrigin.Client) {
		return i.RightOrigin.Client, true
	}
	if i.Parent != nil && i.Parent.ID != nil && i.ID.Client != i.Parent.ID.Client && i.Parent.ID.Clock >= store.GetState(i.Parent.ID.Client) {
		return i.Parent.ID.Client, true
	}
	return 0, false
}

func integrateNode(txn *Transaction, node Node, offset uint64) error {
	if node.IsGC() {
		if offset == 0 {
			txn.doc.store.Append(node)
		} else {
			txn.doc.store.Append(NewGCNode(ID{Client: node.Client(), Clock: node.Clock() + offset}, node.NodeLen()-offset))
		}
		return nil
	}
	item := node.AsItem()
	if _, hasMissing, err := item.resolveDeps(txn); err != nil {
		return err
	} else if hasMissing {
		return NewIncompleteDocumentError("unresolved dependency during integration")
	}
	return item.integrate(txn, offset)
}

// applyDeleteSet marks the deleted ranges, returning ranges that target
// not-yet-integrated structs (Yjs readAndApplyDeleteSet).
func applyDeleteSet(txn *Transaction, ds DeleteSet) (*DeleteSet, error) {
	store := txn.doc.store
	unapplied := NewDeleteSet()
	clients := make([]ClientID, 0, len(ds.clients))
	for c := range ds.clients {
		clients = append(clients, c)
	}
	sort.Slice(clients, func(a, b int) bool { return clients[a] < clients[b] })

	for _, client := range clients {
		ranges := ds.clients[client]
		state := store.GetState(client)
		for _, r := range ranges {
			clock, clockEnd := r.Clock, r.Clock+r.Len
			if clock >= state {
				unapplied.Add(client, clock, clockEnd-clock)
				continue
			}
			if state < clockEnd {
				unapplied.Add(client, state, clockEnd-state)
			}
			entry, ok := store.clients[client]
			if !ok {
				continue
			}
			i := findIndex(entry.list, clock)
			if i >= len(entry.list) {
				continue
			}
			node := entry.list[i]
			if node.Clock() < clock && node.IsItem() && !node.AsItem().IsDeleted() {
				if _, err := splitNodeAt(&entry.list, i, clock-node.Clock()); err != nil {
					return nil, err
				}
				i++
			}
			for ; i < len(entry.list); i++ {
				node = entry.list[i]
				if node.Clock() >= clockEnd {
					break
				}
				if !node.IsItem() {
					continue
				}
				item := node.AsItem()
				if item.IsDeleted() {
					continue
				}
				if clockEnd < item.ID.Clock+item.Len() {
					if _, err := splitNodeAt(&entry.list, i, clockEnd-item.ID.Clock); err != nil {
						return nil, err
					}
					item = entry.list[i].AsItem()
				}
				item.deleteItem(txn)
			}
			store.clients[client] = entry
		}
	}
	if unapplied.Len() > 0 {
		return &unapplied, nil
	}
	return nil, nil
}

// createDeleteSetFromStructStore derives the delete set from the current store
// (Yjs createDeleteSetFromStructStore).
func createDeleteSetFromStructStore(store *BlockStore) DeleteSet {
	ds := NewDeleteSet()
	for client, entry := range store.clients {
		var items []DeleteRange
		for i := 0; i < len(entry.list); i++ {
			node := entry.list[i]
			if !node.IsDeleted() {
				continue
			}
			clock := node.Clock()
			length := node.NodeLen()
			for i+1 < len(entry.list) && entry.list[i+1].IsDeleted() {
				i++
				length += entry.list[i].NodeLen()
			}
			items = append(items, DeleteRange{Clock: clock, Len: length})
		}
		if len(items) > 0 {
			ds.clients[client] = items
		}
	}
	return ds
}

func indexOfID(ids []ClientID, id ClientID) int {
	for i, c := range ids {
		if c == id {
			return i
		}
	}
	return -1
}
