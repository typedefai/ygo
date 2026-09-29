package ygo_test

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/assert"
	"riguz.com/ygo/pkg/ygo"
	"riguz.com/ygo/pkg/ygo/awareness"
	ygsync "riguz.com/ygo/pkg/ygo/sync"
)

// y-protocols sync + awareness fixtures. Regenerate with:
//
//	bun run testutil/gen_protocol_fixtures.js
const protocolFixturesPath = "../../testutil/fixtures/yjs_protocol.json"

type syncFixture struct {
	Name   string            `json:"name"`
	Full   string            `json:"full"`
	Step1  string            `json:"step1"`
	Step2  string            `json:"step2"`
	Update string            `json:"update"`
	SV     map[string]uint64 `json:"sv"`
}

type awarenessFixture struct {
	Name     string          `json:"name"`
	ClientID uint64          `json:"clientID"`
	Update   string          `json:"update"`
	States   json.RawMessage `json:"states"`
	Clock    uint64          `json:"clock"`
}

type protocolFixtures struct {
	Sync      []syncFixture      `json:"sync"`
	Awareness []awarenessFixture `json:"awareness"`
}

func loadProtocolFixtures(t *testing.T) protocolFixtures {
	t.Helper()
	raw, err := os.ReadFile(filepath.FromSlash(protocolFixturesPath))
	if err != nil {
		t.Fatalf("read %s: %v (run `bun run testutil/gen_protocol_fixtures.js`)", protocolFixturesPath, err)
	}
	var f protocolFixtures
	assert.NilError(t, json.Unmarshal(raw, &f))
	return f
}

func TestConformance_SyncProtocol(t *testing.T) {
	fixtures := loadProtocolFixtures(t)
	assert.Assert(t, len(fixtures.Sync) > 0)
	for _, f := range fixtures.Sync {
		f := f
		t.Run(f.Name, func(t *testing.T) {
			full, err := hex.DecodeString(f.Full)
			assert.NilError(t, err)
			doc := ygo.NewDocWithOptions(ygo.DocOptions{ClientId: 1, Gc: true})
			assert.NilError(t, doc.ApplyUpdateV1(full, nil))

			// Our sync message bytes must match y-protocols exactly.
			step1, err := ygsync.EncodeSyncStep1(doc)
			assert.NilError(t, err)
			assert.Equal(t, f.Step1, hex.EncodeToString(step1), "SyncStep1")

			step2, err := ygsync.EncodeSyncStep2(doc, nil)
			assert.NilError(t, err)
			assert.Equal(t, f.Step2, hex.EncodeToString(step2), "SyncStep2")

			update, err := ygsync.EncodeUpdate(full)
			assert.NilError(t, err)
			assert.Equal(t, f.Update, hex.EncodeToString(update), "Update")

			// An empty peer replying to the yjs SyncStep1 has nothing to send.
			empty := ygo.NewDocWithOptions(ygo.DocOptions{ClientId: 9, Gc: true})
			reply, err := ygsync.ApplySyncMessage(empty, step1, nil)
			assert.NilError(t, err)
			msgType, payload, err := ygsync.ReadSyncMessage(reply)
			assert.NilError(t, err)
			assert.Equal(t, uint64(ygsync.MsgSyncStep2), msgType)
			assert.Equal(t, "0000", hex.EncodeToString(payload), "empty peer reply")

			// A peer at the full state replying to an empty peer's SyncStep1
			// sends the whole document.
			emptyStep1, err := ygsync.EncodeSyncStep1(empty)
			assert.NilError(t, err)
			fullReply, err := ygsync.ApplySyncMessage(doc, emptyStep1, nil)
			assert.NilError(t, err)
			_, fullPayload, err := ygsync.ReadSyncMessage(fullReply)
			assert.NilError(t, err)
			assert.Equal(t, f.Full, hex.EncodeToString(fullPayload), "full-state reply payload")

			// yjs's SyncStep2 applies cleanly into a Go doc.
			fresh := ygo.NewDocWithOptions(ygo.DocOptions{ClientId: 9, Gc: true})
			decodedStep2, err := hex.DecodeString(f.Step2)
			assert.NilError(t, err)
			_, err = ygsync.ApplySyncMessage(fresh, decodedStep2, nil)
			assert.NilError(t, err)
			out, err := fresh.EncodeStateAsUpdateV1()
			assert.NilError(t, err)
			assert.Equal(t, f.Full, hex.EncodeToString(out), "applied full state")
		})
	}
}

func TestConformance_AwarenessProtocol(t *testing.T) {
	fixtures := loadProtocolFixtures(t)
	assert.Assert(t, len(fixtures.Awareness) > 0)
	for _, f := range fixtures.Awareness {
		f := f
		t.Run(f.Name, func(t *testing.T) {
			doc := ygo.NewDocWithOptions(ygo.DocOptions{ClientId: f.ClientID, Gc: true})
			aw := awareness.New(doc)
			update, err := hex.DecodeString(f.Update)
			assert.NilError(t, err)
			assert.NilError(t, aw.ApplyUpdate(update))

			got, err := json.Marshal(aw.GetState(f.ClientID))
			assert.NilError(t, err)
			assert.Equal(t, canonicalJSON(t, f.States), canonicalJSON(t, json.RawMessage(got)), "awareness state")

			// Re-encoding round-trips through our own decoder.
			reencoded, err := aw.EncodeUpdate([]uint64{f.ClientID})
			assert.NilError(t, err)
			other := awareness.New(ygo.NewDocWithOptions(ygo.DocOptions{ClientId: 100, Gc: true}))
			assert.NilError(t, other.ApplyUpdate(reencoded))
			got2, err := json.Marshal(other.GetState(f.ClientID))
			assert.NilError(t, err)
			assert.Equal(t, canonicalJSON(t, f.States), canonicalJSON(t, json.RawMessage(got2)), "re-encoded awareness state")
		})
	}
}

func TestSyncPeerConvergence(t *testing.T) {
	alice := ygo.NewDocWithOptions(ygo.DocOptions{ClientId: 1, Gc: true})
	bob := ygo.NewDocWithOptions(ygo.DocOptions{ClientId: 2, Gc: true})
	alice.GetText("body").Insert(0, "Hello from Alice!")
	bob.GetText("body").Insert(0, "Hello from Bob!")

	// Bob sends step1; Alice replies step2; Bob applies.
	step1, err := ygsync.EncodeSyncStep1(bob)
	assert.NilError(t, err)
	reply, err := ygsync.ApplySyncMessage(alice, step1, nil)
	assert.NilError(t, err)
	_, err = ygsync.ApplySyncMessage(bob, reply, nil)
	assert.NilError(t, err)

	// Alice sends step1; Bob replies step2; Alice applies.
	step1, err = ygsync.EncodeSyncStep1(alice)
	assert.NilError(t, err)
	reply, err = ygsync.ApplySyncMessage(bob, step1, nil)
	assert.NilError(t, err)
	_, err = ygsync.ApplySyncMessage(alice, reply, nil)
	assert.NilError(t, err)

	aText := alice.GetText("body").ToString()
	bText := bob.GetText("body").ToString()
	assert.Equal(t, aText, bText, "peers converged")
}
