package ygo

type DocStore struct {
	blocks *BlockStore
}

func NewDocStore() *DocStore {
	return &DocStore{blocks: NewBlockStore()}
}

func (s *DocStore) GetStateVector() StateVector {
	if s == nil || s.blocks == nil {
		return NewStateVector()
	}
	return s.blocks.GetStateVector()
}

func (s *DocStore) AppendNode(node Node) {
	if s.blocks == nil {
		s.blocks = NewBlockStore()
	}
	s.blocks.Append(node)
}
