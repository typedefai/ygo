// Package awareness implements the y-protocols awareness layer: ephemeral,
// last-write-wins presence state (cursors, user info, …) that is not part of
// the CRDT document.
//
// Wire format: VarUint(clientCount), then per client
// (clientID:VarUint, clock:VarUint, state:VarString(json)); a null state means
// the client disconnected.
package awareness

import (
	"bytes"
	"encoding/json"
	"fmt"

	"riguz.com/ygo/internal/lib0"
	"riguz.com/ygo/pkg/ygo"
)

// State is one client's presence state at a given clock.
type State struct {
	Clock uint64
	Data  any
}

// Awareness tracks remote presence state for one document.
type Awareness struct {
	clientID uint64
	states   map[uint64]State
}

// New creates an awareness instance for a document.
func New(doc *ygo.Doc) *Awareness {
	return &Awareness{
		clientID: doc.ClientID(),
		states:   map[uint64]State{},
	}
}

// ClientID returns the local client id.
func (a *Awareness) ClientID() uint64 { return a.clientID }

// SetLocalState replaces the local state and increments its clock.
func (a *Awareness) SetLocalState(state any) {
	cur := a.states[a.clientID]
	a.states[a.clientID] = State{Clock: cur.Clock + 1, Data: state}
}

// SetLocalStateField merges one field into the local state map.
func (a *Awareness) SetLocalStateField(key string, value any) {
	m, _ := a.states[a.clientID].Data.(map[string]any)
	next := make(map[string]any, len(m)+1)
	for k, v := range m {
		next[k] = v
	}
	next[key] = value
	a.SetLocalState(next)
}

// GetState returns a client's state (nil when absent).
func (a *Awareness) GetState(client uint64) any {
	if s, ok := a.states[client]; ok {
		return s.Data
	}
	return nil
}

// States returns a copy of all client states.
func (a *Awareness) States() map[uint64]any {
	out := make(map[uint64]any, len(a.states))
	for c, s := range a.states {
		out[c] = s.Data
	}
	return out
}

// RemoveState removes a client (e.g. on disconnect).
func (a *Awareness) RemoveState(client uint64) {
	delete(a.states, client)
}

// EncodeUpdate serialises the states for the given clients (all when nil).
func (a *Awareness) EncodeUpdate(clients []uint64) ([]byte, error) {
	if clients == nil {
		for c := range a.states {
			clients = append(clients, c)
		}
	}
	w := lib0.NewBufferWrite()
	if err := w.WriteVarUint64(uint64(len(clients))); err != nil {
		return nil, err
	}
	for _, c := range clients {
		state := a.states[c]
		if err := w.WriteVarUint64(c); err != nil {
			return nil, err
		}
		if err := w.WriteVarUint64(state.Clock); err != nil {
			return nil, err
		}
		data, err := json.Marshal(state.Data)
		if err != nil {
			return nil, err
		}
		s := string(data)
		if err := w.WriteVarString(&s); err != nil {
			return nil, err
		}
	}
	return w.ToBytes(), nil
}

// ApplyUpdate merges an awareness update, ignoring stale clocks.
func (a *Awareness) ApplyUpdate(update []byte) error {
	r := lib0.NewBufferRead(bytes.NewReader(update))
	count, err := r.ReadVarUint()
	if err != nil {
		return err
	}
	for i := uint64(0); i < count; i++ {
		client, err := r.ReadVarUint()
		if err != nil {
			return err
		}
		clock, err := r.ReadVarUint()
		if err != nil {
			return err
		}
		raw, err := r.ReadVarString()
		if err != nil {
			return err
		}
		var data any
		if err := json.Unmarshal([]byte(raw), &data); err != nil {
			return fmt.Errorf("ygo/awareness: bad state json: %w", err)
		}
		cur, exists := a.states[client]
		apply := !exists || cur.Clock < clock || (cur.Clock == clock && data == nil && exists)
		if !apply {
			continue
		}
		if data == nil {
			delete(a.states, client)
		} else {
			a.states[client] = State{Clock: clock, Data: data}
		}
	}
	return nil
}
