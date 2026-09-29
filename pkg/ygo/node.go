package ygo

// Node represents a node in the CRDT update structure.
// It can be a GC (garbage collected), Skip, or Item node.
type Node struct {
	tag  nodeTag
	gc   *NodeLen
	skip *NodeLen
	item *Item
}

type nodeTag uint8

const (
	nodeTagGC   nodeTag = 0
	nodeTagSkip nodeTag = 1
	nodeTagItem nodeTag = 2
)

// NodeLen is the simple representation used by GC and Skip nodes.
type NodeLen struct {
	ID  ID
	Len uint64
}

func NewGCNode(id ID, length uint64) Node {
	return Node{tag: nodeTagGC, gc: &NodeLen{ID: id, Len: length}}
}

func NewSkipNode(id ID, length uint64) Node {
	return Node{tag: nodeTagSkip, skip: &NodeLen{ID: id, Len: length}}
}

func NewItemNode(item *Item) Node {
	return Node{tag: nodeTagItem, item: item}
}

func (n *Node) IsGC() bool   { return n.tag == nodeTagGC }
func (n *Node) IsSkip() bool { return n.tag == nodeTagSkip }
func (n *Node) IsItem() bool { return n.tag == nodeTagItem }

func (n *Node) NodeID() ID {
	switch n.tag {
	case nodeTagGC:
		return n.gc.ID
	case nodeTagSkip:
		return n.skip.ID
	case nodeTagItem:
		return n.item.ID
	}
	return ID{}
}

func (n *Node) Client() ClientID {
	return n.NodeID().Client
}

func (n *Node) Clock() Clock {
	return n.NodeID().Clock
}

func (n *Node) NodeLen() uint64 {
	switch n.tag {
	case nodeTagGC:
		return n.gc.Len
	case nodeTagSkip:
		return n.skip.Len
	case nodeTagItem:
		return n.item.Len()
	}
	return 0
}

func (n *Node) AsItem() *Item {
	if n.tag == nodeTagItem {
		return n.item
	}
	return nil
}

func (n *Node) IsDeleted() bool {
	switch n.tag {
	case nodeTagGC:
		return true
	case nodeTagItem:
		return n.item.IsDeleted()
	default:
		return false
	}
}

func (n *Node) LastID() *ID {
	if n.tag == nodeTagItem {
		lid := n.item.LastId()
		return &lid
	}
	return nil
}

// ReadNode reads a Node from the decoder. The id is the expected ID for this node.
func ReadNode(decoder Decoder, id ID) (Node, error) {
	info, err := decoder.ReadInfo()
	if err != nil {
		return Node{}, err
	}
	first5Bit := info & 0b11111

	switch first5Bit {
	case BLOCK_GC_REF_NUMBER:
		length, err := decoder.ReadVarUint()
		if err != nil {
			return Node{}, err
		}
		return NewGCNode(id, length), nil
	case BLOCK_SKIP_REF_NUMBER:
		length, err := decoder.ReadVarUint()
		if err != nil {
			return Node{}, err
		}
		return NewSkipNode(id, length), nil
	default:
		item, err := ReadItem(decoder, id, info, first5Bit)
		if err != nil {
			return Node{}, err
		}
		return NewItemNode(item), nil
	}
}

// WriteNode writes the Node to the encoder. offset > 0 writes only the suffix
// from that clock offset (used by state-vector diff encoding).
func (n *Node) WriteNode(encoder Encoder, offset uint64) error {
	switch n.tag {
	case nodeTagGC:
		// GC uses writeLen (the len column in V2); Skip writes a plain VarUint
		// to rest because its length can't use predictable-length encoding.
		if err := encoder.WriteInfo(0); err != nil {
			return err
		}
		return encoder.WriteLen(n.gc.Len - offset)
	case nodeTagSkip:
		if err := encoder.WriteInfo(10); err != nil {
			return err
		}
		return encoder.WriteVarUint64(n.skip.Len - offset)
	case nodeTagItem:
		return n.item.WriteItem(encoder, offset)
	}
	return nil
}
