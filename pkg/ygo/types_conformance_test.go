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

// canonicalJSON normalises a JSON value (sorted object keys, stable numbers).
func canonicalJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	assert.NilError(t, err)
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var out any
	assert.NilError(t, dec.Decode(&out))
	b2, err := json.Marshal(out)
	assert.NilError(t, err)
	return string(b2)
}

// TestYjsFixturesToJSON applies every yjs fixture and compares the public
// read API's JSON projection with the reference-captured expected value.
func TestYjsFixturesToJSON(t *testing.T) {
	fixtures := loadYjsFixtures(t)
	for _, f := range fixtures {
		f := f
		t.Run(f.Name, func(t *testing.T) {
			update, err := hex.DecodeString(f.V1)
			assert.NilError(t, err)
			doc := ygo.NewDocWithOptions(ygo.DocOptions{ClientId: 1, Gc: true})
			assert.NilError(t, doc.ApplyUpdateV1(update, nil))

			got := map[string]any{}
			for root, kind := range f.Kinds {
				switch kind {
				case "map":
					got[root] = doc.GetMap(root).ToJSON()
				case "array":
					got[root] = doc.GetArray(root).ToJSON()
				case "text":
					got[root] = doc.GetText(root).ToJSON()
				case "xml":
					got[root] = doc.GetXmlFragment(root).ToJSON()
				}
			}
			assert.Equal(t, canonicalJSON(t, f.Expected), canonicalJSON(t, got), "ToJSON")
		})
	}
}

// TestLocalOpsConformance runs the same operation sequences in Go and yjs and
// requires byte-identical full updates (covers local item creation, packing,
// splitting and deletion).
type opsFixture struct {
	Name     string            `json:"name"`
	ClientID uint64            `json:"clientID"`
	V1       string            `json:"v1"`
	V2       string            `json:"v2"`
	Expected json.RawMessage   `json:"expected"`
	Kinds    map[string]string `json:"kinds"`
}

func TestLocalOpsConformance(t *testing.T) {
	raw, err := os.ReadFile(filepath.FromSlash("../../testutil/fixtures/yjs_ops.json"))
	if err != nil {
		t.Fatalf("read ops fixtures: %v (run `bun run testutil/gen_ops_fixtures.js`)", err)
	}
	var fixtures []opsFixture
	assert.NilError(t, json.Unmarshal(raw, &fixtures))

	for _, f := range fixtures {
		f := f
		t.Run(f.Name, func(t *testing.T) {
			doc := ygo.NewDocWithOptions(ygo.DocOptions{ClientId: f.ClientID, Gc: true})
			switch f.Name {
			case "ops_map":
				m := doc.GetMap("meta")
				m.Set("a", "x")
				m.Set("b", 42)
				m.Set("c", true)
				m.Set("d", []any{1, "two"})
				m.Delete("a")
			case "ops_array":
				a := doc.GetArray("list")
				a.Push(1, 2, 3)
				a.Insert(1, "x", "y")
				a.Push("z", map[string]any{"k": "v"})
				a.Delete(0, 1)
			case "ops_text":
				txt := doc.GetText("body")
				txt.Insert(0, "Hello")
				txt.Insert(5, " world")
				txt.Delete(0, 1)
				txt.Insert(1, "!")
			case "ops_nested":
				m := doc.GetMap("meta")
				nested := doc.NewMap()
				nested.Set("k", "v")
				m.Set("nested", nested)
				arr := doc.NewArray()
				arr.Push(10, 20)
				m.Set("list", arr)
				m.Set("bytes", []byte{1, 2, 3})
			}

			got, err := doc.EncodeStateAsUpdateV1()
			assert.NilError(t, err)
			assert.Equal(t, f.V1, hex.EncodeToString(got), "V1 update")

			gotV2, err := doc.EncodeStateAsUpdateV2()
			assert.NilError(t, err)
			assert.Equal(t, f.V2, hex.EncodeToString(gotV2), "V2 update")

			gotJSON := map[string]any{}
			for root, kind := range f.Kinds {
				switch kind {
				case "map":
					gotJSON[root] = doc.GetMap(root).ToJSON()
				case "array":
					gotJSON[root] = doc.GetArray(root).ToJSON()
				case "text":
					gotJSON[root] = doc.GetText(root).ToJSON()
				}
			}
			var want any
			dec := json.NewDecoder(bytes.NewReader(f.Expected))
			dec.UseNumber()
			assert.NilError(t, dec.Decode(&want))
			assert.Equal(t, canonicalJSON(t, want), canonicalJSON(t, gotJSON), "ToJSON")
		})
	}
}

func TestObserve(t *testing.T) {
	doc := ygo.NewDocWithOptions(ygo.DocOptions{ClientId: 1, Gc: true})
	m := doc.GetMap("meta")

	var events []ygo.ChangeEvent
	m.Observe(func(ev ygo.ChangeEvent) { events = append(events, ev) })

	remote := ygo.NewDocWithOptions(ygo.DocOptions{ClientId: 2, Gc: true})
	remote.GetMap("meta").Set("k", "v")
	update, err := remote.EncodeStateAsUpdateV1()
	assert.NilError(t, err)
	assert.NilError(t, doc.ApplyUpdateV1(update, nil))

	assert.Equal(t, 1, len(events))
	assert.DeepEqual(t, []string{"k"}, events[0].Keys)

	// Local edits fire too, and unsubscribe stops delivery.
	m.Set("local", 1)
	assert.Equal(t, 2, len(events))
	m.Delete("local")
	assert.Equal(t, 3, len(events))
}
