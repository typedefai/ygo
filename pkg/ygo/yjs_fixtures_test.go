package ygo_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/assert"
	"riguz.com/ygo/pkg/ygo"
)

// Yjs-generated update fixtures. Regenerate with:
//
//	bun run testutil/gen_fixtures.js
//
// These tests decode each update with our decoder and re-encode it with the
// same version. They are the input half of the cross-implementation check; the
// outputs are written to testutil/fixtures/go_v{1,2}_reencoded.json and
// verified semantically against yjs by `bun run testutil/verify_go_updates.js`.
const yjsFixturesPath = "../../testutil/fixtures/yjs_updates.json"

type yjsFixture struct {
	Name     string            `json:"name"`
	V1       string            `json:"v1"`
	V2       string            `json:"v2"`
	Expected json.RawMessage   `json:"expected"`
	Kinds    map[string]string `json:"kinds"`
}

type reencodedEntry struct {
	Name      string `json:"name"`
	Original  string `json:"original"`
	Hex       string `json:"hex"`
	Identical bool   `json:"identical"`
}

type convertedEntry struct {
	Name string `json:"name"`
	Hex  string `json:"hex"`
}

func loadYjsFixtures(t *testing.T) []yjsFixture {
	t.Helper()
	raw, err := os.ReadFile(filepath.FromSlash(yjsFixturesPath))
	if err != nil {
		t.Fatalf("read %s: %v (run `bun run testutil/gen_fixtures.js`)", yjsFixturesPath, err)
	}
	var fixtures []yjsFixture
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatalf("parse %s: %v", yjsFixturesPath, err)
	}
	assert.Assert(t, len(fixtures) > 0)
	return fixtures
}

func TestYjsV1FixturesDecodeReencode(t *testing.T) {
	fixtures := loadYjsFixtures(t)
	report := make([]reencodedEntry, 0, len(fixtures))
	identical := 0
	for _, f := range fixtures {
		f := f
		t.Run(f.Name, func(t *testing.T) {
			update, err := hex.DecodeString(f.V1)
			assert.NilError(t, err)
			decoded, err := ygo.DecodeUpdateV1(update)
			assert.NilError(t, err)
			out, err := decoded.EncodeV1()
			assert.NilError(t, err)
			entry := reencodedEntry{Name: f.Name, Original: f.V1, Hex: hex.EncodeToString(out), Identical: bytes.Equal(out, update)}
			if entry.Identical {
				identical++
			}
			report = append(report, entry)
		})
	}
	if !t.Failed() {
		writeReencoded(t, "go_v1_reencoded.json", report)
	}
	t.Logf("%d/%d V1 fixtures re-encoded byte-identically", identical, len(fixtures))
}

func TestYjsV2FixturesDecodeReencode(t *testing.T) {
	fixtures := loadYjsFixtures(t)
	report := make([]reencodedEntry, 0, len(fixtures))
	identical := 0
	for _, f := range fixtures {
		f := f
		t.Run(f.Name, func(t *testing.T) {
			update, err := hex.DecodeString(f.V2)
			assert.NilError(t, err)
			decoded, err := ygo.DecodeUpdateV2(update)
			assert.NilError(t, err)
			out, err := decoded.EncodeV2()
			assert.NilError(t, err)
			entry := reencodedEntry{Name: f.Name, Original: f.V2, Hex: hex.EncodeToString(out), Identical: bytes.Equal(out, update)}
			if entry.Identical {
				identical++
			}
			report = append(report, entry)
		})
	}
	if !t.Failed() {
		writeReencoded(t, "go_v2_reencoded.json", report)
	}
	t.Logf("%d/%d V2 fixtures re-encoded byte-identically", identical, len(fixtures))
}

// TestYjsUpdateFormatConversion checks the struct-level V1<->V2 conversion
// path (no document integration). Outputs are verified against yjs by
// testutil/verify_go_updates.js.
func TestYjsUpdateFormatConversion(t *testing.T) {
	fixtures := loadYjsFixtures(t)
	v1ToV2 := make([]convertedEntry, 0, len(fixtures))
	v2ToV1 := make([]convertedEntry, 0, len(fixtures))
	for _, f := range fixtures {
		f := f
		t.Run(f.Name, func(t *testing.T) {
			v1, err := hex.DecodeString(f.V1)
			assert.NilError(t, err)
			v2, err := hex.DecodeString(f.V2)
			assert.NilError(t, err)

			outV2, err := ygo.UpdateV1ToV2(v1)
			assert.NilError(t, err)
			outV1, err := ygo.UpdateV2ToV1(v2)
			assert.NilError(t, err)

			// Sanity: converted updates must decode in the target format.
			_, err = ygo.DecodeUpdateV2(outV2)
			assert.NilError(t, err)
			_, err = ygo.DecodeUpdateV1(outV1)
			assert.NilError(t, err)

			v1ToV2 = append(v1ToV2, convertedEntry{Name: f.Name, Hex: hex.EncodeToString(outV2)})
			v2ToV1 = append(v2ToV1, convertedEntry{Name: f.Name, Hex: hex.EncodeToString(outV1)})
		})
	}
	if !t.Failed() {
		writeReport(t, "go_v1_to_v2.json", v1ToV2)
		writeReport(t, "go_v2_to_v1.json", v2ToV1)
	}
}

func writeReencoded(t *testing.T, name string, report []reencodedEntry) {
	t.Helper()
	writeReport(t, name, report)
}

func writeReport(t *testing.T, name string, report any) {
	t.Helper()
	out, err := json.MarshalIndent(report, "", "  ")
	assert.NilError(t, err)
	path := filepath.Join("../../testutil/fixtures", name)
	assert.NilError(t, os.WriteFile(path, append(out, '\n'), 0o644))
}
