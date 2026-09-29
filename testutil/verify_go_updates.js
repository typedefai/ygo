// Verifies Go's decode -> re-encode of Yjs updates semantically, for both V1
// and V2.
//
//   1. go test ./pkg/ygo/ -run TestYjsV
//   2. bun run testutil/verify_go_updates.js
//
// For each fixture and version, applies the original update and Go's
// re-encoded update to fresh yjs documents and checks:
//   - Go's bytes apply without error,
//   - the resulting stores are identical (same canonical encodeStateAsUpdate),
//   - the roots match the expected JSON captured at generation time.
import * as Y from 'yjs'
import * as fs from 'node:fs'
import * as path from 'node:path'
import { fileURLToPath } from 'node:url'

const here = path.dirname(fileURLToPath(import.meta.url))
const fixturesPath = path.join(here, 'fixtures', 'yjs_updates.json')

const hex = (u8) => Array.from(u8, (b) => ('0' + b.toString(16)).slice(-2)).join('')
const fromHex = (s) => Uint8Array.from(Buffer.from(s, 'hex'))

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

const getRoot = (doc, kind, name) => {
  switch (kind) {
    case 'map': return doc.getMap(name)
    case 'array': return doc.getArray(name)
    case 'text': return doc.getText(name)
    case 'xml': return doc.getXmlFragment(name)
    default: throw new Error(`unknown root kind ${kind}`)
  }
}

const rootsPlain = (doc, kinds) => {
  const out = {}
  for (const [k, kind] of Object.entries(kinds)) out[k] = toPlain(getRoot(doc, kind, k))
  return out
}

const stable = (v) => JSON.stringify(v)

const applyV1 = (updateHex) => {
  const doc = new Y.Doc({ gc: true })
  Y.applyUpdate(doc, fromHex(updateHex))
  return doc
}

const applyV2 = (updateHex) => {
  const doc = new Y.Doc({ gc: true })
  Y.applyUpdateV2(doc, fromHex(updateHex))
  return doc
}

const fixtures = JSON.parse(fs.readFileSync(fixturesPath, 'utf8'))

const versions = [
  { tag: 'v1', report: 'go_v1_reencoded.json', applySource: applyV1, applyTarget: applyV1, source: 'v1' },
  { tag: 'v2', report: 'go_v2_reencoded.json', applySource: applyV2, applyTarget: applyV2, source: 'v2' },
  { tag: 'v1->v2', report: 'go_v1_to_v2.json', applySource: applyV1, applyTarget: applyV2, source: 'v1' },
  { tag: 'v2->v1', report: 'go_v2_to_v1.json', applySource: applyV2, applyTarget: applyV1, source: 'v2' },
]

let pass = 0
let fail = 0
for (const v of versions) {
  const reportPath = path.join(here, 'fixtures', v.report)
  if (!fs.existsSync(reportPath)) {
    console.log(`SKIP ${v.tag}: ${v.report} not found (run the Go fixture tests)`)
    continue
  }
  const byName = new Map(JSON.parse(fs.readFileSync(reportPath, 'utf8')).map((r) => [r.name, r]))
  for (const f of fixtures) {
    const go = byName.get(f.name)
    if (!go) {
      console.log(`FAIL ${v.tag}/${f.name}: no Go re-encode entry`)
      fail++
      continue
    }
    const reasons = []
    try {
      const origDoc = v.applySource(f[v.source])
      const goDoc = v.applyTarget(go.hex)
      const origCanonical = hex(Y.encodeStateAsUpdate(origDoc))
      const goCanonical = hex(Y.encodeStateAsUpdate(goDoc))
      if (origCanonical !== goCanonical) reasons.push('canonical store encoding differs')

      const origPlain = rootsPlain(origDoc, f.kinds)
      const goPlain = rootsPlain(goDoc, f.kinds)
      if (stable(origPlain) !== stable(goPlain)) reasons.push('root JSON differs')
      if (stable(origPlain) !== stable(f.expected)) reasons.push('original does not match expected fixture JSON')
    } catch (e) {
      reasons.push(`apply error: ${e}`)
    }
    if (reasons.length) {
      console.log(`FAIL ${v.tag}/${f.name} (go-byte-identical=${go.identical}): ${reasons.join('; ')}`)
      fail++
    } else {
      console.log(`PASS ${v.tag}/${f.name}${go.identical ? ' (byte-identical)' : ''}`)
      pass++
    }
  }
}

// ── mergeUpdates ─────────────────────────────────────────────────────────────
// Our merged update must be semantically identical to applying all inputs
// sequentially, for both wire formats.
const mergesPath = path.join(here, 'fixtures', 'yjs_merges.json')
const mergesReportPath = path.join(here, 'fixtures', 'go_merges.json')
if (fs.existsSync(mergesReportPath)) {
  const merges = JSON.parse(fs.readFileSync(mergesPath, 'utf8'))
  const goMerges = new Map(JSON.parse(fs.readFileSync(mergesReportPath, 'utf8')).map((r) => [r.name, r]))
  for (const m of merges) {
    const go = goMerges.get(m.name)
    const reasons = []
    if (!go) {
      reasons.push('no Go merge entry')
    } else {
      try {
        const seq1 = new Y.Doc({ gc: true })
        m.updatesV1.forEach((u) => Y.applyUpdate(seq1, fromHex(u)))
        const yjsV1 = applyV1(m.v1)
        const goV1 = applyV1(go.v1)
        const cSeq1 = hex(Y.encodeStateAsUpdate(seq1))
        if (cSeq1 !== hex(Y.encodeStateAsUpdate(yjsV1))) reasons.push('fixture: merged V1 != sequential apply')
        if (cSeq1 !== hex(Y.encodeStateAsUpdate(goV1))) reasons.push('Go merged V1 differs semantically')

        const seq2 = new Y.Doc({ gc: true })
        m.updatesV2.forEach((u) => Y.applyUpdateV2(seq2, fromHex(u)))
        const yjsV2 = applyV2(m.v2)
        const goV2 = applyV2(go.v2)
        const cSeq2 = hex(Y.encodeStateAsUpdateV2(seq2))
        if (cSeq2 !== hex(Y.encodeStateAsUpdateV2(yjsV2))) reasons.push('fixture: merged V2 != sequential apply')
        if (cSeq2 !== hex(Y.encodeStateAsUpdateV2(goV2))) reasons.push('Go merged V2 differs semantically')
      } catch (e) {
        reasons.push(`apply error: ${e}`)
      }
    }
    if (reasons.length) {
      console.log(`FAIL merge/${m.name}: ${reasons.join('; ')}`)
      fail++
    } else {
      const tags = `${go.identicalV1 ? ' (v1 byte-identical)' : ''}${go.identicalV2 ? ' (v2 byte-identical)' : ''}`
      console.log(`PASS merge/${m.name}${tags}`)
      pass++
    }
  }
}

console.log(`\n${pass} passed, ${fail} failed`)
process.exit(fail === 0 ? 0 : 1)
