// Generates Y.mergeUpdates fixtures from the JavaScript reference.
//
//   bun run testutil/gen_merge_fixtures.js
//
// Writes testutil/fixtures/yjs_merges.json, consumed by
// pkg/ygo/merge_conformance_test.go and testutil/verify_go_updates.js.
import * as Y from 'yjs'
import * as fs from 'node:fs'
import * as path from 'node:path'
import { fileURLToPath } from 'node:url'

const here = path.dirname(fileURLToPath(import.meta.url))
const outDir = path.join(here, 'fixtures')
fs.mkdirSync(outDir, { recursive: true })

const hex = (u8) => Array.from(u8, (b) => ('0' + b.toString(16)).slice(-2)).join('')

// Encodes the doc in both formats; sv may be an encoded state vector.
const cap = (doc, sv) => [
  Y.encodeStateAsUpdate(doc, sv),
  Y.encodeStateAsUpdateV2(doc, sv),
]

const merges = []
const addMerge = (name, v1updates, v2updates) => {
  const v1 = Y.mergeUpdates(v1updates)
  const v2 = Y.mergeUpdatesV2(v2updates)
  const sv = {}
  Y.decodeStateVector(Y.encodeStateVectorFromUpdateV2(v2)).forEach((clock, client) => {
    sv[String(client)] = clock
  })
  // Canonical document encodings after applying every input update (used to
  // check out-of-order/pending integration in Go).
  const d1 = new Y.Doc({ gc: true })
  v1updates.forEach((u) => Y.applyUpdate(d1, u))
  const d2 = new Y.Doc({ gc: true })
  v2updates.forEach((u) => Y.applyUpdateV2(d2, u))
  merges.push({
    name,
    updatesV1: v1updates.map(hex),
    updatesV2: v2updates.map(hex),
    v1: hex(v1),
    v2: hex(v2),
    docV1: hex(Y.encodeStateAsUpdate(d1)),
    docV2: hex(Y.encodeStateAsUpdateV2(d2)),
    sv,
  })
}

// 1. Sequential edits from one client (adjacent items are not merged at struct
// level).
{
  const doc = new Y.Doc({ gc: true })
  doc.clientID = 1
  const t = doc.getText('body')
  t.insert(0, 'Hello')
  const sv1 = Y.encodeStateVector(doc)
  const a = cap(doc)
  t.insert(5, ' world')
  const b = cap(doc, sv1)
  const sv2 = Y.encodeStateVector(doc)
  t.insert(11, '!')
  const c = cap(doc, sv2)
  addMerge('text_sequential', [a[0], b[0], c[0]], [a[1], b[1], c[1]])
}

// 2. Concurrent edits from two clients.
{
  const a = new Y.Doc({ gc: true })
  a.clientID = 1
  a.getMap('meta').set('a', 1)
  const b = new Y.Doc({ gc: true })
  b.clientID = 2
  b.getMap('meta').set('b', 2)
  b.getMap('meta').set('b', 3)
  const ca = cap(a)
  const cb = cap(b)
  addMerge('map_concurrent', [ca[0], cb[0]], [ca[1], cb[1]])
}

// 3. Same update twice (full overlap).
{
  const doc = new Y.Doc({ gc: true })
  doc.clientID = 5
  doc.getArray('list').push([1, 'two', true])
  const c = cap(doc)
  addMerge('duplicate', [c[0], c[0]], [c[1], c[1]])
}

// 4. Partial overlap: full state plus a tail encoded against an earlier SV.
{
  const doc = new Y.Doc({ gc: true })
  doc.clientID = 9
  const t = doc.getText('t')
  t.insert(0, '0123456789')
  const full = cap(doc)
  const svMid = Y.encodeStateVector(doc)
  t.insert(10, 'abcdef')
  const tail = cap(doc, svMid)
  addMerge('overlap_tail', [full[0], tail[0]], [full[1], tail[1]])
}

// 5. Clock gap between structs from different updates -> Skip in the merge.
{
  const d1 = new Y.Doc({ gc: true })
  d1.clientID = 7
  d1.getText('t').insert(0, 'ab') // clocks 0-2
  const base = cap(d1)

  const d2 = new Y.Doc({ gc: true })
  d2.clientID = 7
  const t = d2.getText('t')
  t.insert(0, '01234') // clocks 0-5
  const svA = Y.encodeStateVector(d2)
  t.insert(5, '56789') // clocks 5-10
  const tail = cap(d2, svA)
  addMerge('gap_skip', [base[0], tail[0]], [base[1], tail[1]])
}

// 6. Rich text: formatting and embeds across incremental updates.
{
  const doc = new Y.Doc({ gc: true })
  doc.clientID = 3
  const t = doc.getText('body')
  t.insert(0, 'Hello world')
  const sv1 = Y.encodeStateVector(doc)
  const u1 = cap(doc)
  t.format(0, 5, { bold: true })
  const u2 = cap(doc, sv1)
  const sv2 = Y.encodeStateVector(doc)
  t.insertEmbed(11, { type: 'image', src: 'a.png' })
  const u3 = cap(doc, sv2)
  addMerge('ytext_rich', [u1[0], u2[0], u3[0]], [u1[1], u2[1], u3[1]])
}

// 7. GC structs crossing updates (deletes).
{
  const a = new Y.Doc({ gc: true })
  a.clientID = 1
  a.getArray('list').push(['a1', 'a2', 'a3'])
  const u1 = cap(a)
  const b = new Y.Doc({ gc: true })
  b.clientID = 2
  b.getArray('list').push(['b1'])
  Y.applyUpdate(b, u1[0])
  b.getArray('list').delete(1, 2)
  const u2 = cap(b)
  addMerge('array_gc', [u1[0], u2[0]], [u1[1], u2[1]])
}

// 8. XML across updates.
{
  const doc = new Y.Doc({ gc: true })
  doc.clientID = 4
  const frag = doc.getXmlFragment('content')
  const p = new Y.XmlElement('p')
  frag.insert(0, [p])
  const txt = new Y.XmlText()
  p.insert(0, [txt])
  const sv1 = Y.encodeStateVector(doc)
  const u1 = cap(doc)
  txt.insert(0, 'hello')
  const u2 = cap(doc, sv1)
  addMerge('xml_incremental', [u1[0], u2[0]], [u1[1], u2[1]])
}

const outPath = path.join(outDir, 'yjs_merges.json')
fs.writeFileSync(outPath, JSON.stringify(merges, null, 2) + '\n')
console.log(`wrote ${outPath}`)
for (const m of merges) console.log(`  ${m.name}: ${m.updatesV1.length} updates -> v1=${m.v1.length / 2}b v2=${m.v2.length / 2}b`)
