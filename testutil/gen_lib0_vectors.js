// Generates lib0 wire vectors from the reference implementation.
//
//   bun run testutil/gen_lib0_vectors.js
//
// Writes testutil/vectors/lib0_vectors.json, consumed by
// internal/lib0/conformance_vectors_test.go. Pinned to the same lib0 version
// yjs@13.6.30 resolves (see testutil/package.json).
import * as enc from 'lib0/encoding'
import * as fs from 'node:fs'
import * as path from 'node:path'
import { fileURLToPath } from 'node:url'

const here = path.dirname(fileURLToPath(import.meta.url))
const outDir = path.join(here, 'vectors')
fs.mkdirSync(outDir, { recursive: true })

const hex = (u8) => Array.from(u8, (b) => ('0' + b.toString(16)).slice(-2)).join('')

const capture = (write) => {
  const e = enc.createEncoder()
  write(e)
  return hex(enc.toUint8Array(e))
}

const vectors = {}

// VarUint (LEB128, 7 bits/byte).
vectors.varuint = [0, 1, 63, 64, 127, 128, 255, 300, 1000, 65535, 4294967295, 4294900000, 2 ** 53 - 1]
  .map((v) => ({ value: String(v), hex: capture((e) => enc.writeVarUint(e, v)) }))

// VarInt (sign-magnitude; sign bit 0x40). Restricted to int32 range: lib0's
// writeVarInt uses 32-bit shifts, and Yjs only ever writes int32-magnitude
// values through it (Any integers, IntDiffOptRle diffs).
vectors.varint = [0, 1, -1, 5, -5, 63, -63, 64, -64, 127, -127, 128, -128, 2147483647, -2147483647, -2147483648]
  .map((v) => ({ value: String(v), hex: capture((e) => enc.writeVarInt(e, v)) }))
// lib0 distinguishes -0 from +0 via math.isNegativeZero.
vectors.varint.push({ value: '-0', hex: capture((e) => enc.writeVarInt(e, -0)) })

vectors.varstring = ['', 'abc', 'Hello World!', 'Hello, 中国！𐐷𐐷𐐷']
  .map((v) => ({ value: v, hex: capture((e) => enc.writeVarString(e, v)) }))

vectors.varuint8array = [
  { bytes: [], },
  { bytes: [0, 1, 128, 254] },
  { bytes: [42, 59, 76, 157] },
].map((v) => ({ value: Buffer.from(v.bytes).toString('hex'), hex: capture((e) => enc.writeVarUint8Array(e, Uint8Array.from(v.bytes))) }))

// writeAny tags. `go` describes the value for the Go side; `skipGo` marks
// vectors Go cannot express (e.g. JavaScript -0 has no int64 equivalent).
const anyVectors = [
  { name: 'undefined', js: undefined, go: { t: 'undefined' } },
  { name: 'null', js: null, go: { t: 'null' } },
  { name: 'true', js: true, go: { t: 'true' } },
  { name: 'false', js: false, go: { t: 'false' } },
  { name: 'int0', js: 0, go: { t: 'int', v: 0 } },
  { name: 'int5', js: 5, go: { t: 'int', v: 5 } },
  { name: 'intneg5', js: -5, go: { t: 'int', v: -5 } },
  { name: 'int_max', js: 2147483647, go: { t: 'int', v: 2147483647 } },
  { name: 'int_min', js: -2147483648, go: { t: 'int', v: -2147483648 } },
  { name: 'negzero', js: -0, go: null, skipGo: true, note: 'JS -0 → tag125 + VarInt(-0); no int64 equivalent' },
  { name: 'float32_exact', js: 1.5, go: { t: 'number', v: 1.5 } },
  { name: 'float64_only', js: 1.1, go: { t: 'number', v: 1.1 } },
  { name: 'int_beyond_int32_pow2', js: 2 ** 40, go: { t: 'number', v: 2 ** 40 } },
  { name: 'int_beyond_int32_odd', js: 2 ** 40 + 1, go: { t: 'number', v: 2 ** 40 + 1 } },
  { name: 'int_safe_max', js: 2 ** 53, go: { t: 'number', v: 2 ** 53 } },
  { name: 'int_unsafe', js: 2 ** 53 + 1, go: { t: 'number', v: 2 ** 53 + 1 }, skipGo: true, note: 'JS Number loses precision; Go emits BigInt instead' },
  { name: 'bigint', js: 123n, go: { t: 'bigint', v: 123 } },
  { name: 'bigint_neg', js: -1n, go: { t: 'bigint', v: -1 } },
  { name: 'string', js: 'J. Mes', go: { t: 'string', v: 'J. Mes' } },
  { name: 'string_unicode', js: '你好𐐷', go: { t: 'string', v: '你好𐐷' } },
  { name: 'bytes', js: Uint8Array.from([42, 59, 76, 157]), go: { t: 'bytes', hex: '2a3b4c9d' } },
  { name: 'array', js: [undefined, 255, -2147483648], go: { t: 'array', v: [{ t: 'undefined' }, { t: 'int', v: 255 }, { t: 'int', v: -2147483648 }] } },
  { name: 'array_nested', js: [1, [2, 'x']], go: { t: 'array', v: [{ t: 'int', v: 1 }, { t: 'array', v: [{ t: 'int', v: 2 }, { t: 'string', v: 'x' }] }] } },
  { name: 'object', js: { age: 18, name: 'J. Mes' }, go: { t: 'object', v: { age: { t: 'int', v: 18 }, name: { t: 'string', v: 'J. Mes' } } } },
]
vectors.any = anyVectors.map((v) => ({
  name: v.name,
  go: v.go,
  skipGo: v.skipGo || false,
  note: v.note,
  hex: capture((e) => enc.writeAny(e, v.js)),
}))

// Stateful RLE encoders.
const rle = (values) => {
  const r = new enc.RleEncoder(enc.writeUint8)
  values.forEach((v) => r.write(v))
  return hex(enc.toUint8Array(r))
}
vectors.rle = [[], [1, 1, 1, 7], [1, 2, 3, 255], [0, 0, 0], [5, 5, 5, 5, 9]]
  .map((v) => ({ value: v, hex: rle(v) }))

const uintopt = (values) => {
  const r = new enc.UintOptRleEncoder()
  values.forEach((v) => r.write(v))
  return hex(r.toUint8Array())
}
vectors.uintopt = [[], [1, 2, 3, 3, 3], [0, 0], [0, 0, 0], [0, 1, 1, 0], [1, 2, 3, 65535, 18273719133]]
  .map((v) => ({ value: v, hex: uintopt(v) }))

const intdiff = (values) => {
  const r = new enc.IntDiffOptRleEncoder()
  values.forEach((v) => r.write(v))
  return hex(r.toUint8Array())
}
vectors.intdiff = [[], [1, 2, 3, 2], [0, 0], [1, 1, 1, 2, 3, 4, 5, 6], [-3, -3, 7]]
  .map((v) => ({ value: v, hex: intdiff(v) }))

const strings = (values) => {
  const r = new enc.StringEncoder()
  values.forEach((v) => r.write(v))
  return hex(r.toUint8Array())
}
vectors.stringencoder = [[], [''], ['abc'], ['Hello', 'World'], ['', 'abc', ''], ['a😀b', 'c'], ['Hello, 中国！𐐷𐐷𐐷']]
  .map((v) => ({ value: v, hex: strings(v) }))

const outPath = path.join(outDir, 'lib0_vectors.json')
fs.writeFileSync(outPath, JSON.stringify(vectors, null, 2) + '\n')
console.log(`wrote ${outPath}`)
for (const [k, v] of Object.entries(vectors)) console.log(`  ${k}: ${v.length} vectors`)
