package ygo

// Content operation methods used by Item integration. Kept separate from the
// type definitions so the wire codec in block.go stays readable.

// ── DeletedContent ───────────────────────────────────────────────────────────

func (c *DeletedContent) Splice(offset uint64) ItemContent {
	return &DeletedContent{Len: c.Len - offset}
}
func (c *DeletedContent) Integrate(_ *Transaction, _ *Item) {}
func (c *DeletedContent) Delete(_ *Transaction)             {}
func (c *DeletedContent) MergeWith(_ ItemContent) bool      { return false }

// ── JsonContent ──────────────────────────────────────────────────────────────

func (c *JsonContent) Splice(offset uint64) ItemContent {
	right := make([]Any, uint64(len(c.Data))-offset)
	copy(right, c.Data[offset:])
	return &JsonContent{Data: right}
}
func (c *JsonContent) Integrate(_ *Transaction, _ *Item) {}
func (c *JsonContent) Delete(_ *Transaction)             {}
func (c *JsonContent) MergeWith(right ItemContent) bool {
	r, ok := right.(*JsonContent)
	if !ok {
		return false
	}
	c.Data = append(c.Data, r.Data...)
	return true
}

// ── BinaryContent ────────────────────────────────────────────────────────────

func (c *BinaryContent) Splice(_ uint64) ItemContent       { return c }
func (c *BinaryContent) Integrate(_ *Transaction, _ *Item) {}
func (c *BinaryContent) Delete(_ *Transaction)             {}
func (c *BinaryContent) MergeWith(_ ItemContent) bool      { return false }

// ── StringContent ────────────────────────────────────────────────────────────

func (c *StringContent) Splice(offset uint64) ItemContent {
	_, right := splitAsUtf16Str(c.Data, offset)
	return &StringContent{Data: right}
}
func (c *StringContent) Integrate(_ *Transaction, _ *Item) {}
func (c *StringContent) Delete(_ *Transaction)             {}
func (c *StringContent) MergeWith(right ItemContent) bool {
	r, ok := right.(*StringContent)
	if !ok {
		return false
	}
	c.Data += r.Data
	return true
}

// ── EmbedContent ─────────────────────────────────────────────────────────────

func (c *EmbedContent) Splice(_ uint64) ItemContent       { return c }
func (c *EmbedContent) Integrate(_ *Transaction, _ *Item) {}
func (c *EmbedContent) Delete(_ *Transaction)             {}
func (c *EmbedContent) MergeWith(_ ItemContent) bool      { return false }

// ── FormatContent ────────────────────────────────────────────────────────────

func (c *FormatContent) Splice(_ uint64) ItemContent       { return c }
func (c *FormatContent) Integrate(_ *Transaction, _ *Item) {}
func (c *FormatContent) Delete(_ *Transaction)             {}
func (c *FormatContent) MergeWith(_ ItemContent) bool      { return false }

// ── TypeContent ──────────────────────────────────────────────────────────────

func (c *TypeContent) Splice(_ uint64) ItemContent { return c }

// Integrate binds the live nested type to this content. Items belonging to the
// nested type are integrated later, resolving their parent through this object.
func (c *TypeContent) Integrate(txn *Transaction, item *Item) {
	if c.typ == nil {
		c.typ = &abstractType{
			item:    item,
			doc:     txn.doc,
			itemMap: map[string]*Item{},
		}
		return
	}
	c.typ.item = item
	c.typ.doc = txn.doc
	c.typ.flushPending()
}

func (c *TypeContent) Delete(_ *Transaction) {}
func (c *TypeContent) MergeWith(_ ItemContent) bool {
	return false
}

// ── AnyContent ───────────────────────────────────────────────────────────────

func (c *AnyContent) Splice(offset uint64) ItemContent {
	right := make([]Any, uint64(len(c.Data))-offset)
	copy(right, c.Data[offset:])
	return &AnyContent{Data: right}
}
func (c *AnyContent) Integrate(_ *Transaction, _ *Item) {}
func (c *AnyContent) Delete(_ *Transaction)             {}
func (c *AnyContent) MergeWith(right ItemContent) bool {
	r, ok := right.(*AnyContent)
	if !ok {
		return false
	}
	c.Data = append(c.Data, r.Data...)
	return true
}

// ── DocContent ───────────────────────────────────────────────────────────────

func (c *DocContent) Splice(_ uint64) ItemContent       { return c }
func (c *DocContent) Integrate(_ *Transaction, _ *Item) {}
func (c *DocContent) Delete(_ *Transaction)             {}
func (c *DocContent) MergeWith(_ ItemContent) bool      { return false }

// ── MoveContent ──────────────────────────────────────────────────────────────

func (c *MoveContent) Splice(_ uint64) ItemContent       { return c }
func (c *MoveContent) Integrate(_ *Transaction, _ *Item) {}
func (c *MoveContent) Delete(_ *Transaction)             {}
func (c *MoveContent) MergeWith(_ ItemContent) bool      { return false }
