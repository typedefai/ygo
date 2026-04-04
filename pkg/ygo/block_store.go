package ygo

type BlockStore struct {
	clients map[ClientID]ClientBlockList
}

func NewBlockStore() *BlockStore {
	return &BlockStore{clients: make(map[ClientID]ClientBlockList)}
}

func (b *BlockStore) GetStateVector() StateVector {
	return NewStateVectorFrom(b)
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

func (c *ClientBlockList) Append(node Node) {
	c.list = append(c.list, node)
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

func (s *StateVector) Encode(encoder Encoder) error {
	if err := encoder.WriteVarInt(len(s.vector)); err != nil {
		return err
	}
	for client, clock := range s.vector {
		if err := encoder.WriteVarUint64(uint64(client)); err != nil {
			return err
		}
		if err := encoder.WriteVarUint64(clock); err != nil {
			return err
		}
	}
	return nil
}
