package ygo

import "sort"

// Transaction batches mutations. Cleanup (GC, delete-set merge, struct
// squashing, observers) runs when the outermost transaction exits, mirroring
// Yjs.
type Transaction struct {
	doc         *Doc
	origin      any
	local       bool
	beforeState StateVector
	afterState  StateVector
	deleteSet   DeleteSet
	changed     map[*abstractType]map[string]struct{}
}

func (t *Transaction) Doc() *Doc                { return t.doc }
func (t *Transaction) Origin() any              { return t.origin }
func (t *Transaction) Local() bool              { return t.local }
func (t *Transaction) BeforeState() StateVector { return t.beforeState }
func (t *Transaction) AfterState() StateVector  { return t.afterState }
func (t *Transaction) DeleteSet() DeleteSet     { return t.deleteSet }

func (t *Transaction) addChanged(typ *abstractType, parentSub *string) {
	if typ == nil {
		return
	}
	if t.changed == nil {
		t.changed = map[*abstractType]map[string]struct{}{}
	}
	key := ""
	if parentSub != nil {
		key = *parentSub
	}
	m, ok := t.changed[typ]
	if !ok {
		m = map[string]struct{}{}
		t.changed[typ] = m
	}
	m[key] = struct{}{}
}

// Transact runs fn in a transaction. Nested calls reuse the open transaction
// (ambient nesting, as in Yjs). origin is stored on the transaction.
func (d *Doc) Transact(fn func(*Transaction), origin any) {
	if d.transaction != nil {
		fn(d.transaction)
		return
	}
	txn := &Transaction{
		doc:         d,
		origin:      origin,
		local:       true,
		beforeState: d.store.GetStateVector(),
		deleteSet:   NewDeleteSet(),
		changed:     map[*abstractType]map[string]struct{}{},
	}
	d.transaction = txn
	defer func() { d.transaction = nil }()
	fn(txn)
	txn.afterState = d.store.GetStateVector()
	d.cleanupTransaction(txn)
}

// CurrentTransaction returns the open transaction, if any.
func (d *Doc) CurrentTransaction() *Transaction { return d.transaction }

func (d *Doc) cleanupTransaction(txn *Transaction) {
	if d.gc {
		d.tryGcDeleteSet(txn.deleteSet)
	}
	d.tryMergeDeleteSet(txn.deleteSet)

	for _, client := range txn.afterState.ClientIDs() {
		after := txn.afterState.Get(client)
		before := txn.beforeState.Get(client)
		if before == after {
			continue
		}
		entry := d.store.clients[client]
		if len(entry.list) == 0 {
			continue
		}
		first := findIndex(entry.list, before)
		if first < 1 {
			first = 1
		}
		for i := len(entry.list) - 1; i >= first; {
			i -= 1 + tryToMergeWithLefts(&entry.list, i)
		}
		d.store.clients[client] = entry
	}

	d.fireObservers(txn)
}

// fireObservers notifies registered type observers once per transaction, in a
// deterministic order.
func (d *Doc) fireObservers(txn *Transaction) {
	if len(txn.changed) == 0 {
		return
	}
	types := make([]*abstractType, 0, len(txn.changed))
	for t := range txn.changed {
		types = append(types, t)
	}
	sort.Slice(types, func(i, j int) bool {
		if types[i].name != types[j].name {
			return types[i].name < types[j].name
		}
		if types[i].item == nil || types[j].item == nil {
			return types[i].item == nil
		}
		return types[i].item.ID.Clock < types[j].item.ID.Clock
	})
	for _, t := range types {
		if len(t.observers) == 0 {
			continue
		}
		keys := make([]string, 0, len(txn.changed[t]))
		for k := range txn.changed[t] {
			if k != "" {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		ev := &changeEvent{typ: t, keys: keys}
		for _, fn := range t.observers {
			if fn != nil {
				fn(ev)
			}
		}
	}
}

// tryGcDeleteSet replaces the content of freshly deleted items with
// ContentDeleted tombstones (Yjs Item.gc).
func (d *Doc) tryGcDeleteSet(ds DeleteSet) {
	for client, ranges := range ds.clients {
		entry, ok := d.store.clients[client]
		if !ok {
			continue
		}
		for di := len(ranges) - 1; di >= 0; di-- {
			r := ranges[di]
			end := r.Clock + r.Len
			for i := findIndex(entry.list, r.Clock); i < len(entry.list) && entry.list[i].Clock() < end; i++ {
				node := entry.list[i]
				if !node.IsItem() {
					continue
				}
				item := node.AsItem()
				if item.IsDeleted() && !item.Info.IsKeep() {
					item.gc()
				}
			}
		}
		d.store.clients[client] = entry
	}
}

// tryMergeDeleteSet compacts adjacent deleted structs inside delete ranges
// (Yjs tryMergeDeleteSet).
func (d *Doc) tryMergeDeleteSet(ds DeleteSet) {
	for client, ranges := range ds.clients {
		entry, ok := d.store.clients[client]
		if !ok {
			continue
		}
		for di := len(ranges) - 1; di >= 0; di-- {
			r := ranges[di]
			start := findIndex(entry.list, r.Clock+r.Len-1)
			if start >= len(entry.list) {
				start = len(entry.list) - 1
			}
			for si := start; si > 0 && entry.list[si].Clock() >= r.Clock; si-- {
				si -= tryToMergeWithLefts(&entry.list, si)
				if si <= 0 {
					break
				}
			}
		}
		d.store.clients[client] = entry
	}
}

// findIndex returns the first index whose node ends after clock.
func findIndex(list []Node, clock uint64) int {
	lo, hi := 0, len(list)
	for lo < hi {
		mid := (lo + hi) / 2
		if list[mid].Clock()+list[mid].NodeLen() > clock {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return lo
}

// tryToMergeWithLefts merges list[pos] into its left neighbours while possible,
// returning the number of merged structs (Yjs tryToMergeWithLefts).
func tryToMergeWithLefts(list *[]Node, pos int) int {
	s := *list
	i := pos
	for i > 0 {
		right := s[i]
		left := s[i-1]
		if !nodesMergeable(left, right) {
			break
		}
		if !mergeNodeInto(&left, right) {
			break
		}
		s[i-1] = left
		i--
	}
	merged := pos - i
	if merged > 0 && pos+1 <= len(s) {
		s = append(s[:pos+1-merged], s[pos+1:]...)
		*list = s
	}
	return merged
}

func nodesMergeable(a, b Node) bool {
	if a.IsItem() != b.IsItem() {
		return false
	}
	if a.IsItem() {
		return a.AsItem().IsDeleted() == b.AsItem().IsDeleted()
	}
	// GC and Skip never merge with each other.
	return a.IsGC() == b.IsGC()
}

func mergeNodeInto(left *Node, right Node) bool {
	switch {
	case left.IsGC() && right.IsGC():
		*left = NewGCNode(left.NodeID(), left.NodeLen()+right.NodeLen())
		return true
	case left.IsSkip() && right.IsSkip():
		*left = NewSkipNode(left.NodeID(), left.NodeLen()+right.NodeLen())
		return true
	case left.IsItem() && right.IsItem():
		return left.AsItem().mergeWith(right.AsItem())
	default:
		return false
	}
}
