// Generates encodeStateAsUpdate(doc, stateVector) diff fixtures.
//
//   bun run testutil/gen_diff_fixtures.js
//
// Writes testutil/fixtures/yjs_diffs.json, consumed by
// pkg/ygo/diff_conformance_test.go.
import * as Y from 'yjs'
import * as fs from 'node:fs'
import * as path from 'node:path'
import { fileURLToPath } from 'node:url'

const here = path.dirname(fileURLToPath(import.meta.url))
const outDir = path.join(here, 'fixtures')
fs.mkdirSync(outDir, { recursive: true })

const hex = (u8) => Array.from(u8, (b) => ('0' + b.toString(16)).slice(-2)).join('')

const diffs = []
const addDiff = (name, doc, sv) => {
  const svMap = {}
  Y.decodeStateVector(sv).forEach((clock, client) => {
    svMap[String(client)] = clock
  })
  diffs.push({
    name,
    fullV1: hex(Y.encodeStateAsUpdate(doc)),
    fullV2: hex(Y.encodeStateAsUpdateV2(doc)),
    sv: svMap,
    svBytes: hex(sv),
    diffV1: hex(Y.encodeStateAsUpdate(doc, sv)),
    diffV2: hex(Y.encodeStateAsUpdateV2(doc, sv)),
  })
}

// 1. State vector cuts into the middle of a single string item (offset write).
{
  const doc = new Y.Doc({ gc: true })
  doc.clientID = 1
  doc.getText('body').insert(0, 'x'.repeat(100))
  const sv = Uint8Array.from([1, 1, 40]) // {client 1: clock 40}
  addDiff('text_mid_item', doc, sv)
}

// 2. Same, with a state vector that lands exactly on the item boundary.
{
  const doc = new Y.Doc({ gc: true })
  doc.clientID = 2
  doc.getText('body').insert(0, 'hello')
  const sv = Uint8Array.from([1, 2, 5])
  addDiff('text_at_boundary', doc, sv)
}

// 3. Map with several keys: SV taken part-way through the history.
{
  const doc = new Y.Doc({ gc: true })
  doc.clientID = 3
  const m = doc.getMap('meta')
  m.set('a', 1)
  m.set('b', 'two')
  const sv = Y.encodeStateVector(doc)
  m.set('c', true)
  m.set('d', { nested: [1, 2, 3] })
  addDiff('map_partial', doc, sv)
}

// 4. Array across transactions, SV after the first insert.
{
  const doc = new Y.Doc({ gc: true })
  doc.clientID = 4
  const a = doc.getArray('list')
  a.push([1, 2, 3])
  const sv = Y.encodeStateVector(doc)
  a.insert(1, ['x', 'y'])
  a.push([4])
  addDiff('array_partial', doc, sv)
}

// 5. Two clients, target knows one client fully and part of the other.
{
  const a = new Y.Doc({ gc: true })
  a.clientID = 5
  const ta = a.getText('t')
  ta.insert(0, 'AAAA')
  const sv = Y.encodeStateVector(a)

  const b = new Y.Doc({ gc: true })
  b.clientID = 6
  b.getText('t').insert(0, 'BBBB')
  Y.applyUpdate(b, Y.encodeStateAsUpdate(a))
  addDiff('two_clients_partial', b, sv)
}

// 6. Deleted items: the delete set is always written in full.
{
  const doc = new Y.Doc({ gc: true })
  doc.clientID = 7
  const t = doc.getText('t')
  t.insert(0, '0123456789')
  t.delete(2, 4)
  const sv = Y.encodeStateVector(doc)
  doc.getText('t').insert(0, 'pre-')
  addDiff('with_delete_set', doc, sv)
}

// 7. GC-enabled deletes crossing the SV boundary.
{
  const doc = new Y.Doc({ gc: true })
  doc.clientID = 8
  const m = doc.getMap('m')
  m.set('keep', 1)
  m.set('gone', 2)
  const sv = Y.encodeStateVector(doc)
  m.delete('gone')
  m.set('after', 3)
  addDiff('gc_after_sv', doc, sv)
}

const outPath = path.join(outDir, 'yjs_diffs.json')
fs.writeFileSync(outPath, JSON.stringify(diffs, null, 2) + '\n')
console.log(`wrote ${outPath}`)
for (const d of diffs) console.log(`  ${d.name}: full=${d.fullV1.length / 2}b diff=${d.diffV1.length / 2}b`)
