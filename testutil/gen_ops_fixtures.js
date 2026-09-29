// Generates local-operation fixtures: yjs performs a fixed op sequence and we
// record the resulting full update plus the JSON projection.
//
//   bun run testutil/gen_ops_fixtures.js
import * as Y from 'yjs'
import * as fs from 'node:fs'
import * as path from 'node:path'
import { fileURLToPath } from 'node:url'

const here = path.dirname(fileURLToPath(import.meta.url))
const outDir = path.join(here, 'fixtures')
fs.mkdirSync(outDir, { recursive: true })

const hex = (u8) => Array.from(u8, (b) => ('0' + b.toString(16)).slice(-2)).join('')

const toPlain = (v) => {
  if (v === undefined) return { __undefined: true }
  if (v instanceof Uint8Array) return { __bytes: hex(v) }
  if (v && typeof v.toJSON === 'function') return toPlain(v.toJSON())
  if (Array.isArray(v)) return v.map(toPlain)
  if (v && typeof v === 'object') {
    const o = {}
    for (const k of Object.keys(v)) o[k] = toPlain(v[k])
    return o
  }
  return v
}

const ops = []
const addOp = (name, clientID, build, roots) => {
  const doc = new Y.Doc({ gc: true })
  doc.clientID = clientID
  build(doc)
  const expected = {}
  const kinds = {}
  for (const [rootName, type] of Object.entries(roots(doc))) {
    expected[rootName] = toPlain(type)
    kinds[rootName] =
      type instanceof Y.Map ? 'map'
      : type instanceof Y.Array ? 'array'
      : type instanceof Y.Text ? 'text'
      : 'xml'
  }
  ops.push({
    name,
    clientID,
    v1: hex(Y.encodeStateAsUpdate(doc)),
    v2: hex(Y.encodeStateAsUpdateV2(doc)),
    expected,
    kinds,
  })
}

addOp(
  'ops_map',
  1,
  (doc) => {
    const m = doc.getMap('meta')
    m.set('a', 'x')
    m.set('b', 42)
    m.set('c', true)
    m.set('d', [1, 'two'])
    m.delete('a')
  },
  (doc) => ({ meta: doc.getMap('meta') }),
)

addOp(
  'ops_array',
  2,
  (doc) => {
    const a = doc.getArray('list')
    a.push([1, 2, 3])
    a.insert(1, ['x', 'y'])
    a.push(['z', { k: 'v' }])
    a.delete(0, 1)
  },
  (doc) => ({ list: doc.getArray('list') }),
)

addOp(
  'ops_text',
  3,
  (doc) => {
    const t = doc.getText('body')
    t.insert(0, 'Hello')
    t.insert(5, ' world')
    t.delete(0, 1)
    t.insert(1, '!')
  },
  (doc) => ({ body: doc.getText('body') }),
)

addOp(
  'ops_nested',
  4,
  (doc) => {
    const m = doc.getMap('meta')
    const nested = new Y.Map()
    nested.set('k', 'v')
    m.set('nested', nested)
    const arr = new Y.Array()
    arr.push([10, 20])
    m.set('list', arr)
    m.set('bytes', Uint8Array.from([1, 2, 3]))
  },
  (doc) => ({ meta: doc.getMap('meta') }),
)

const outPath = path.join(outDir, 'yjs_ops.json')
fs.writeFileSync(outPath, JSON.stringify(ops, null, 2) + '\n')
console.log(`wrote ${outPath}`)
for (const o of ops) console.log(`  ${o.name}: ${o.v1.length / 2}b`)
