package ygo

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// Public shared-type handles. They wrap the live abstractType created for a
// root name or a nested ContentType.

type Text struct{ t *abstractType }
type Map struct{ t *abstractType }
type Array struct{ t *abstractType }
type XmlFragment struct{ t *abstractType }

// ref returns the wire parent reference for items owned by this type.
func (t *abstractType) ref() *Parent {
	if t.item == nil {
		name := t.name
		return &Parent{Named: &name}
	}
	id := t.item.ID
	return &Parent{ID: &id}
}

// GetText resolves (creating on demand) a root text type.
func (d *Doc) GetText(name string) *Text { return &Text{t: d.GetOrCreateType(name)} }

// GetMap resolves a root map type.
func (d *Doc) GetMap(name string) *Map { return &Map{t: d.GetOrCreateType(name)} }

// GetArray resolves a root array type.
func (d *Doc) GetArray(name string) *Array { return &Array{t: d.GetOrCreateType(name)} }

// GetXmlFragment resolves a root XML fragment type.
func (d *Doc) GetXmlFragment(name string) *XmlFragment {
	return &XmlFragment{t: d.GetOrCreateType(name)}
}

// NewMap creates a detached nested map (attach it with a container's Set/Insert).
func (d *Doc) NewMap() *Map { return &Map{t: newAbstractType("", d)} }

// NewArray creates a detached nested array.
func (d *Doc) NewArray() *Array { return &Array{t: newAbstractType("", d)} }

// NewText creates a detached nested text.
func (d *Doc) NewText() *Text { return &Text{t: newAbstractType("", d)} }

// ── observers ────────────────────────────────────────────────────────────────

type changeEvent struct {
	typ  *abstractType
	keys []string
}

// ChangeEvent is delivered to observers once per transaction that touched the
// type. Keys lists changed map keys (empty for sequence changes).
type ChangeEvent struct {
	Target any
	Keys   []string
}

func (t *abstractType) observe(fn func(*changeEvent)) func() {
	t.observers = append(t.observers, fn)
	idx := len(t.observers) - 1
	return func() {
		if idx < len(t.observers) {
			t.observers[idx] = nil
		}
	}
}

func (m *Map) Observe(fn func(ChangeEvent)) func() {
	return m.t.observe(func(ev *changeEvent) {
		fn(ChangeEvent{Target: m, Keys: ev.keys})
	})
}

func (a *Array) Observe(fn func(ChangeEvent)) func() {
	return a.t.observe(func(ev *changeEvent) { fn(ChangeEvent{Target: a}) })
}

func (t *Text) Observe(fn func(ChangeEvent)) func() {
	return t.t.observe(func(ev *changeEvent) { fn(ChangeEvent{Target: t}) })
}

func (x *XmlFragment) Observe(fn func(ChangeEvent)) func() {
	return x.t.observe(func(ev *changeEvent) { fn(ChangeEvent{Target: x}) })
}

// ── reads ────────────────────────────────────────────────────────────────────

// Len returns the number of countable, non-deleted units.
func (t *abstractType) Len() int {
	return int(t.length)
}

func (m *Map) Len() int {
	n := 0
	for _, item := range m.t.itemMap {
		if item != nil && !item.IsDeleted() {
			n++
		}
	}
	return n
}

func (a *Array) Len() int { return a.t.Len() }

func (t *Text) Len() int { return t.t.Len() }

// Get returns the current value for a map key (nil if absent).
func (m *Map) Get(key string) any {
	item := m.t.mapGet(key)
	if item == nil || item.IsDeleted() {
		return nil
	}
	vals := contentToGo(item)
	if len(vals) == 0 {
		return nil
	}
	return vals[0]
}

// Keys returns the map's keys.
func (m *Map) Keys() []string {
	keys := make([]string, 0, len(m.t.itemMap))
	for k, item := range m.t.itemMap {
		if item != nil && !item.IsDeleted() {
			keys = append(keys, k)
		}
	}
	return keys
}

// Entries returns the map's key/value pairs (deleted keys omitted).
func (m *Map) Entries() map[string]any {
	out := make(map[string]any, len(m.t.itemMap))
	for k, item := range m.t.itemMap {
		if item == nil || item.IsDeleted() {
			continue
		}
		out[k] = m.Get(k)
	}
	return out
}

// ToJSON returns a JSON-compatible projection of the map.
func (m *Map) ToJSON() map[string]any { return m.Entries() }

// Get returns the element at index, or nil.
func (a *Array) Get(index int) any {
	slice := a.ToSlice()
	if index < 0 || index >= len(slice) {
		return nil
	}
	return slice[index]
}

// ToSlice returns a JSON-compatible projection of the array.
func (a *Array) ToSlice() []any {
	out := []any{}
	n := a.t.start
	for n != nil {
		if !n.IsDeleted() && n.IsCountable() {
			out = append(out, contentToGo(n)...)
		}
		n = n.Right
	}
	return out
}

func (a *Array) ToJSON() []any { return a.ToSlice() }

// ToString returns the visible text (embeds and formatting contribute nothing).
func (t *Text) ToString() string {
	var b strings.Builder
	n := t.t.start
	for n != nil {
		if !n.IsDeleted() && n.IsCountable() {
			if s, ok := n.Content.(*StringContent); ok {
				b.WriteString(s.Data)
			}
		}
		n = n.Right
	}
	return b.String()
}

func (t *Text) ToJSON() string { return t.ToString() }

// contentToGo flattens an item's content into one or more JSON-compatible Go
// values (an Any run of N values yields N values).
func contentToGo(item *Item) []any {
	switch c := item.Content.(type) {
	case *AnyContent:
		out := make([]any, len(c.Data))
		for i, a := range c.Data {
			out[i] = anyToGo(a)
		}
		return out
	case *JsonContent:
		out := make([]any, len(c.Data))
		for i, a := range c.Data {
			out[i] = anyToGo(a)
		}
		return out
	case *StringContent:
		return []any{c.Data}
	case *BinaryContent:
		return []any{map[string]any{"__bytes": hex.EncodeToString(c.Data)}}
	case *EmbedContent:
		return []any{anyToGo(c.Data)}
	case *TypeContent:
		return []any{typeToGo(c.typ)}
	case *DeletedContent:
		return nil
	default:
		return nil
	}
}

func anyToGo(a Any) any {
	switch a.Tag {
	case AnyUndefined:
		return map[string]any{"__undefined": true}
	case AnyNull:
		return nil
	case AnyTrue:
		return true
	case AnyFalse:
		return false
	case AnyInteger:
		return a.IntVal
	case AnyFloat32:
		// Widen to float64 so JSON formatting matches a JS Number.
		return float64(a.F32Val)
	case AnyFloat64:
		return a.F64Val
	case AnyBigInt64:
		return a.BigVal
	case AnyString:
		return a.StrVal
	case AnyBinary:
		return map[string]any{"__bytes": hex.EncodeToString(a.BinVal)}
	case AnyArray:
		out := make([]any, len(a.ArrVal))
		for i, e := range a.ArrVal {
			out[i] = anyToGo(e)
		}
		return out
	case AnyObject:
		out := make(map[string]any, len(a.ObjVal))
		for k, e := range a.ObjVal {
			out[k] = anyToGo(e)
		}
		return out
	default:
		return nil
	}
}

func typeToGo(t *abstractType) any {
	if t == nil {
		return nil
	}
	if t.item != nil {
		if tc, ok := t.item.Content.(*TypeContent); ok {
			switch tc.TypeRef.Kind {
			case YTypeMap:
				return (&Map{t: t}).ToJSON()
			case YTypeArray:
				return (&Array{t: t}).ToSlice()
			case YTypeText, YTypeXMLText:
				return (&Text{t: t}).ToString()
			case YTypeXMLElement, YTypeXMLFragment:
				return (&XmlFragment{t: t}).ToString()
			}
		}
	}
	return nil
}

// goToAny converts a Go value into an Any payload (map/array element values).
func goToAny(v any) Any {
	switch x := v.(type) {
	case nil:
		return NullAny()
	case bool:
		return BoolAny(x)
	case string:
		return StringAny(x)
	case int:
		return intAny(int64(x))
	case int8:
		return intAny(int64(x))
	case int16:
		return intAny(int64(x))
	case int32:
		return intAny(int64(x))
	case int64:
		return intAny(x)
	case uint:
		return intAny(int64(x))
	case uint8:
		return intAny(int64(x))
	case uint16:
		return intAny(int64(x))
	case uint32:
		return intAny(int64(x))
	case uint64:
		return intAny(int64(x))
	case float32:
		return Float32Any(x)
	case float64:
		if x == float64(int32(x)) && x >= -2147483647 && x <= 2147483647 {
			return IntegerAny(int32(x))
		}
		return Float64Any(x)
	case []byte:
		return BinaryAny(x)
	case []any:
		out := make([]Any, len(x))
		for i, e := range x {
			out[i] = goToAny(e)
		}
		return ArrayAny(out)
	case map[string]any:
		out := make(map[string]Any, len(x))
		for k, e := range x {
			out[k] = goToAny(e)
		}
		return ObjectAny(out)
	default:
		return UndefinedAny()
	}
}

func intAny(v int64) Any {
	if v >= -2147483647 && v <= 2147483647 {
		return IntegerAny(int32(v))
	}
	return BigInt64Any(v)
}

// ── XML ──────────────────────────────────────────────────────────────────────

// ToString renders the fragment as XML (matching Yjs XmlFragment.toString).
func (x *XmlFragment) ToString() string { return xmlToString(x.t, false) }

func (x *XmlFragment) ToJSON() string { return x.ToString() }

func xmlToString(t *abstractType, includeSelf bool) string {
	var b strings.Builder
	if includeSelf && t.item != nil {
		tc, _ := t.item.Content.(*TypeContent)
		if tc != nil && tc.TypeRef.Kind == YTypeXMLElement {
			b.WriteString("<")
			name := ""
			if tc.TypeRef.TagName != nil {
				name = *tc.TypeRef.TagName
			}
			b.WriteString(name)
			// attributes are map entries on the element
			keys := make([]string, 0, len(t.itemMap))
			for k := range t.itemMap {
				keys = append(keys, k)
			}
			sortStrings(keys)
			for _, k := range keys {
				item := t.itemMap[k]
				if item == nil || item.IsDeleted() {
					continue
				}
				vals := contentToGo(item)
				if len(vals) == 0 {
					continue
				}
				fmt.Fprintf(&b, " %s=%q", k, xmlAttrValue(vals[0]))
			}
			b.WriteString(">")
			b.WriteString(xmlChildren(t))
			b.WriteString("</")
			b.WriteString(name)
			b.WriteString(">")
			return b.String()
		}
	}
	b.WriteString(xmlChildren(t))
	return b.String()
}

func xmlChildren(t *abstractType) string {
	var b strings.Builder
	n := t.start
	for n != nil {
		if !n.IsDeleted() && n.IsCountable() {
			switch c := n.Content.(type) {
			case *StringContent:
				b.WriteString(c.Data)
			case *TypeContent:
				if c.typ != nil {
					switch c.TypeRef.Kind {
					case YTypeXMLElement:
						b.WriteString(xmlToString(c.typ, true))
					case YTypeXMLText:
						b.WriteString(xmlChildren(c.typ))
					default:
						b.WriteString(xmlToString(c.typ, false))
					}
				}
			}
		}
		n = n.Right
	}
	return b.String()
}

func xmlAttrValue(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return ""
	default:
		return fmt.Sprint(x)
	}
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// ── local writes ─────────────────────────────────────────────────────────────

func (d *Doc) newItem(parent *abstractType, parentSub *string, content ItemContent) *Item {
	it := &Item{
		ID:         ID{Client: ClientID(d.clientId), Clock: d.store.GetState(ClientID(d.clientId))},
		Parent:     parent.ref(),
		ParentSub:  parentSub,
		Content:    content,
		Info:       NewItemFlags(0),
		parentType: parent,
	}
	if content.IsCountable() {
		it.Info.SetCountable()
	}
	return it
}

// Set sets a map key. Nested types are created with Doc.NewMap/NewArray/NewText.
func (m *Map) Set(key string, value any) {
	m.t.run(func() {
		m.setNow(key, value)
	})
}

func (m *Map) setNow(key string, value any) {
	m.t.doc.Transact(func(txn *Transaction) {
		left := m.t.mapGet(key)
		content := valueContent(value)
		item := m.t.doc.newItem(m.t, &key, content)
		if left != nil {
			last := left.LastId()
			item.Origin = &last
			item.Left = left
		}
		_ = item.integrate(txn, 0)
	}, nil)
}

func (m *Map) Delete(key string) {
	m.t.run(func() {
		m.t.doc.Transact(func(txn *Transaction) {
			if item := m.t.mapGet(key); item != nil {
				item.deleteItem(txn)
			}
		}, nil)
	})
}

func valueContent(value any) ItemContent {
	switch v := value.(type) {
	case *Map:
		return &TypeContent{TypeRef: NewYTypeRef(YTypeMap, nil), typ: v.t}
	case *Array:
		return &TypeContent{TypeRef: NewYTypeRef(YTypeArray, nil), typ: v.t}
	case *Text:
		return &TypeContent{TypeRef: NewYTypeRef(YTypeText, nil), typ: v.t}
	case *XmlFragment:
		return &TypeContent{TypeRef: NewYTypeRef(YTypeXMLFragment, nil), typ: v.t}
	case []byte:
		return &BinaryContent{Data: v}
	default:
		return &AnyContent{Data: []Any{goToAny(value)}}
	}
}

// Insert inserts values at index, packing consecutive primitives into a single
// ContentAny item (Yjs typeListInsertGenerics).
func (a *Array) Insert(index int, values ...any) {
	if len(values) == 0 {
		return
	}
	a.t.run(func() {
		a.t.doc.Transact(func(txn *Transaction) {
			insertGenerics(txn, a.t, uint64(index), values)
		}, nil)
	})
}

func (a *Array) Push(values ...any) {
	a.t.run(func() {
		a.t.doc.Transact(func(txn *Transaction) {
			insertGenerics(txn, a.t, a.t.length, values)
		}, nil)
	})
}

func (a *Array) Delete(index int, length int) {
	if length <= 0 {
		return
	}
	a.t.run(func() {
		a.t.doc.Transact(func(txn *Transaction) {
			deleteRange(txn, a.t, uint64(index), uint64(length))
		}, nil)
	})
}

// Insert inserts plain text at index. Attributes/formatting are not yet
// supported for local writes (applied formats are preserved).
func (t *Text) Insert(index int, text string) {
	if text == "" {
		return
	}
	t.t.run(func() {
		t.t.doc.Transact(func(txn *Transaction) {
			insertString(txn, t.t, uint64(index), text)
		}, nil)
	})
}

func (t *Text) Delete(index int, length int) {
	if length <= 0 {
		return
	}
	t.t.run(func() {
		t.t.doc.Transact(func(txn *Transaction) {
			deleteRange(txn, t.t, uint64(index), uint64(length))
		}, nil)
	})
}

// insertGenerics packs primitives into ContentAny runs, matching
// typeListInsertGenericsAfter.
func insertGenerics(txn *Transaction, parent *abstractType, index uint64, values []any) {
	left := listReference(txn, parent, index)
	right := parent.start
	if left != nil {
		right = left.Right
	}
	doc := txn.doc

	var anyRun []Any
	flushAny := func() {
		if len(anyRun) > 0 {
			left = insertAfter(txn, parent, left, right, &AnyContent{Data: anyRun})
			anyRun = nil
		}
	}
	for _, v := range values {
		switch valueContent(v).(type) {
		case *AnyContent:
			anyRun = append(anyRun, goToAny(v))
		default:
			flushAny()
			left = insertAfter(txn, parent, left, right, valueContent(v))
		}
	}
	flushAny()
	_ = doc
}

func insertAfter(txn *Transaction, parent *abstractType, left, right *Item, content ItemContent) *Item {
	item := txn.doc.newItem(parent, nil, content)
	if left != nil {
		last := left.LastId()
		item.Origin = &last
		item.Left = left
	}
	if right != nil {
		rid := right.ID
		item.RightOrigin = &rid
		item.Right = right
	}
	_ = item.integrate(txn, 0)
	return item
}

func insertString(txn *Transaction, parent *abstractType, index uint64, text string) {
	left := listReference(txn, parent, index)
	right := parent.start
	if left != nil {
		right = left.Right
	}
	insertAfter(txn, parent, left, right, &StringContent{Data: text})
}

// listReference returns the item the insertion at index should follow (nil at
// the start), splitting the item that contains the index if necessary.
func listReference(txn *Transaction, parent *abstractType, index uint64) *Item {
	if index == 0 {
		return nil
	}
	remaining := index
	n := parent.start
	for n != nil {
		if !n.IsDeleted() && n.IsCountable() && n.Len() > 0 {
			if remaining <= n.Len() {
				if remaining < n.Len() {
					if _, err := txn.doc.store.GetItemCleanStart(ID{Client: n.ID.Client, Clock: n.ID.Clock + remaining}); err != nil {
						return nil
					}
				}
				node, err := txn.doc.store.GetItemCleanEnd(ID{Client: n.ID.Client, Clock: n.ID.Clock + remaining - 1})
				if err == nil && node.IsItem() {
					return node.AsItem()
				}
				return nil
			}
			remaining -= n.Len()
		}
		n = n.Right
	}
	return nil
}

// deleteRange deletes length countable units starting at index.
func deleteRange(txn *Transaction, parent *abstractType, index, length uint64) {
	left := listReference(txn, parent, index)
	var item *Item
	if left == nil {
		item = parent.start
	} else {
		item = left.Right
	}
	for length > 0 && item != nil {
		if !item.IsDeleted() && item.IsCountable() {
			if length < item.Len() {
				if _, err := txn.doc.store.GetItemCleanStart(ID{Client: item.ID.Client, Clock: item.ID.Clock + length}); err != nil {
					return
				}
			}
			item.deleteItem(txn)
			length -= item.Len()
		}
		item = item.Right
	}
}

// String is a debug helper.
func (m *Map) String() string { return fmt.Sprint(m.ToJSON()) }

// Int returns v as an int (convenience for tests).
func Int(v any) int {
	switch x := v.(type) {
	case int32:
		return int(x)
	case int64:
		return int(x)
	case float64:
		return int(x)
	case string:
		n, _ := strconv.Atoi(x)
		return n
	default:
		return 0
	}
}
