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
		Gc:       false,
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

type Doc struct {
	clientId  uint64
	options   DocOptions
	store     *DocStore
	publisher *DocPublisher
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
		store:     NewDocStore(),
		publisher: NewDocPublisher(),
	}
}

func (d *Doc) ClientID() uint64 {
	return d.clientId
}

func (d *Doc) Options() DocOptions {
	return d.options
}

func (d *Doc) Store() *DocStore {
	return d.store
}

func (d *Doc) Publisher() *DocPublisher {
	return d.publisher
}
