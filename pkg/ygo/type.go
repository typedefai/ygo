package ygo

// abstractType is the live, mutable container a set of Items integrates into.
// Root types live in Doc.share; nested types are reached through the
// TypeContent that embeds them. Maps use itemMap, sequences use start/length.
type abstractType struct {
	start     *Item
	itemMap   map[string]*Item
	length    uint64
	item      *Item // embedding item; nil for root types
	name      string
	doc       *Doc
	observers []func(*changeEvent)
	pending   []func()
}

// run executes fn immediately for attached/root types; for a detached nested
// type it is queued and flushed when the type is integrated (Yjs _pending).
func (t *abstractType) run(fn func()) {
	if t.item == nil && t.name == "" {
		t.pending = append(t.pending, fn)
		return
	}
	fn()
}

func (t *abstractType) flushPending() {
	for len(t.pending) > 0 {
		fn := t.pending[0]
		t.pending = t.pending[1:]
		fn()
	}
}

func newAbstractType(name string, doc *Doc) *abstractType {
	return &abstractType{itemMap: map[string]*Item{}, name: name, doc: doc}
}

// mapGet returns the current (right-most) item for a map key, if any.
func (t *abstractType) mapGet(key string) *Item {
	if t.itemMap == nil {
		return nil
	}
	return t.itemMap[key]
}

func (t *abstractType) mapSet(key string, item *Item) {
	if t.itemMap == nil {
		t.itemMap = map[string]*Item{}
	}
	t.itemMap[key] = item
}
