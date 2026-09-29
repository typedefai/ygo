package ygo_test

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"gotest.tools/assert"
	"riguz.com/ygo/pkg/ygo"
)

// Y.mergeUpdates fixtures. Regenerate with:
//
//	bun run testutil/gen_merge_fixtures.js
const mergeFixturesPath = "../../testutil/fixtures/yjs_merges.json"
const mergesReportPath = "../../testutil/fixtures/go_merges.json"

type mergeFixture struct {
	Name      string            `json:"name"`
	UpdatesV1 []string          `json:"updatesV1"`
	UpdatesV2 []string          `json:"updatesV2"`
	V1        string            `json:"v1"`
	V2        string            `json:"v2"`
	DocV1     string            `json:"docV1"`
	DocV2     string            `json:"docV2"`
	SV        map[string]uint64 `json:"sv"`
}

// TestYjsApplyOutOfOrder applies each merge fixture's input updates in reverse
// order, exercising the pending/retry path, and requires the doc to converge
// to yjs's canonical encoding.
func TestYjsApplyOutOfOrder(t *testing.T) {
	raw, err := os.ReadFile(filepath.FromSlash(mergeFixturesPath))
	if err != nil {
		t.Fatalf("read %s: %v", mergeFixturesPath, err)
	}
	var fixtures []mergeFixture
	assert.NilError(t, json.Unmarshal(raw, &fixtures))

	for _, f := range fixtures {
		f := f
		t.Run(f.Name, func(t *testing.T) {
			// forward V1
			docF := ygo.NewDocWithOptions(ygo.DocOptions{ClientId: 1, Gc: true})
			for _, u := range decodeHexList(t, f.UpdatesV1) {
				assert.NilError(t, docF.ApplyUpdateV1(u, nil))
			}
			outF, err := docF.EncodeStateAsUpdateV1()
			assert.NilError(t, err)
			assert.Equal(t, f.DocV1, hex.EncodeToString(outF), "forward V1")

			// reverse V1 (pending)
			docR := ygo.NewDocWithOptions(ygo.DocOptions{ClientId: 1, Gc: true})
			ups := decodeHexList(t, f.UpdatesV1)
			for i := len(ups) - 1; i >= 0; i-- {
				assert.NilError(t, docR.ApplyUpdateV1(ups[i], nil))
			}
			outR, err := docR.EncodeStateAsUpdateV1()
			assert.NilError(t, err)
			assert.Equal(t, f.DocV1, hex.EncodeToString(outR), "reverse V1")

			// forward V2
			docF2 := ygo.NewDocWithOptions(ygo.DocOptions{ClientId: 1, Gc: true})
			for _, u := range decodeHexList(t, f.UpdatesV2) {
				assert.NilError(t, docF2.ApplyUpdateV2(u, nil))
			}
			outF2, err := docF2.EncodeStateAsUpdateV2()
			assert.NilError(t, err)
			assert.Equal(t, f.DocV2, hex.EncodeToString(outF2), "forward V2")

			// reverse V2 (pending)
			docR2 := ygo.NewDocWithOptions(ygo.DocOptions{ClientId: 1, Gc: true})
			ups2 := decodeHexList(t, f.UpdatesV2)
			for i := len(ups2) - 1; i >= 0; i-- {
				assert.NilError(t, docR2.ApplyUpdateV2(ups2[i], nil))
			}
			outR2, err := docR2.EncodeStateAsUpdateV2()
			assert.NilError(t, err)
			assert.Equal(t, f.DocV2, hex.EncodeToString(outR2), "reverse V2")
		})
	}
}

type mergedEntry struct {
	Name        string `json:"name"`
	V1          string `json:"v1"`
	V2          string `json:"v2"`
	IdenticalV1 bool   `json:"identicalV1"`
	IdenticalV2 bool   `json:"identicalV2"`
}

func TestYjsMergeConformance(t *testing.T) {
	raw, err := os.ReadFile(filepath.FromSlash(mergeFixturesPath))
	if err != nil {
		t.Fatalf("read %s: %v (run `bun run testutil/gen_merge_fixtures.js`)", mergeFixturesPath, err)
	}
	var fixtures []mergeFixture
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatalf("parse %s: %v", mergeFixturesPath, err)
	}
	assert.Assert(t, len(fixtures) > 0)

	report := make([]mergedEntry, 0, len(fixtures))
	for _, f := range fixtures {
		f := f
		t.Run(f.Name, func(t *testing.T) {
			upsV1 := decodeHexList(t, f.UpdatesV1)
			mergedV1, err := ygo.MergeUpdatesV1(upsV1)
			assert.NilError(t, err)
			gotV1 := hex.EncodeToString(mergedV1)

			upsV2 := decodeHexList(t, f.UpdatesV2)
			mergedV2, err := ygo.MergeUpdatesV2(upsV2)
			assert.NilError(t, err)
			gotV2 := hex.EncodeToString(mergedV2)

			// Byte-identity is expected except where Yjs resolves an overlap
			// with a different (but semantically equivalent) struct split.
			identicalV1 := gotV1 == f.V1
			identicalV2 := gotV2 == f.V2
			if !identicalV1 || !identicalV2 {
				t.Logf("not byte-identical (v1=%v v2=%v); semantic equivalence is checked by testutil/verify_go_updates.js", identicalV1, identicalV2)
			}

			// State vector derived from the merged update must match yjs.
			assertStateVector(t, f.SV, mergedV1, mergedV2)

			// The merged update must be decodable.
			_, err = ygo.DecodeUpdateV1(mergedV1)
			assert.NilError(t, err)
			_, err = ygo.DecodeUpdateV2(mergedV2)
			assert.NilError(t, err)

			report = append(report, mergedEntry{
				Name: f.Name, V1: gotV1, V2: gotV2,
				IdenticalV1: identicalV1, IdenticalV2: identicalV2,
			})
		})
	}
	if !t.Failed() {
		writeReport(t, "go_merges.json", report)
	}
}

func assertStateVector(t *testing.T, want map[string]uint64, mergedV1, mergedV2 []uint8) {
	t.Helper()
	for _, sv := range []ygo.StateVector{
		mustSV(t, func() (ygo.StateVector, error) { return ygo.EncodeStateVectorFromUpdateV1(mergedV1) }),
		mustSV(t, func() (ygo.StateVector, error) { return ygo.EncodeStateVectorFromUpdateV2(mergedV2) }),
	} {
		assert.Equal(t, len(want), sv.Len(), "state vector size")
		for client, clock := range want {
			c, err := strconv.ParseUint(client, 10, 64)
			if err != nil {
				t.Fatalf("bad client key %q: %v", client, err)
			}
			assert.Equal(t, ygo.Clock(clock), sv.Get(ygo.ClientID(c)), "state vector clock for client %s", client)
		}
	}
}

func mustSV(t *testing.T, fn func() (ygo.StateVector, error)) ygo.StateVector {
	t.Helper()
	sv, err := fn()
	assert.NilError(t, err)
	return sv
}

func decodeHexList(t *testing.T, list []string) [][]uint8 {
	t.Helper()
	out := make([][]uint8, len(list))
	for i, s := range list {
		b, err := hex.DecodeString(s)
		assert.NilError(t, err)
		out[i] = b
	}
	return out
}
