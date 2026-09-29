package ygo

import (
	"reflect"
)

func sameID(a, b *ID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Client == b.Client && a.Clock == b.Clock
}

// getItem returns the Item starting exactly at id, or nil for GC/Skip/missing.
func getItem(store *BlockStore, id ID) *Item {
	node, ok := store.Find(id)
	if !ok || !node.IsItem() {
		return nil
	}
	return node.AsItem()
}

// resolveDeps resolves Origin/RightOrigin to live neighbours, links them, and
// determines the parent type. It mirrors Yjs Item.getMissing: it returns the
// client of an unavailable cross-client dependency, or resolves everything and
// returns hasMissing=false.
func (i *Item) resolveDeps(txn *Transaction) (ClientID, bool, error) {
	store := txn.doc.store
	if i.Origin != nil && i.Origin.Client != i.ID.Client && i.Origin.Clock >= store.GetState(i.Origin.Client) {
		return i.Origin.Client, true, nil
	}
	if i.RightOrigin != nil && i.RightOrigin.Client != i.ID.Client && i.RightOrigin.Clock >= store.GetState(i.RightOrigin.Client) {
		return i.RightOrigin.Client, true, nil
	}
	if i.Parent != nil && i.Parent.ID != nil && i.ID.Client != i.Parent.ID.Client && i.Parent.ID.Clock >= store.GetState(i.Parent.ID.Client) {
		return i.Parent.ID.Client, true, nil
	}

	leftGC, rightGC := false, false
	if i.Origin != nil {
		ln, err := store.GetItemCleanEnd(*i.Origin)
		if err != nil {
			return 0, false, err
		}
		last := ID{Client: ln.Client(), Clock: ln.Clock() + ln.NodeLen() - 1}
		if ln.IsItem() {
			i.Left = ln.AsItem()
		} else {
			leftGC = true
		}
		i.Origin = &last
	}
	if i.RightOrigin != nil {
		rn, err := store.GetItemCleanStart(*i.RightOrigin)
		if err != nil {
			return 0, false, err
		}
		rid := rn.NodeID()
		if rn.IsItem() {
			i.Right = rn.AsItem()
		} else {
			rightGC = true
		}
		i.RightOrigin = &rid
	}

	if leftGC || rightGC {
		i.parentType = nil
	} else if i.Parent != nil && i.Parent.Named != nil {
		i.parentType = txn.doc.GetOrCreateType(*i.Parent.Named)
	} else if i.Parent != nil && i.Parent.ID != nil {
		parentNode, _ := store.Find(*i.Parent.ID)
		if parentNode.IsItem() {
			if tc, ok := parentNode.AsItem().Content.(*TypeContent); ok {
				i.parentType = tc.typ
			}
		}
		if i.parentType == nil {
			return 0, false, nil // parent GC'd: integrate as GC
		}
	}

	if i.parentType == nil && !leftGC && !rightGC {
		if i.Left != nil {
			i.parentType = i.Left.parentType
			if i.ParentSub == nil {
				i.ParentSub = i.Left.ParentSub
			}
		} else if i.Right != nil {
			i.parentType = i.Right.parentType
			i.ParentSub = i.Right.ParentSub
		}
	}
	return 0, false, nil
}

// integrate places the item in its parent's list using the YATA conflict
// resolution algorithm (Yjs Item.integrate).
func (i *Item) integrate(txn *Transaction, offset uint64) error {
	store := txn.doc.store
	if offset > 0 {
		i.ID.Clock += offset
		ln, err := store.GetItemCleanEnd(ID{Client: i.ID.Client, Clock: i.ID.Clock - 1})
		if err != nil {
			return err
		}
		last := ID{Client: ln.Client(), Clock: ln.Clock() + ln.NodeLen() - 1}
		i.Left = ln.AsItem()
		i.Origin = &last
		i.Content = i.Content.Splice(offset)
	}

	parent := i.parentType
	if parent == nil {
		// Parent unknown: integrate as GC struct (Yjs).
		store.Append(NewGCNode(i.ID, i.Len()))
		return nil
	}

	if (i.Left == nil && (i.Right == nil || i.Right.Left != nil)) || (i.Left != nil && i.Left.Right != i.Right) {
		left := i.Left
		var o *Item
		if left != nil {
			o = left.Right
		} else if i.ParentSub != nil {
			o = parent.mapGet(*i.ParentSub)
			for o != nil && o.Left != nil {
				o = o.Left
			}
		} else {
			o = parent.start
		}

		conflicting := map[*Item]struct{}{}
		itemsBeforeOrigin := map[*Item]struct{}{}
		for o != nil && o != i.Right {
			itemsBeforeOrigin[o] = struct{}{}
			conflicting[o] = struct{}{}
			if sameID(i.Origin, o.Origin) {
				if o.ID.Client < i.ID.Client {
					left = o
					conflicting = map[*Item]struct{}{}
				} else if sameID(i.RightOrigin, o.RightOrigin) {
					break
				}
			} else if o.Origin != nil && setHasID(store, itemsBeforeOrigin, *o.Origin) {
				if _, ok := conflicting[getItem(store, *o.Origin)]; !ok {
					left = o
					conflicting = map[*Item]struct{}{}
				}
			} else {
				break
			}
			o = o.Right
		}
		i.Left = left
	}

	if i.Left != nil {
		right := i.Left.Right
		i.Right = right
		i.Left.Right = i
	} else {
		var r *Item
		if i.ParentSub != nil {
			r = parent.mapGet(*i.ParentSub)
			for r != nil && r.Left != nil {
				r = r.Left
			}
		} else {
			r = parent.start
			parent.start = i
		}
		i.Right = r
	}
	if i.Right != nil {
		i.Right.Left = i
	} else if i.ParentSub != nil {
		parent.mapSet(*i.ParentSub, i)
		if i.Left != nil {
			i.Left.deleteItem(txn)
		}
	}

	if i.ParentSub == nil && i.IsCountable() && !i.IsDeleted() {
		parent.length += i.Len()
	}

	store.Append(NewItemNode(i))
	i.Content.Integrate(txn, i)
	txn.addChanged(parent, i.ParentSub)

	if (parent.item != nil && parent.item.IsDeleted()) || (i.ParentSub != nil && i.Right != nil) {
		i.deleteItem(txn)
	}
	return nil
}

func setHasID(store *BlockStore, set map[*Item]struct{}, id ID) bool {
	item := getItem(store, id)
	if item == nil {
		return false
	}
	_, ok := set[item]
	return ok
}

// deleteItem marks the item deleted and records the range (Yjs Item.delete).
func (i *Item) deleteItem(txn *Transaction) {
	if i.IsDeleted() {
		return
	}
	if i.ParentSub == nil && i.IsCountable() && i.parentType != nil {
		i.parentType.length -= i.Len()
	}
	i.MarkAsDeleted()
	txn.deleteSet.Add(i.ID.Client, i.ID.Clock, i.Len())
	txn.addChanged(i.parentType, i.ParentSub)
	i.Content.Delete(txn)
}

// gc replaces deleted content with a tombstone (Yjs Item.gc with parentGCd
// false).
func (i *Item) gc() {
	if _, ok := i.Content.(*DeletedContent); ok {
		return
	}
	i.Content = &DeletedContent{Len: i.Len()}
}

// mergeWith merges right into i when they are adjacent, same-client, same
// origin/rightOrigin/deleted state, and the content supports merging (Yjs
// Item.mergeWith). It returns whether the merge happened.
func (i *Item) mergeWith(right *Item) bool {
	if i.Right != right {
		return false
	}
	if !sameID(right.Origin, idPtr(i.LastId())) {
		return false
	}
	if !sameID(i.RightOrigin, right.RightOrigin) {
		return false
	}
	if i.ID.Client != right.ID.Client {
		return false
	}
	if i.ID.Clock+i.Len() != right.ID.Clock {
		return false
	}
	if i.IsDeleted() != right.IsDeleted() {
		return false
	}
	if reflect.TypeOf(i.Content) != reflect.TypeOf(right.Content) {
		return false
	}
	if !i.Content.MergeWith(right.Content) {
		return false
	}
	i.Right = right.Right
	if right.Right != nil {
		right.Right.Left = i
	}
	if i.ParentSub != nil && i.parentType != nil && i.parentType.mapGet(*i.ParentSub) == right {
		i.parentType.mapSet(*i.ParentSub, i)
	}
	return true
}

func idPtr(id ID) *ID { return &id }
