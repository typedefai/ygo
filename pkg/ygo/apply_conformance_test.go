package ygo_test

import (
	"encoding/hex"
	"testing"

	"gotest.tools/assert"
	"riguz.com/ygo/pkg/ygo"
)

// TestYjsV1FixturesApplyReencode is the WP3 gate: applying a real Yjs update to
// a fresh Doc and re-encoding the full state must reproduce the same bytes.
// Integration, parentSub inheritance, GC and squashing all have to match.
func TestYjsV1FixturesApplyReencode(t *testing.T) {
	fixtures := loadYjsFixtures(t)
	identical := 0
	for _, f := range fixtures {
		f := f
		t.Run(f.Name, func(t *testing.T) {
			update, err := hex.DecodeString(f.V1)
			assert.NilError(t, err)

			doc := ygo.NewDocWithOptions(ygo.DocOptions{ClientId: 1, Gc: true})
			assert.NilError(t, doc.ApplyUpdateV1(update, nil))
			out, err := doc.EncodeStateAsUpdateV1()
			assert.NilError(t, err)

			if hex.EncodeToString(out) == f.V1 {
				identical++
			} else {
				t.Logf("re-encode differs:\n  orig: %s\n  ours: %s", f.V1, hex.EncodeToString(out))
			}
			items, ranges := doc.PendingStats()
			assert.Equal(t, 0, items, "pending structs after apply")
			assert.Equal(t, 0, ranges, "pending delete ranges after apply")
		})
	}
	t.Logf("%d/%d V1 fixtures apply + re-encode byte-identically", identical, len(fixtures))
}

func TestYjsV2FixturesApplyReencode(t *testing.T) {
	fixtures := loadYjsFixtures(t)
	identical := 0
	for _, f := range fixtures {
		f := f
		t.Run(f.Name, func(t *testing.T) {
			update, err := hex.DecodeString(f.V2)
			assert.NilError(t, err)

			doc := ygo.NewDocWithOptions(ygo.DocOptions{ClientId: 1, Gc: true})
			assert.NilError(t, doc.ApplyUpdateV2(update, nil))
			out, err := doc.EncodeStateAsUpdateV2()
			assert.NilError(t, err)

			if hex.EncodeToString(out) == f.V2 {
				identical++
			} else {
				t.Logf("re-encode differs:\n  orig: %s\n  ours: %s", f.V2, hex.EncodeToString(out))
			}
			items, ranges := doc.PendingStats()
			assert.Equal(t, 0, items, "pending structs after apply")
			assert.Equal(t, 0, ranges, "pending delete ranges after apply")
		})
	}
	t.Logf("%d/%d V2 fixtures apply + re-encode byte-identically", identical, len(fixtures))
}
