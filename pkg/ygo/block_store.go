package ygo

import (
	"fmt"
	"sort"
)

type BlockStore struct {
	clients map[ClientID]ClientBlockList
}

func NewBlockStore() *BlockStore {
	return &BlockStore{clients: make(map[ClientID]ClientBlockList)}
}

func (b *BlockStore) GetStateVector() StateVector {
	return NewStateVectorFrom(b)
}

// GetState returns the highest clock + 1 integrated for a client.
func (b *BlockStore) GetState(client ClientID) Clock {
	entry, ok := b.clients[client]
	if !ok {
		return 0
	}
	return entry.GetState()
}

// Find returns the node containing id, if any.
func (b *BlockStore) Find(id ID) (Node, bool) {
	entry, ok := b.clients[id.Client]
	if !ok {
		return Node{}, false
	}
	node, _, ok := entry.Find(id)
	return node, ok
}

// GetItemCleanStart returns the node starting exactly at id, splitting the
// containing node if id falls inside it (Yjs getItemCleanStart).
func (b *BlockStore) GetItemCleanStart(id ID) (Node, error) {
	entry, ok := b.clients[id.Client]
	if !ok {
		return Node{}, NewIncompleteDocumentError(fmt.Sprintf("no structs for client %d", id.Client))
	}
	node, err := entry.SplitAt(id)
	if err != nil {
		return Node{}, err
	}
	b.clients[id.Client] = entry
	return node, nil
}

// GetItemCleanEnd returns the node ending exactly at id, splitting the
// containing node if id falls before its end (Yjs getItemCleanEnd).
func (b *BlockStore) GetItemCleanEnd(id ID) (Node, error) {
	entry, ok := b.clients[id.Client]
	if !ok {
		return Node{}, NewIncompleteDocumentError(fmt.Sprintf("no structs for client %d", id.Client))
	}
	node, i, ok := entry.Find(id)
	if !ok {
		return Node{}, NewIncompleteDocumentError(fmt.Sprintf("cannot find struct %d@%d", id.Client, id.Clock))
	}
	if node.Clock()+node.NodeLen()-1 == id.Clock {
		return node, nil
	}
	offset := id.Clock - node.Clock() + 1
	if _, err := splitNodeAt(&entry.list, i, offset); err != nil {
		return Node{}, err
	}
	b.clients[id.Client] = entry
	return entry.list[i], nil
}

type ClientBlockList struct {
	list []Node
}

func (c *ClientBlockList) GetState() Clock {
	if len(c.list) == 0 {
		return 0
	}
	last := c.list[len(c.list)-1]
	return last.Clock() + last.NodeLen()
}

func (c *ClientBlockList) Get(index int) (Node, bool) {
	if index < 0 || index >= len(c.list) {
		return Node{}, false
	}
	return c.list[index], true
}

func (c *ClientBlockList) Len() int {
	return len(c.list)
}

// Append inserts a node keeping the list sorted by clock.
func (c *ClientBlockList) Append(node Node) {
	i := sort.Search(len(c.list), func(i int) bool { return c.list[i].Clock() >= node.Clock() })
	c.list = insertAt(c.list, i, node)
}

// Find returns the node containing id via binary search.
func (c *ClientBlockList) Find(id ID) (Node, int, bool) {
	i := sort.Search(len(c.list), func(i int) bool {
		return c.list[i].Clock()+c.list[i].NodeLen() > id.Clock
	})
	if i < len(c.list) && c.list[i].Clock() <= id.Clock {
		return c.list[i], i, true
	}
	return Node{}, 0, false
}

// SplitAt splits the node containing id so that a node starting exactly at id
// exists, and returns it.
func (c *ClientBlockList) SplitAt(id ID) (Node, error) {
	node, i, ok := c.Find(id)
	if !ok {
		return Node{}, NewIncompleteDocumentError(fmt.Sprintf("cannot find struct %d@%d", id.Client, id.Clock))
	}
	if node.Clock() == id.Clock {
		return node, nil
	}
	offset := id.Clock - node.Clock()
	if _, err := splitNodeAt(&c.list, i, offset); err != nil {
		return Node{}, err
	}
	return c.list[i+1], nil
}

func (b *BlockStore) Append(node Node) {
	entry := b.clients[node.Client()]
	entry.Append(node)
	b.clients[node.Client()] = entry
}

func (b *BlockStore) Get(clientID ClientID) (ClientBlockList, bool) {
	entry, ok := b.clients[clientID]
	return entry, ok
}

// splitNodeAt splits the node at list[i] at offset, updating the list in
// place. For Items the original object is mutated into the left half (so
// parent.start and map pointers stay valid); GC/Skip are replaced by new
// halves. It returns the new right node.
func splitNodeAt(list *[]Node, i int, offset uint64) (Node, error) {
	node := (*list)[i]
	switch node.tag {
	case nodeTagGC:
		n := node.gc
		left := NewGCNode(n.ID, offset)
		right := NewGCNode(ID{Client: n.ID.Client, Clock: n.ID.Clock + offset}, n.Len-offset)
		(*list)[i] = left
		*list = insertAt(*list, i+1, right)
		return right, nil
	case nodeTagSkip:
		n := node.skip
		left := NewSkipNode(n.ID, offset)
		right := NewSkipNode(ID{Client: n.ID.Client, Clock: n.ID.Clock + offset}, n.Len-offset)
		(*list)[i] = left
		*list = insertAt(*list, i+1, right)
		return right, nil
	default:
		right, err := node.item.SplitAt(offset)
		if err != nil {
			return Node{}, err
		}
		*list = insertAt(*list, i+1, NewItemNode(right))
		return NewItemNode(right), nil
	}
}

func insertAt(list []Node, index int, node Node) []Node {
	list = append(list, Node{})
	copy(list[index+1:], list[index:])
	list[index] = node
	return list
}

type StateVector struct {
	vector map[ClientID]Clock
}

func NewStateVector() StateVector {
	return StateVector{
		vector: make(map[ClientID]Clock),
	}
}
func NewStateVectorFrom(ss *BlockStore) StateVector {
	sv := NewStateVector()
	if ss == nil {
		return sv
	}
	for clientId, clientStructList := range ss.clients {
		sv.vector[clientId] = clientStructList.GetState()
	}
	return sv
}

func (s *StateVector) IsEmpty() bool {
	return len(s.vector) == 0
}

// ClientIDs returns the clients present in the state vector (unordered).
func (s *StateVector) ClientIDs() []ClientID {
	ids := make([]ClientID, 0, len(s.vector))
	for c := range s.vector {
		ids = append(ids, c)
	}
	return ids
}

func (s *StateVector) Len() int {
	return len(s.vector)
}

func (s *StateVector) Get(clientId ClientID) Clock {
	return s.vector[clientId]
}

func (s *StateVector) Contains(id ID) bool {
	return id.Clock <= s.Get(id.Client)
}

func (s *StateVector) IncreaseBy(clientId ClientID, delta Clock) {
	if delta > 0 {
		e := s.vector[clientId]
		e += delta
		s.vector[clientId] = e
	}
}

func (s *StateVector) SetMin(clientId ClientID, clock Clock) {
	c, exists := s.vector[clientId]
	if !exists || clock < c {
		s.vector[clientId] = clock
	}
}

func (s *StateVector) SetMax(clientId ClientID, clock Clock) {
	c, exists := s.vector[clientId]
	if !exists || clock > c {
		s.vector[clientId] = clock
	}
}

func (s *StateVector) Merge(other *StateVector) {
	for client, clock := range other.vector {
		s.SetMax(client, clock)
	}
}

// Encode writes the state vector in lib0's writeStateVector layout:
// VarUint(clientCount) followed by (client, clock) VarUint pairs. Clients are
// sorted for deterministic output.
func (s *StateVector) Encode(encoder Encoder) error {
	clients := make([]ClientID, 0, len(s.vector))
	for client := range s.vector {
		clients = append(clients, client)
	}
	sort.Slice(clients, func(i, j int) bool { return clients[i] < clients[j] })

	if err := encoder.WriteVarUint64(uint64(len(clients))); err != nil {
		return err
	}
	for _, client := range clients {
		if err := encoder.WriteVarUint64(uint64(client)); err != nil {
			return err
		}
		if err := encoder.WriteVarUint64(s.vector[client]); err != nil {
			return err
		}
	}
	return nil
}
