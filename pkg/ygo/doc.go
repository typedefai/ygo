package ygo

import (
	"math/rand"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
)

type DocOptions struct {
	ClientId uint64
	Guid     string
	Gc       bool
}

func NewDocOptions(options ...func(*DocOptions)) (*DocOptions, error) {
	source := rand.NewSource(time.Now().UnixNano())
	rng := rand.New(source)
	id, err := gonanoid.New()
	if err != nil {
		return nil, err
	}
	val := &DocOptions{
		ClientId: uint64(rng.Uint32()),
		Guid:     id,
		Gc:       true,
	}
	for _, o := range options {
		o(val)
	}
	return val, nil
}

func WithClientId(id uint64) func(*DocOptions) {
	return func(s *DocOptions) {
		s.ClientId = id
	}
}

func WithGuid(guid string) func(*DocOptions) {
	return func(s *DocOptions) {
		s.Guid = guid
	}
}

func WithGc(autoGc bool) func(*DocOptions) {
	return func(s *DocOptions) {
		s.Gc = autoGc
	}
}

// Doc is the root CRDT document: a struct store plus the live shared types.
type Doc struct {
	clientId    uint64
	options     DocOptions
	store       *BlockStore
	share       map[string]*abstractType
	publisher   *DocPublisher
	transaction *Transaction
	gc          bool
	gcFilter    func(*Item) bool
	pending     map[ClientID][]Node
	pendingDs   *DeleteSet
}

func NewDoc() (*Doc, error) {
	options, err := NewDocOptions()
	if err != nil {
		return nil, err
	}
	return NewDocWithOptions(*options), nil
}

func NewDocWithOptions(options DocOptions) *Doc {
	return &Doc{
		clientId:  options.ClientId,
		options:   options,
		store:     NewBlockStore(),
		share:     map[string]*abstractType{},
		publisher: NewDocPublisher(),
		gc:        options.Gc,
		gcFilter:  func(*Item) bool { return true },
		pending:   map[ClientID][]Node{},
	}
}

func (d *Doc) ClientID() uint64 {
	return d.clientId
}

func (d *Doc) Options() DocOptions {
	return d.options
}

func (d *Doc) Store() *BlockStore {
	return d.store
}

func (d *Doc) Publisher() *DocPublisher {
	return d.publisher
}

// StateVector returns the highest integrated clock per client.
func (d *Doc) StateVector() StateVector {
	return d.store.GetStateVector()
}

// GetOrCreateType resolves (creating on demand) a root shared type by name.
func (d *Doc) GetOrCreateType(name string) *abstractType {
	t, ok := d.share[name]
	if !ok {
		t = newAbstractType(name, d)
		d.share[name] = t
	}
	return t
}

// ApplyUpdateV1 integrates a V1 update.
func (d *Doc) ApplyUpdateV1(update []uint8, origin any) error {
	return d.applyUpdate(func() (Update, error) { return DecodeUpdateV1(update) }, origin)
}

// ApplyUpdateV2 integrates a V2 update.
func (d *Doc) ApplyUpdateV2(update []uint8, origin any) error {
	return d.applyUpdate(func() (Update, error) { return DecodeUpdateV2(update) }, origin)
}

func (d *Doc) applyUpdate(decode func() (Update, error), origin any) error {
	var applyErr error
	d.Transact(func(txn *Transaction) {
		// Retry delete ranges that targeted structs which arrived earlier.
		if d.pendingDs != nil && d.pendingDs.Len() > 0 {
			rest, err := applyDeleteSet(txn, *d.pendingDs)
			if err != nil {
				applyErr = err
				return
			}
			d.pendingDs = rest
		}

		u, err := decode()
		if err != nil {
			applyErr = err
			return
		}

		pending, err := integrateStructs(txn, u.blocks)
		if err != nil {
			applyErr = err
			return
		}
		d.addPendingNodes(pending)

		rest, err := applyDeleteSet(txn, u.deleteSet)
		if err != nil {
			applyErr = err
			return
		}
		d.pendingDs = rest

		d.retryPending(txn)

		if d.pendingDs != nil {
			rest2, err := applyDeleteSet(txn, *d.pendingDs)
			if err != nil {
				applyErr = err
				return
			}
			d.pendingDs = rest2
		}
	}, origin)
	return applyErr
}

func (d *Doc) addPendingNodes(nodes []Node) {
	for _, n := range nodes {
		d.pending[n.Client()] = append(d.pending[n.Client()], n)
	}
}

func (d *Doc) addPendingMap(blocks map[ClientID][]Node) {
	for client, nodes := range blocks {
		d.pending[client] = append(d.pending[client], nodes...)
	}
}

func (d *Doc) retryPending(txn *Transaction) {
	if len(d.pending) == 0 {
		return
	}
	blocks := d.pending
	d.pending = map[ClientID][]Node{}
	rest, err := integrateStructs(txn, blocks)
	if err != nil {
		d.addPendingMap(blocks)
		return
	}
	d.addPendingNodes(rest)
}

// PendingStats reports how many structs/ranges are parked awaiting dependencies.
func (d *Doc) PendingStats() (items int, deleteRanges int) {
	for _, nodes := range d.pending {
		items += len(nodes)
	}
	if d.pendingDs != nil {
		for _, ranges := range d.pendingDs.clients {
			deleteRanges += len(ranges)
		}
	}
	return items, deleteRanges
}

// EncodeStateAsUpdateV1 serialises the full document state as a V1 update.
func (d *Doc) EncodeStateAsUpdateV1() ([]uint8, error) {
	return EncodeStateAsUpdateV1(d, nil)
}

// EncodeStateAsUpdateV2 serialises the full document state as a V2 update.
func (d *Doc) EncodeStateAsUpdateV2() ([]uint8, error) {
	return EncodeStateAsUpdateV2(d, nil)
}
