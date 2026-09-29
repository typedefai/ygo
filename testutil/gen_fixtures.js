// Generates Yjs update fixtures (V1 + V2) from the JavaScript reference.
//
//   bun run testutil/gen_fixtures.js
//
// Writes testutil/fixtures/yjs_updates.json. Consumed by
// pkg/ygo/yjs_fixtures_test.go and verified semantically by
// testutil/verify_go_updates.js.
import * as Y from 'yjs'
import * as fs from 'node:fs'
import * as path from 'node:path'
import { fileURLToPath } from 'node:url'

const here = path.dirname(fileURLToPath(import.meta.url))
const outDir = path.join(here, 'fixtures')
fs.mkdirSync(outDir, { recursive: true })

const hex = (u8) => Array.from(u8, (b) => ('0' + b.toString(16)).slice(-2)).join('')

// The kind of shared type, so verifiers can instantiate the matching typed
// root (`doc.getMap(name)` etc.) after applying an update. yjs stores updates
// against generic root types until an application asks for a concrete one.
const kindOf = (v) => {
  if (v instanceof Y.Map) return 'map'
  if (v instanceof Y.Array) return 'array'
  if (v instanceof Y.Text) return 'text'
  if (v instanceof Y.XmlFragment) return 'xml'
  throw new Error(`unsupported root type: ${v?.constructor?.name}`)
}

// JSON-safe projection of a shared value; preserves Uint8Array and undefined,
// which plain JSON.stringify would mangle.
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

const fixtures = []

// build: (doc) => roots: { name: sharedType }, or { doc, roots } when the
// fixture merges several documents.
const add = (name, build) => {
  const doc = new Y.Doc({ gc: true })
  doc.clientID = 1
  let result = build(doc) || {}
  let actual = doc
  if (result.doc) {
    actual = result.doc
    result = result.roots || {}
  }
  const roots = result
  const expected = {}
  const kinds = {}
  for (const [k, v] of Object.entries(roots)) {
    expected[k] = toPlain(v)
    kinds[k] = kindOf(v)
  }
  fixtures.push({
    name,
    v1: hex(Y.encodeStateAsUpdate(actual)),
    v2: hex(Y.encodeStateAsUpdateV2(actual)),
    expected,
    kinds,
  })
}

add('ymap_basic', (doc) => {
  const m = doc.getMap('meta')
  m.set('s', 'hello')
  m.set('i', 42)
  m.set('neg', -7)
  m.set('f', 1.5)
  m.set('big', 2 ** 40)
  m.set('b', true)
  m.set('n', null)
  m.set('bytes', Uint8Array.from([1, 2, 3]))
  m.set('obj', { a: 1, b: 'x' })
  m.set('arr', [1, 'two', false])
  return { meta: m }
})

add('ymap_dupkey', (doc) => {
  const m = doc.getMap('meta')
  m.set('k', 'first')
  m.set('k', 'second')
  m.set('k', 'third')
  return { meta: m }
})

add('ymap_delete', (doc) => {
  const m = doc.getMap('meta')
  m.set('keep', 'yes')
  m.set('doomed', 'x')
  m.delete('doomed')
  return { meta: m }
})

add('yarray_mixed', (doc) => {
  const a = doc.getArray('list')
  a.insert(0, [1, 'two', true, null, { k: 'v' }])
  a.push(['tail'])
  a.delete(1, 1)
  return { list: a }
})

add('yarray_nested', (doc) => {
  const a = doc.getArray('list')
  const inner = new Y.Array()
  inner.push([1, 2, 3])
  const m = new Y.Map()
  m.set('inside', 'map')
  a.push([inner, m])
  return { list: a }
})

add('ymap_nested_types', (doc) => {
  const m = doc.getMap('meta')
  const nested = new Y.Map()
  nested.set('k', 'v')
  m.set('nested', nested)
  const list = new Y.Array()
  list.push([10, 20])
  m.set('list', list)
  return { meta: m }
})

add('ytext_insert', (doc) => {
  const t = doc.getText('body')
  t.insert(0, 'Hello ')
  t.insert(6, 'world')
  t.insert(11, '!')
  return { body: t }
})

add('ytext_unicode', (doc) => {
  const t = doc.getText('body')
  t.insert(0, 'a😀b你好')
  return { body: t }
})

add('ytext_delete', (doc) => {
  const t = doc.getText('body')
  t.insert(0, 'Hello world')
  t.delete(5, 6)
  return { body: t }
})

add('ytext_format', (doc) => {
  const t = doc.getText('body')
  t.insert(0, 'Hello world')
  t.format(0, 5, { bold: true })
  t.insert(5, ' brave')
  return { body: t }
})

add('ytext_embed', (doc) => {
  const t = doc.getText('body')
  t.insert(0, 'before ')
  t.insertEmbed(7, { type: 'image', src: 'a.png' })
  t.insert(8, ' after')
  return { body: t }
})

add('yxml_basic', (doc) => {
  const frag = doc.getXmlFragment('content')
  const p = new Y.XmlElement('p')
  p.setAttribute('id', '1')
  frag.insert(0, [p])
  const txt = new Y.XmlText()
  p.insert(0, [txt])
  txt.insert(0, 'hello')
  return { content: frag }
})

add('two_clients_concurrent', () => {
  const a = new Y.Doc({ gc: true })
  a.clientID = 1
  const b = new Y.Doc({ gc: true })
  b.clientID = 2
  a.getMap('meta').set('shared', 'from-a')
  a.getMap('meta').set('a-only', 1)
  b.getMap('meta').set('shared', 'from-b')
  b.getMap('meta').set('b-only', 2)
  Y.applyUpdate(b, Y.encodeStateAsUpdate(a))
  return { doc: b, roots: { meta: b.getMap('meta') } }
})

// Merge two clients where deletes were made before the merge (exercises GC'd
// items crossing the wire).
add('two_clients_delete', () => {
  const a = new Y.Doc({ gc: true })
  a.clientID = 1
  const b = new Y.Doc({ gc: true })
  b.clientID = 2
  a.getArray('list').push(['a1', 'a2'])
  b.getArray('list').push(['b1'])
  a.getArray('list').delete(1, 1)
  Y.applyUpdate(b, Y.encodeStateAsUpdate(a))
  return { doc: b, roots: { list: b.getArray('list') } }
})

const outPath = path.join(outDir, 'yjs_updates.json')
fs.writeFileSync(outPath, JSON.stringify(fixtures, null, 2) + '\n')
console.log(`wrote ${outPath}`)
for (const f of fixtures) console.log(`  ${f.name}: v1=${f.v1.length / 2}b v2=${f.v2.length / 2}b`)
