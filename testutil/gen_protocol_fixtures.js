// Generates y-protocols sync and awareness fixtures.
//
//   bun run testutil/gen_protocol_fixtures.js
//
// Writes testutil/fixtures/yjs_protocol.json.
import * as Y from 'yjs'
import * as encoding from 'lib0/encoding'
import * as syncProtocol from 'y-protocols/sync'
import * as awarenessProtocol from 'y-protocols/awareness'
import * as fs from 'node:fs'
import * as path from 'node:path'
import { fileURLToPath } from 'node:url'

const here = path.dirname(fileURLToPath(import.meta.url))
const outDir = path.join(here, 'fixtures')
fs.mkdirSync(outDir, { recursive: true })

const hex = (u8) => Array.from(u8, (b) => ('0' + b.toString(16)).slice(-2)).join('')

const syncFixtures = []
const addSync = (name, build) => {
  const doc = new Y.Doc({ gc: true })
  doc.clientID = 1
  build(doc)
  const full = Y.encodeStateAsUpdate(doc)

  const e1 = encoding.createEncoder()
  syncProtocol.writeSyncStep1(e1, doc)
  const e2 = encoding.createEncoder()
  syncProtocol.writeSyncStep2(e2, doc)
  const e3 = encoding.createEncoder()
  syncProtocol.writeUpdate(e3, full)

  const sv = {}
  Y.decodeStateVector(Y.encodeStateVector(doc)).forEach((clock, client) => {
    sv[String(client)] = clock
  })
  syncFixtures.push({
    name,
    full: hex(full),
    step1: hex(encoding.toUint8Array(e1)),
    step2: hex(encoding.toUint8Array(e2)),
    update: hex(encoding.toUint8Array(e3)),
    sv,
  })
}

addSync('sync_text', (doc) => {
  doc.getText('body').insert(0, 'hello world')
})
addSync('sync_map', (doc) => {
  const m = doc.getMap('meta')
  m.set('a', 1)
  m.set('b', 'two')
  m.set('c', true)
})

const awarenessFixtures = []
{
  const doc = new Y.Doc({ gc: true })
  doc.clientID = 1
  const aw = new awarenessProtocol.Awareness(doc)
  aw.setLocalStateField('user', { name: 'alice', color: '#f00' })
  awarenessFixtures.push({
    name: 'awareness_basic',
    clientID: 1,
    update: hex(awarenessProtocol.encodeAwarenessUpdate(aw, [1])),
    states: aw.getStates().get(1),
    clock: aw.meta.get(1).clock,
  })
}
{
  const doc = new Y.Doc({ gc: true })
  doc.clientID = 2
  const aw = new awarenessProtocol.Awareness(doc)
  aw.setLocalStateField('cursor', 42)
  awarenessFixtures.push({
    name: 'awareness_cursor',
    clientID: 2,
    update: hex(awarenessProtocol.encodeAwarenessUpdate(aw, [2])),
    states: aw.getStates().get(2),
    clock: aw.meta.get(2).clock,
  })
}

const out = { sync: syncFixtures, awareness: awarenessFixtures }
const outPath = path.join(outDir, 'yjs_protocol.json')
fs.writeFileSync(outPath, JSON.stringify(out, null, 2) + '\n')
console.log(`wrote ${outPath}`)
