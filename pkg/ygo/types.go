package ygo

// Parent represents the parent of an Item in the CRDT tree.
// It can be either a named root type (string) or an ID reference.
type Parent struct {
	Named *string // root type name, e.g. "text"
	ID    *ID     // reference to parent item by ID
}

func ParentFromString(name string) Parent {
	return Parent{Named: &name}
}

func ParentFromID(id ID) Parent {
	return Parent{ID: &id}
}
