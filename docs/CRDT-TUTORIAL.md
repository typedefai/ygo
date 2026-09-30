# From Zero to CRDTs — and on to a Go Port of Yjs

A step-by-step tutorial for experienced programmers with little prior CRDT
knowledge. By the end you should be able to look at any line of Yjs's merge
algorithm and explain why it is there, and you should know how to build (or
extend) a byte-compatible Go port.

Everything in this document is grounded in real code: this tutorial ships in the
`ygo` repository, and the list-CRDT algorithms it builds up from live in a
checkout of [josephg/reference-crdts](https://github.com/josephg/reference-crdts)
next to it (with a Go port under `reference-crdts/go/`).

| Path | What it is |
|---|---|
| `reference-crdts/` | Joseph Gentle's reference CRDTs (TypeScript) |
| `reference-crdts/go/` | The Go port of the same (6 list CRDTs, share one core) |
| `ygo/` | Your full Yjs port (wire codecs, CRDT engine, types, sync) |
| `reearth-ygo/` | A mature third-party Go port, useful as a second opinion |
| `averyyan-yjs-go/` | An abandoned half-port (do not copy; useful as a cautionary tale) |

How to use this tutorial:

1. Read a part, then run the code it references. Every sequence in this
   document was produced by the code you have.
2. Do the exercises. They are small and cumulative.
3. Only after Part IV–VII should you read Part IX–XI; they map the simple
   model onto Yjs's full machinery and the wire format.

Commands:

```bash
# list CRDTs (6 algorithms, property tests included)
cd reference-crdts/go && go test ./...          # ~30s
go run ./cmd/demo                                # see the algorithms diverge

# full Yjs port
cd ygo && go test ./... && make conformance
```

---

## Table of contents

- **Part I — Why CRDTs exist**
- **Part II — Warm-up: registers, versions, sets**
- **Part III — Sequence CRDT foundations**
- **Part IV — RGA (Automerge), step by step**
- **Part V — YATA / Yjs, step by step**
- **Part VI — Fugue**
- **Part VII — Sync9 and span splitting**
- **Part VIII — Merging, causality and anti-entropy**
- **Part IX — Yjs's full model (the target)**
- **Part X — Wire format and protocols**
- **Part XI — Roadmap to a Go port of Yjs**
- **Appendix A — Commands**
- **Appendix B — Glossary**
- **Appendix C — Reading list**
- **Appendix D — Exercise hints**

---

# Part I — Why CRDTs exist

## 1.1 The problem

You have a document that several peers can edit. Peers can be offline; messages
can arrive late, out of order, or twice. You want this guarantee:

> If every peer eventually sees every operation, every peer's document
> eventually looks the same.

The naive approach is Last-Write-Wins on the whole document (or per field).
That works for coarse-grained state but loses information for fine-grained
edits:

```
alice: insert(0, "A")   → "A"
bob:   insert(0, "B")   → "B"
```

After merging, LWW picks one; one user's insertion vanishes. That is not
acceptable for collaborative text.

## 1.2 Two families of solutions

- **Operational Transformation (OT)**: transform operations against
  concurrent operations before applying them. Correct but requires a central
  server to define a total order, and transforms are famously fiddly.
- **CRDTs (Conflict-free Replicated Data Types)**: design the data type so
  that operations *commute*. Concurrent operations can be applied in any order
  and the result is the same. No central authority needed.

Yjs, Automerge and the reference repo you have are all CRDTs.

## 1.3 The vocabulary you need

A CRDT is a data type with a **merge** function `⊔` (join) and the following
algebraic properties, which together give *Strong Eventual Consistency* (SEC):

| Property | Meaning | Why it matters |
|---|---|---|
| **Commutative** | `a ⊔ b = b ⊔ a` | message order doesn't matter |
| **Associative** | `(a ⊔ b) ⊔ c = a ⊔ (b ⊔ c)` | batching doesn't matter |
| **Idempotent** | `a ⊔ a = a` | duplicate delivery is harmless |

If merges are commutative, associative and idempotent, then any two replicas
that have seen the same set of operations have identical state — no matter in
what order or how often.

## 1.4 The two core inventions

Every CRDT you will study is built from these:

1. **Globally unique operation IDs.** Every insertion gets an ID that no other
   operation can have, e.g. `(peerID, counter)`. IDs let us refer to items
   unambiguously and break ties deterministically in a way that is the same on
   every peer.

2. **Tombstones.** A delete doesn't remove the item; it marks it. The item
   stays as an invisible anchor, because other peers' concurrent insertions
   may refer to it by ID.

Everything else is engineering: how to order concurrent insertions, how to
encode IDs compactly, how to garbage-collect tombstones.

## 1.5 State-based vs op-based

- **Op-based (operation) CRDTs** broadcast operations; each op is applied once,
  in causal order. Delete sets and Yjs "updates" are op-based-ish.
- **State-based (state) CRDTs** broadcast whole state and merge with `⊔`;
  robust to loss/reorder because of idempotence. Yjs's `EncodeStateAsUpdate` is
  a delta-state: "the ops you're missing plus the delete set", and applying the
  same update twice is harmless — so it has state-based robustness with
  op-based cost.

---

# Part II — Warm-up: registers, versions, sets

Before sequences, let's build the reflexes with tiny types. All code here is
pedagogical; the reference implementations are in `reference-crdts/go/crdts.go`
for sequences, and the ideas below reappear in Yjs's maps.

## 2.1 LWW register

A register holds one value. Last write wins, ties broken by peer ID:

```go
type LWW struct {
    value  any
    ts     int64  // Lamport-ish timestamp
    peer   string // tie-break
}

func (a *LWW) Set(value any, ts int64, peer string) {
    if ts > a.ts || (ts == a.ts && peer > a.peer) {
        a.value, a.ts, a.peer = value, ts, peer
    }
}

func (a *LWW) Merge(b *LWW) {
    if b.ts > a.ts || (b.ts == a.ts && b.peer > a.peer) {
        *a = *b
    }
}
```

Check the three properties yourself: order doesn't matter, batching doesn't
matter, duplicates don't matter. This is exactly how a `Y.Map` key resolves
concurrent writes (Yjs uses the *clock* and *client id* instead of
timestamp/peer).

## 2.2 Versions (a.k.a. state vectors)

A **version** records, per peer, the highest operation counter you have seen:
`map[peer]counter`. It answers two questions cheaply:

- "Have I already applied op `(peer, n)`?" → `version[peer] >= n`.
- "What am I missing from you?" → everything above my version.

Yjs calls this a **state vector**. In the reference code:

```go
type Version map[string]int
type Id struct { Agent string; Seq int }

func IsInVersion(id *Id, version Version) bool {
    if id == nil { return true }
    seq, ok := version[id.Agent]
    return ok && seq >= id.Seq
}
```

Important: a version is *not* a timestamp. It's a summary of causal history.

## 2.3 Counters

- **G-Counter** (grow-only): each peer keeps its own count; merge is
  element-wise `max`; the value is the sum.
- **PN-Counter**: two G-Counters (increments, decrements); merge is max on
  each; value is `inc - dec`.

These are the simplest state-based CRDTs; they show that `max` is a natural
join.

## 2.4 Add-wins OR-Set (observed-remove)

A set where concurrent `add` beats `remove`:

```go
type ORSet struct {
    // element -> set of unique add-tags
    adds map[string]map[string]bool
    // removed add-tags
    removed map[string]bool
}
```

`add(e)` creates a globally unique tag `(peer, counter)` and inserts it under
`e`. `remove(e)` adds every tag currently observed for `e` to `removed`. Merge
is set-union of both maps. `contains(e)` = any tag for `e` not in `removed`.

This is the pattern behind `Y.Map` values and `Y.Array` elements: the value is
there if some add wasn't observed-removed. Yjs's maps are LWW rather than
add-wins, but the "unique tags + tombstones" idea is the same.

## 2.5 What we learned

- Merge should be a **join**: commutative, associative, idempotent.
- Unique IDs + tombstones + deterministic tie-break = convergence.
- Versions summarise causal history.

Sequence CRDTs keep all of these and add one hard problem: **ordering**.

---

# Part III — Sequence CRDT foundations

## 3.1 Why lists are hard

For a set, "where" doesn't exist. For a list, an item's position is defined by
*history*: "between the `h` and the `i`" is not stable if those characters are
concurrently edited. So we cannot use indices on the wire.

The insight shared by all list CRDTs:

> An insertion is not "at index 5". It is "immediately after item X" (or
> "between X and Y"). Indices are computed locally from the item order.

## 3.2 The reference data model

From `reference-crdts/go/crdts.go`:

```go
type Id struct { Agent string; Seq int }

type Item struct {
    Content     any
    ID          Id

    OriginLeft  *Id // nil for start. Aka "parent" in automerge semantics.
    OriginRight *Id // yjs/yjsmod only: nil for end.
    Seq         int // automerge only
    InsertAfter bool // sync9 only

    IsDeleted bool
}

type Doc struct {
    Content []*Item // the document-ordered list
    Version Version
    Length  int
    MaxSeq  int
}
```

The whole document is one **document-ordered list**. Deleted items stay in the
list (tombstones). `Content == nil` marks a sync9 span placeholder, which we'll
meet in Part VII.

## 3.3 Two mental models

**Model A — tree (RGA/Automerge).** Each item stores its parent
(`OriginLeft`). The document is a depth-first traversal of the tree, where
siblings (same parent) are ordered by **descending `seq`, then ascending agent**. Inserting "after X" makes
X your parent.

```
null
├── A.0 "a"
│   └── A.1 "b"
└── B.0 "c"
```

**Model B — interval (YATA/Yjs).** Each item stores the item to its left and
the item to its right **at the time it was inserted** (`OriginLeft`,
`OriginRight`). The pair is a "causal interval" that must contain the new item.
Integration scans the interval and uses tie-breaks to find the exact spot.

The reference repo implements both behind one `Item` and one `Doc`, which is
what makes it such a good teaching artifact.

## 3.4 Deletes are tombstones

```go
func LocalDelete(doc *Doc, agent string, pos int) {
    item := doc.Content[findItemAtPos(doc, pos, false)]
    if !item.IsDeleted {
        item.IsDeleted = true
        doc.Length--
    }
}
```

Why not remove the item? Because a concurrent insert may have
`OriginLeft = that item`. If we removed it, the merge would have no anchor and
the insert would be lost or placed wrongly. Tombstones keep every anchor alive.
The cost is memory; Yjs's GC (Part IX.7) reclaims the *content* but keeps a
zero-width anchor.

## 3.5 Causality: you can't integrate an operation before its anchors

An item can only be integrated when its `OriginLeft`/`OriginRight` are already
present, and when all of its own agent's earlier operations are present
(sequence numbers are contiguous). The reference encodes that as:

```go
func CanInsertNow(op *Item, doc *Doc) bool {
    prev := &Id{Agent: op.ID.Agent, Seq: op.ID.Seq - 1}
    return !IsInVersion(&op.ID, doc.Version) &&
        (op.ID.Seq == 0 || IsInVersion(prev, doc.Version)) &&
        IsInVersion(op.OriginLeft, doc.Version) &&
        IsInVersion(op.OriginRight, doc.Version)
}
```

When an operation arrives before its dependencies, you **buffer** it and retry
later. The reference's `MergeInto` does exactly that:

```go
func MergeInto(algorithm *Algorithm, dest, src *Doc) {
    // ...collect missing ops...
    for remaining > 0 {
        mergedOnThisPass := 0
        for i, op := range missing {
            if op == nil || !CanInsertNow(op, dest) { continue }
            algorithm.Integrate(dest, op)
            missing[i] = nil
            remaining--
            mergedOnThisPass++
        }
        if mergedOnThisPass == 0 { panic("no progress") }
    }
}
```

Yjs calls the buffered state `pendingStructs`; we'll see it in Part IX.6.

## 3.6 Exercise 3.1

Two peers, empty document:

```
A: local insert 'x' at 0
B: local insert 'y' at 0
```

After both see each other's op, what are the possible final orders? Why is
"any causally-valid integration order gives the same result" a *requirement*,
not a hope?

(Hint: run `go run ./cmd/demo` in `reference-crdts/go` and look at the first
block. All six algorithms print `[a b]`.)

---

# Part IV — RGA (Automerge), step by step

RGA = Replicated Growable Array. Automerge is the most famous user.

## 4.1 The model

- An item's parent is `OriginLeft` (nil = root).
- The document is a pre-order traversal of the tree.
- Siblings sharing a parent are ordered by **descending `seq`** (bigger seq
  first), ties by **ascending agent**, where `seq` is a per-peer monotonic
  counter assigned at creation (not the item `ID`).

## 4.2 The code, annotated

From `reference-crdts/go/crdts.go` (the short variant used by Automerge):

```go
func integrateRGASmol(doc *Doc, newItem *Item, idxHint int) {
    agent := newItem.ID.Agent
    seq := newItem.ID.Seq
    parent := findItem(doc, newItem.OriginLeft, idxHint-1) // index of parent

    i := parent + 1
    for ; i < len(doc.Content); i++ {
        o := doc.Content[i]
        if newItem.Seq > o.Seq { break }        // fast path: all later siblings have smaller seq
        oparent := findItem(doc, o.OriginLeft, idxHint-1)
        if oparent < parent ||
            (oparent == parent && newItem.Seq == o.Seq && agent < o.ID.Agent) {
            break
        }
    }
    insertAt(doc, i, newItem)
    doc.Version[agent] = seq
    if newItem.Seq > doc.MaxSeq { doc.MaxSeq = newItem.Seq }
}
```

Reading it in prose:

1. Start scanning immediately after the parent.
2. `oparent < parent` means we have left the parent's subtree; insert before
   that.
3. `oparent == parent` means `o` is our sibling. Siblings sort by descending
   seq then ascending agent. `newItem.Seq > o.Seq` is the fast path (bigger seq
   comes first); `newItem.Seq == o.Seq && agent < o.ID.Agent` breaks the tie on
   the agent id.
4. Anything else is a descendant of a sibling: skip it (it stays within the
   sibling's subtree).

## 4.3 Trace: `interleavingBackward`

Input (from `crdts_test.go`): A inserts three items each **before** the
previous one, B does the same concurrently:

```
A.0 "a"            B.0 "b"
A.1 "a", right=A.0 B.1 "b", right=B.0
A.2 "a", right=A.1 B.2 "b", right=B.1
```

RGA ignores `OriginRight`, so each agent's items form a chain of parents
(`A.1`'s parent is the root too — `OriginLeft` is nil! only `OriginRight` points
left). So all six items are siblings at the root. Automerge orders siblings by
`(seq, agent)`:

```
automerge / interleavingBackward -> [a b a b a b]
  a [A.2] seq 2
  b [B.2] seq 2
  a [A.1] seq 1
  b [B.1] seq 1
  a [A.0] seq 0
  b [B.0] seq 0
```

Notice `seq` is *inverted* here relative to insertion order: each "insert
before" gets a larger `MaxSeq` locally, yet must appear to the left. The
reference's automerge wrapper inverts client ids to match real Automerge; the
important lesson is:

> RGA cannot see the "before" relationship. It only sees parent + seq, so two
> concurrent chains at the same parent **interleave**.

Every other algorithm in the reference produces `[a a a b b b]` here. This is
the interleaving problem, and it is the reason YATA and Fugue exist.

## 4.4 Exercise 4.1

For the simple concurrent case `A.0 "a"`, `B.0 "b"` (both parent nil, seq 0),
trace `integrateRGASmol` by hand:

- Integrating `b` into a document containing only `a`: at `i=0`, `oparent ==
  parent`, `seq == 0`, `"B" < "A"` is false → continue → insert at 1.
- Integrating `a` into a document containing only `b`: `"A" < "B"` is true →
  break → insert at 0.

Both give `[a b]`. This is the tie-break doing its job.

---

# Part V — YATA / Yjs, step by step

YATA is the algorithm in the YATA paper by Nicolaescu et al., and it is what
Yjs implements. The reference's `integrateYjsMod` and `integrateYjs` are
faithful distillations.

## 5.1 originLeft + originRight = an interval

When you insert at index `i`, you record:

- `originLeft`: the item currently at `i-1` (nil if inserting at the start),
- `originRight`: the item currently at `i` (nil if inserting at the end).

These are stable across concurrent edits: even if other items later appear
between them, your insert is still "somewhere between these two anchors".

Try the intuition: your insert happened **after** everything to the left of
`originLeft` and **before** everything to the right of `originRight`. The set
of items between the two anchors is the only place it can go. Integration must
decide exactly where among concurrent items with the same anchors.

## 5.2 The conflict scan

Start just after `originLeft`, walk right until `originRight`. For each item
`other` you meet, compare **its** anchors (`oleft`, `oright`) to yours:

- If `other` is anchored further left (`oleft < left`), it must stay left of
  you; insert here.
- If `other` has the same `oleft`:
  - if its `oright` is *left* of yours, it is a "narrower interval" — you might
    belong after it, so keep scanning (this is the `scanning` flag),
  - if its `oright` equals yours, it is a raw conflict; order by agent id,
  - if its `oright` is *right* of yours, it is wider; skip it.
- If `other` is anchored further right (`oleft > left`), skip it.

The `destIdx` variable remembers the last position you are still allowed to
insert at; `scanning` suppresses updates to `destIdx` while you are following a
possible chain.

## 5.3 The code, annotated

```go
func integrateYjsMod(doc *Doc, newItem *Item, idxHint int) {
    // Items must arrive in per-agent order; mark this one seen.
    lastSeen := -1
    if v, ok := doc.Version[newItem.ID.Agent]; ok { lastSeen = v }
    if newItem.ID.Seq != lastSeen+1 { panic("operations out of order") }
    doc.Version[newItem.ID.Agent] = newItem.ID.Seq

    left := findItem(doc, newItem.OriginLeft, idxHint-1)  // index of left anchor
    destIdx := left + 1
    right := len(doc.Content)
    if newItem.OriginRight != nil { right = findItem(doc, newItem.OriginRight, idxHint) }
    scanning := false

    for i := destIdx; ; i++ {
        if !scanning { destIdx = i }      // candidate position
        if i == len(doc.Content) { break } // ran off the end: insert at destIdx
        if i == right { break }            // reached our right anchor: insert

        other := doc.Content[i]
        oleft := findItem(doc, other.OriginLeft, idxHint-1)
        oright := len(doc.Content)
        if other.OriginRight != nil { oright = findItem(doc, other.OriginRight, idxHint) }

        if oleft < left {
            break                                   // anchored further left
        } else if oleft == left {
            if oright < right {
                scanning = true                      // follow the narrower interval
                continue
            } else if oright == right {
                if newItem.ID.Agent < other.ID.Agent { break } // raw tie-break
                scanning = false
                continue
            } else { // oright > right
                scanning = false
                continue
            }
        } else {
            continue                                 // anchored further right
        }
    }

    insertAt(doc, destIdx, newItem)
}
```

The reference comments summarise the loop as two lines:

```
yjsmod: if (oleft < left || (oleft == left && oright == right && agent < o.agent)) break
        if (oleft == left) scanning = oright < right
```

and the original Yjs differs in the middle row:

```go
} else if oleft == left {
    if newItem.ID.Agent > other.ID.Agent {   // note: inverted first
        scanning = false
        continue
    } else if oright == right {
        break
    } else {
        scanning = true
        continue
    }
}
```

`integrateYjs`'s summary:

```
yjs: if (oleft < left || (oleft == left && oright == right && agent <= o.agent)) break
     if (oleft == left) scanning = agent <= o.agent
```

`<=` vs `<` looks tiny but is observable; that's why the reference keeps both
and why `yjs` skips `withTails2` (see 5.5).

## 5.4 Worked traces (produced by the code you have)

**Concurrent inserts at the same position** — all algorithms agree:

```
concurrent A and B (all agree):
  sync9      [a b]
  yjsmod     [a b]
  fugue      [a b]
  fugueMax   [a b]
  yjs        [a b]
  automerge  [a b]
```

Trace for yjsmod, integrating `b` into `[a]`:
`left = -1`, `destIdx = 0`, `right = 1`. At `i = 0`, `other = a`:
`oleft = -1`, `oright = 1`. `oleft == left`, `oright == right`, and
`"B" < "A"` is false → `scanning = false`, continue. `i = 1` is the end →
insert at `destIdx = 1` → `[a b]`.

**Backward interleaving** — YATA keeps chains together:

```
yjsmod / interleavingBackward -> [a a a b b b]
fugue  / interleavingBackward -> [a a a b b b]
yjs    / interleavingBackward -> [a a a b b b]
automerge / interleavingBackward -> [a b a b a b]   # RGA cannot see originRight
```

**withTails** — two peers each prepend `a0`/`b0` before their own `a`/`b`,
while also appending `a1`/`b1` after:

```
yjsmod / withTails -> [a0 a a1 b0 b b1]

a0 at [A.1] (par/left [nil]) right [A.0]
a  at [A.0] (par/left [nil]) right [nil]
|  a1 at [A.2] (par/left [A.0]) right [nil]
b0 at [B.1] (par/left [nil]) right [B.0]
b  at [B.0] (par/left [nil]) right [nil]
|  b1 at [B.2] (par/left [B.0]) right [nil]
```

The indentation is the *parent tree*: `a1` is a child of `a` (its `OriginLeft`
is `A.0`), while `a0` is anchored at the root with `OriginRight = A.0`. The
document order `[a0 a a1 b0 b b1]` shows that each agent's two items stay
together, and the two peers' blocks don't interleave.

Automerge cannot see `OriginRight`, so `a0`/`b0` (both at the root, seq 1)
interleave:

```
automerge / withTails -> [a0 b0 a a1 b b1]
```

## 5.5 Why `originRight` matters, and the `withTails2` quirk

`originRight` is what lets YATA place items that were inserted **before** an
item that has since been deleted, without waiting: the deleted item survives as
a tombstone and still serves as the right anchor.

`withTails2` is the same puzzle but peer B's `b0` has the agent id `"1"`
instead of `"B"` (a different agent that could be any other peer). Since all
tie-breaks are by agent id, this changes the order:

```
yjsmod / withTails2 -> [a0 a a1 b0 b b1]
yjs    / withTails2 -> [b0 a0 a a1 b b1]     # original Yjs's <= rule
```

The reference marks `withTails2` as `IgnoreTests` for `yjs`. It is not a bug in
either implementation; it is a semantic difference between "Yjs" and "Yjs with
a tweak". The reference's author chose to compare them separately.

## 5.6 Exercise 5.1

Hand-trace `integrateYjsMod` for:

```
newItem = X.0 "x", originLeft = A.0, originRight = nil
document = [A.0 "a", C.0 "c", B.0 "b"]
where C.0 has originLeft = A.0, originRight = nil
      B.0 has originLeft = A.0, originRight = nil
```

At which `i` does it break, and what is the final order? (Answer in
Appendix D.)

---

# Part VI — Fugue

## 6.1 The goal: maximal non-interleaving

YATA works well in practice but there are rare cases where one peer's run of
insertions can interleave with another's. The Fugue paper ("The Art of the
Fugue", Weidner & Gentle, arXiv:2305.00583) defines a stronger property
(*maximal non-interleaving*) and modifies the anchor rule to achieve it.

## 6.2 The key idea: the right parent

Fugue replaces "search for `originRight`" with "find the **right parent**":

> Given an item `other`, its right parent is the first item to its right whose
> `originLeft` equals `other.originLeft`; if none, the end of the document.

The reference implements it as:

```go
getRightParentIdx := func(item *Item) int {
    rightIdx := endIdx
    if item.OriginRight != nil { rightIdx = findItem(doc, item.OriginRight, idxHint) }
    if rightIdx >= len(doc.Content) { return endIdx }
    right := doc.Content[rightIdx]
    if !idEq(right.OriginLeft, item.OriginLeft) { return endIdx }
    return rightIdx
}
```

Then the scan is yjsmod's punnet square with `rightPIdx` in place of the
`originRight` index:

```go
if oleftIdx < leftIdx ||
    (oleftIdx == leftIdx && orightPIdx == rightPIdx && newItem.ID.Agent < other.ID.Agent) {
    break
}
if oleftIdx == leftIdx {
    scanning = orightPIdx < rightPIdx
}
```

Fugue's merge order is identical to Sync9's (Part VII). The reference even
notes that `FugueMax` validates as equivalent to `yjsMod` — so if you
understand yjsmod's loop, you understand most of Fugue's too.

## 6.3 Trace

From the demo and test run:

```
withTails:
  fugue -> [a0 a a1 b0 b b1]
  fugueMax -> [a0 a a1 b0 b b1]     (reuses yjsmod's integrate)
```

The difference from yjsmod shows up in adversarial interleaving cases, where
Fugue's "right parent" makes the scan stop at the correct sibling boundary
instead of walking into a neighbouring chain.

## 6.4 Exercise 6.1

In `reference-crdts/go/crdts.go`, change `Fugue`'s `integrate` to
`integrateYjs` and run the test suite. Which tests fail? This shows that the
differences between the algorithms are *only* in this function — everything
else (ids, versions, merge, deletion) is shared.

---

# Part VII — Sync9 and span splitting

Sync9 (Seph Gentle's "loom") uses a different anchor model:

- `OriginLeft` is the **parent**, not necessarily the immediate left neighbour.
- `InsertAfter` chooses whether you go before or after the parent's own
  content.
- Items are **spans**: an item with `Content == nil` is a zero-length
  placeholder used to split another item, so that "before this content" has a
  distinct position from "after this content".

## 7.1 `integrateSync9`, annotated

```go
parentIdx := findItem2(doc, newItem.OriginLeft, newItem.InsertAfter, idxHint-1)
destIdx := parentIdx + 1

if parentIdx >= 0 && newItem.OriginLeft != nil && !newItem.InsertAfter &&
    doc.Content[parentIdx].Content != nil {
    // We want to insert BEFORE the parent's content. Split the parent into a
    // zero-length placeholder plus its content, then insert before the
    // content. We know we are an only child, so skip the scan.
    split := *doc.Content[parentIdx]
    split.Content = nil
    doc.Content = insertAtRaw(doc.Content, parentIdx, &split)
} else {
    // Otherwise scan right for the next sibling, skipping the parent's
    // children (they have oparentIdx == parentIdx).
    for ; destIdx < len(doc.Content); destIdx++ {
        other := doc.Content[destIdx]
        oparentIdx := findItem2(doc, other.OriginLeft, other.InsertAfter, idxHint-1)
        if oparentIdx < parentIdx { break }
        if oparentIdx == parentIdx {
            if newItem.ID.Agent < other.ID.Agent { break }
            continue
        }
    }
}
```

`findItem2(..., atEnd=true)` matches only items **with content**: the "end" of
a span is the position after its content, and the placeholder represents the
position before it.

## 7.2 Why spans exist

In a model where an item is a single character, "insert before X" and "insert
after the item before X" are the same position. Real rich-text editing needs
both (e.g. inserting a formatting marker that attaches to the *left* edge of a
character). Sync9 models that with spans; Yjs achieves the same thing by
splitting items (Part IX.5).

## 7.3 Exercise 7.1

Run `interleavingBackward2` (in the tests) mentally. Sync9 produces
`[a a b b]`; write down the placeholder items that get created and where.

---

# Part VIII — Merging, causality and anti-entropy

## 8.1 The reference's merge

You already saw `MergeInto`: collect ops missing from `dest.version`, then
repeatedly integrate whatever is causally ready. This is **anti-entropy**: two
peers exchange what the other lacks.

The reference's `CanInsertNow` plus buffering guarantees:

- every op is integrated exactly once,
- every op is integrated after its dependencies,
- therefore `version` is always a valid summary of what the peer saw.

## 8.2 State vectors are an optimization, not a definition

`Version`/`StateVector` lets you skip already-seen ops. The correctness comes
from the CRDT, not the version.

## 8.3 Yjs's delta-state updates

Yjs does not send "operations" one by one over the wire. It sends an
**update**:

```
update = [structs the receiver is missing] + [full delete set]
```

Applying an update is idempotent: structs already present are skipped by
clock, and deletes already applied are no-ops. That single design decision is
why Yjs works over lossy, reordered transports (WebSocket, HTTP polling,
offline-first). We'll decode one byte-by-byte in Part X.

## 8.4 Exercise 8.1 (anti-entropy budget)

Two peers each made 1000 single-character inserts from a shared initial
document of 1000 characters. Compare bytes sent by:

1. naive "send the whole document each time",
2. "send ops since my version",
3. "send an update computed against the peer's state vector".

Which does Yjs do, and when? (Hint: `EncodeStateAsUpdate(doc, remoteSV)`.)

---

# Part IX — Yjs's full model (the target)

Now we leave the single-character list and enter the real Yjs data model. The
reference's algorithms all survive, but they are wrapped in a type system,
splitting, transactions, delete sets and encodings.

## 9.1 Items live in a `Parent`, not just a document list

Yjs generalises the list into shared types:

| Type | Model |
|---|---|
| `YArray` | ordered list of items (exactly the reference's model) |
| `YText` | ordered list where strings are run-length items, plus `ContentFormat` markers |
| `YMap` | items keyed by `parentSub` (the key); LWW by clock/client |
| `YXmlFragment/Element/Text` | ordered lists + attributes (a map) + a tag name |

On the wire, every item stores a `parent` which is either:

- a root type name (e.g. `"body"`), or
- the ID of the item that contains a nested type.

In `ygo`:

```go
// pkg/ygo/types.go
type Parent struct {
    Named *string // root type name
    ID    *ID     // parent item id
}
```

Map keys are `Item.ParentSub *string`. So the same integration algorithm
handles arrays and map entries; only the conflict scan's starting point differs
(`parent.start` vs `parent.mapGet(key)`).

## 9.2 IDs, clocks and clients

```
ID{Client uint32, Clock uint64}
```

- `Client` is a random 32-bit peer id.
- `Clock` is a per-client, strictly increasing counter. Every insertion
  consumes `len(content)` clocks; deletions do **not** consume clocks.
- Items hold a contiguous clock range `[Clock, Clock+Len)`.

The reference's `(agent, seq)` maps exactly onto `(Client, Clock)`. The crucial
upgrade is that Yjs items have **length > 1** (a whole string, an Any run, a
nested type), which is where splitting comes from.

## 9.3 The Item

From `ygo/pkg/ygo/block.go`:

```go
type Item struct {
    ID          ID
    Length      uint64
    Left, Right *Item
    Origin      *ID   // reference's OriginLeft
    RightOrigin *ID   // reference's OriginRight
    Content     ItemContent
    Parent      *Parent
    ParentSub   *string
    Info        ItemFlags
}
```

`ygo`'s naming is yrs-shaped (`Origin` vs `originLeft`), but the semantics are
the reference's. Content types (`StringContent`, `AnyContent`, `FormatContent`,
`TypeContent`, `DocContent`, ...) correspond to Yjs's `ContentString`,
`ContentAny`, `ContentFormat`, `ContentType`, `ContentDoc`.

## 9.4 StructStore

Yjs stores items **per client, in clock order**, in a `StructStore`:

```go
type BlockStore struct {
    clients map[ClientID]ClientBlockList // sorted by clock
}
```

This is the one big departure from the reference's single document-ordered
slice. Why? Because concurrent updates arrive per client, and the store is the
canonical, searchable history; the *document order* is the linked list
`Left`/`Right` inside each type.

`ygo/pkg/ygo/block_store.go` implements:

- `Find(id)` via binary search over clock ranges,
- `GetItemCleanStart(id)` / `GetItemCleanEnd(id)` which **split** an item if
  the id falls inside it.

## 9.5 Splitting: the single most important upgrade

In the reference, an item is one character, so `findItem` never has to split.
Yjs items span clock ranges, and remote operations can refer to a clock *inside*
an item. So Yjs splits on demand:

```
item "hello"  [0,5)
GetItemCleanStart({c,2})  →  "he" [0,2)   "llo" [2,5)
```

`ygo`'s `Item.SplitAt(offset)` mutates the receiver into the left half and
returns the right half — critical so that `parent.start`, map entries and
neighbours keep pointing at the same object (this was a real bug we hit and
fixed). The split also repairs the content: `StringContent` splits on UTF-16
boundaries (Yjs indexes text in UTF-16 code units, like JavaScript strings).

Splitting is also how a whole string insert becomes many items: text-editing
transactions split and merge run-length items constantly.

## 9.6 Integration in Yjs

Yjs's `Item.integrate` is the reference's `integrateYjs` plus:

1. **Dependency resolution** (`getMissing`): turn `Origin`/`RightOrigin` ids
   into live neighbours via `GetItemCleanEnd`/`GetItemCleanStart`; if a
   dependency is missing, return it so the caller can park the item.
2. **Parent resolution**: `Parent.Named` → root type; `Parent.ID` → the
   `ContentType` of that item; otherwise inherit from the left/right neighbour.
   `ParentSub` is inherited from the origin too — this is why a YMap item on
   the wire doesn't repeat its key when it has a left origin.
3. **GC fallback**: if a neighbour is a GC struct (its content was collected),
   the item integrates as a GC struct instead.
4. **Squashing**: after a transaction, adjacent same-client items merge
   (`tryToMergeWithLefts`), which is how 1000 keystrokes become one item.
5. **Pending**: items that arrive before their dependencies are stored in
   `pendingStructs` and retried. Deletes referencing missing items go to
   `pendingDs`.

In `ygo`, read `pkg/ygo/item_integrate.go` (`resolveDeps`, `integrate`,
`deleteItem`, `mergeWith`), then `pkg/ygo/apply.go` (`integrateStructs`,
`applyDeleteSet`) alongside the reference's `integrateYjsMod`. You will see the
punnet square from Part V, now with `findItem` replaced by
`GetItemCleanEnd`/`GetItemCleanStart`.

## 9.7 Deletes, delete sets and GC

Yjs deletes are not encoded in items. They are a separate **DeleteSet**:
`map[ClientID][]{Clock, Len}` ranges. This is the OR-Set "removed tags"
generalised to clock ranges.

- Deleting an item marks it `deleted` and appends its range to the
  transaction's delete set.
- Maps resolve concurrent `set`/`delete` by clock and client (LWW).
- **GC**: after a transaction, deleted items' *content* is replaced with
  `ContentDeleted` (or the item becomes a `GC` struct), freeing memory while
  keeping the anchor. `crdt.WithGC(true)` is the default.
- The `keep` flag (used by UndoManager) prevents a specific item from being
  collected.

`ygo/pkg/ygo/transaction.go` has `tryGcDeleteSet` and `tryMergeDeleteSet`.

## 9.8 Encoding state and diffs

- `EncodeStateVector(doc)` → `{client: clock}` summary (Part II.2).
- `EncodeStateAsUpdate(doc, remoteSV)` → only structs the remote lacks, with
  the first struct written at an **offset** if the remote's clock cuts into
  it (an item can be re-encoded as its suffix).
- `MergeUpdates([...])` → fold several updates into one, filling clock gaps
  with `Skip` structs.

All of this is exercised byte-for-byte in `ygo` by fixtures generated from
real `yjs@13.6.30` (see Part X.6).

## 9.9 Exercise 9.1

Open `ygo/pkg/ygo/apply.go` and find where `pendingStructs` is retried. Then
explain, in terms of `CanInsertNow`, why the retry only needs to happen after a
new update is integrated.

---

# Part X — Wire format and protocols

An understanding-level tour; exact byte walks are in the fixtures and in
`ygo`'s encoder/decoder.

## 10.1 lib0 primitives

Yjs's binary format is `lib0`:

| Primitive | Encoding |
|---|---|
| VarUint | LEB128: 7 bits per byte, MSB = continuation |
| VarInt | sign-magnitude: first byte holds 6 magnitude bits + sign bit (0x40) + continuation (0x80). There is a distinct **-0** (`0x40`), which run-length encoders use as a marker |
| VarString | VarUint(byte length) + UTF-8 bytes |
| Any | tagged union: 119 string, 125 int (VarInt), 124 float32, 123 float64, 122 bigint, 118 object, 117 array, 116 bytes, 120/121 bool, 126 null, 127 undefined |

Two traps we hit in the real port:

- **IDs are VarUint, not VarInt.** Signed and unsigned encodings diverge above
  63, so using the wrong one corrupts every real client id.
- **`-0` matters.** A run of zeros in a UIntOptRle column is encoded as `-0`
  (`0x40`); collapsing it to `+0` desynchronises the decoder.

## 10.2 Update V1, byte by byte

Take `ygo/testutil/fixtures/yjs_updates.json`'s `ytext_insert`:

```
01010100040104626f64790c48656c6c6f20776f726c642100
```

Split it:

```
01             numClients = 1
  01           numberOfStructs = 1
  01           client = 1
  00           clock = 0
  04           info: content tag 4 = ContentString, no origin flags
  01 04 626f6479   parent: named root, "body"
  0c 48656c6c6f20776f726c6421   content: 12 chars "Hello world!"
01             delete set: 0 clients
```

Note the per-client order: **numberOfStructs, client, clock** — not
`client, count, clock`. And clients are written in descending id order. Both
were bugs we found only because the fixtures came from real Yjs.

A richer example, `ymap_dupkey` (setting the same map key three times):

```
01 02 01 00   numClients=1, numStructs=2, client=1, clock=0
21            info = HAS_PARENT_SUB | ContentDeleted (tag 1)
01 04 6d657461  parent "meta"
01 6b           parentSub "k"
02              deleted content length 2   ← first two values GC'd+squashed
a8            info = HAS_ORIGIN | HAS_PARENT_SUB | ContentAny (tag 8)
01 01           origin = (client 1, clock 1)  ← no parent info written
01 77 05 7468697264   Any run of 1: string "third"
01 01 01 00 02   delete set: client 1, one range (0,2)
```

Two lessons visible here:

- Deleted content is replaced by `ContentDeleted` (GC), and adjacent deleted
  items are squashed into one range.
- An item with an origin doesn't write its `parentSub`; the receiver inherits
  it from the origin during integration. Re-encoding without integration loses
  that bit (one of the byte-diff bugs we hit).

## 10.3 Update V2: columns

V2 groups fields into run-length-encoded streams. `ytext_insert` V2:

```
00                         feature flag
00                         keyClock column (empty)
01 01                      client column: [1]
00                         leftClock column (empty)
00                         rightClock column (empty)
01 04                      info column: [String]
13 10 626f6479 48656c6c6f20776f726c6421 04 0c
                           string column: VarString("bodyHello world!")
                           + UIntOptRle lengths [4,12]
01 01                      parentInfo column: [named]
00                         typeRef column (empty)
00                         len column (empty)
01 01 00 00                rest: numClients=1, numStructs=1, clock=0, ds=0
```

The first nine columns are length-prefixed; the trailing `rest` is **raw**.
That detail matters: prefix the rest column and no Yjs decoder will read your
update.

`MergeUpdates` (filling gaps with `Skip`, deduplicating overlaps) and V1↔V2
conversion are separate algorithms; `ygo/pkg/ygo/merge.go` and
`encode_state.go` are the places to read after this tutorial.

## 10.4 Delete sets and state vectors

- DeleteSet wire: `VarUint(clientCount)` then per client
  `VarUint(client), VarUint(rangeCount), [clock, len]*`. V2 delta-encodes the
  clock/len within a client.
- StateVector wire: `VarUint(count)` then `[client, clock]*`.

## 10.5 y-protocols: sync and awareness

- Sync framing: `VarUint(messageType) VarUint8Array(payload)` with
  `0 SyncStep1` (state vector), `1 SyncStep2` (missing update),
  `2 Update` (incremental). See `ygo/pkg/ygo/sync`.
- Awareness is separate ephemeral state (cursors, names) with its own clocks
  and LWW: `VarUint(count)` then `(client, clock, JSON state)*`, where a null
  state means "client left". See `ygo/pkg/ygo/awareness`.

## 10.6 How to know your implementation is compatible

You cannot reason your way to byte compatibility; you must diff against the
reference. `ygo` does this with a `bun`-driven harness:

- `testutil/gen_fixtures.js` produces real Yjs updates for maps, arrays, text,
  formatting, XML, GC and merges.
- Go tests decode them, integrate them, and re-encode — asserting the bytes are
  identical.
- `testutil/verify_go_updates.js` runs the reverse: it feeds Go-produced bytes
  to Yjs and compares the resulting document.

That harness is how every subtle bug in Parts IX–X was found. Build it *before*
you need it.

---

# Part XI — Roadmap to a Go port of Yjs

You already have a working port (`ygo`), conformance-verified against
`yjs@13.6.30`. If you were starting from scratch, this is the order that keeps
you honest at every step.

## 11.1 Milestones

| # | Milestone | Exit criterion |
|---|---|---|
| 0 | Differential fixtures | Real Yjs bytes/cases loadable in tests |
| 1 | lib0 codec | Hex vectors match lib0 exactly |
| 2 | Item/content + Store + splitting | decode → re-encode fixtures byte-identical (V1+V2) |
| 3 | YATA integration + transactions + apply | apply every fixture, re-encode full state byte-identical |
| 4 | State-vector diffs, merge, pending | diff/merge fixtures byte-identical; out-of-order apply converges |
| 5 | Public types + local writes + observers | same op sequences produce byte-identical updates |
| 6 | sync + awareness | y-protocols fixtures byte-identical; peer convergence |

Do not skip step 0. Every bug we found in `ygo` was invisible to
self-consistency tests and visible to the reference fixtures.

## 11.2 Testing strategy

Three layers, strongest last:

1. **Unit tests** of codecs against hand-computed vectors.
2. **Property fuzzers** (like the reference's `integrateFuzz`): integrate the
   same operations in many random causally-valid orders; assert the order
   never changes; merge N docs and assert convergence.
3. **Differential tests against real Yjs**: byte-for-byte round trips plus a
   semantic verifier that replays Go bytes in JS.

## 11.3 Pitfall checklist (all of these were real bugs)

- VarInt vs VarUint for ids/clients (breaks for values ≥ 64).
- `-0` in UIntOptRle run markers.
- V2: the `rest` column is raw; `numberOfStructs, client, clock` order;
  descending client order.
- Content JSON: Yjs's `ContentJSON` is an array of `Any`, not an array of
  strings.
- `writeAny`'s number dispatch: integers in `±(2^31-1)` use VarInt; larger
  reals use float32/float64; beyond 2^53 use bigint.
- Object key order: JS preserves insertion order; Go maps don't. Keep the
  decoded order when re-encoding.
- `parentSub` is inherited at integration, not always on the wire.
- Splitting must mutate the original into the left half, or `parent.start` and
  map pointers go stale.
- `EncodeStateAsUpdate` must include pending structs (with `Skip` gaps).
- GC + squashing: re-encoding must reproduce Yjs's tombstone boundaries.

## 11.4 Performance notes

The reference is deliberately slow; real Yjs adds:

- **run-length items and squashing** (fewer list nodes),
- **search markers** / an LRU index cache for `findItemAtPos`,
- **binary search** over per-client clock arrays,
- **columnar V2 + RLE** for bytes-on-wire,
- arena-friendly allocation (yrs) or pooling.

Correctness first; then measure with a real editing trace.

## 11.5 Capstone

Extend `reference-crdts/go` so that one algorithm (`Fugue`) gains:

1. multi-character items with splitting, and
2. a simple binary encoding of the whole document,

then write the property tests from `crdts_test.go` against the new
representation. When those pass, you have re-derived the core of Yjs.

---

# Appendix A — Commands

```bash
# Reference list CRDTs
cd reference-crdts/go
go test ./...            # all six algorithms + property fuzzers (~30 s)
go test -run TestAlgorithms/yjsmod -v
go run ./cmd/demo        # compare algorithms on the puzzles

# Your Yjs port
cd ygo
go test ./...
make conformance         # regenerate fixtures (bun) + run go tests + JS verifier
```

# Appendix B — Glossary

| Term | Meaning |
|---|---|
| CRDT | Conflict-free Replicated Data Type |
| SEC | Strong Eventual Consistency: same ops seen ⇒ same state |
| RGA | Replicated Growable Array (Automerge's model) |
| YATA | The integration algorithm used by Yjs |
| originLeft / parent | The left anchor / tree parent of an insert |
| originRight | The right anchor; unique to YATA-style models |
| tombstone | A deleted item kept as an anchor |
| GC | Replacing a tombstone's content with a placeholder |
| state vector / version | Per-client highest clock/seq seen |
| struct | Yjs's term for an item (or GC/Skip placeholder) |
| Skip | A clock range only present to keep encodings aligned |
| pendingStructs | Buffered items waiting on missing dependencies |
| delete set | Per-client list of deleted clock ranges |

# Appendix C — Reading list

1. **Yjs `INTERNALS.md`** — the canonical short spec of the YATA algorithm and
   data structures.
2. **YATA paper** — Nicolaescu, Jahns, Derntl, Klamma, *Near Real-time
   Peer-to-peer Shared Editing on Extensible Data Types* (2016).
3. **RGA paper** — Roh, Jeon, Kim, Lee, *Replicated Abstract Data Types*
   (2011).
4. **The Art of the Fugue** — Weidner & Gentle, arXiv:2305.00583.
5. **Automerge papers** — Kleppmann & Beresford, *A Conflict-Free Replicated
   JSON Datatype*; and the Automerge 2.0 columnar encoding write-up.
6. **Shapiro et al.**, *A comprehensive study of Convergent and Commutative
   Replicated Data Types* (2011) — the formal foundation.
7. **Joseph Gentle**, *5000x Faster CRDTs: An Adventure in Optimization* —
   the performance mindset behind this reference repo.
8. `reearth-ygo/docs/ARCHITECTURE.md` and `INTERNALS.md` — a production Go
   port's design notes.

# Appendix D — Exercise hints

- **3.1**: `[x y]` and `[y x]` are both valid if the algorithm has no
  tie-break; the CRDT's tie-break (agent id) makes exactly one of them the
  required answer. Integration order must not change which one.
- **5.1**: `leftIdx = 0` (`A.0`). At `i = 1`, `other = C.0` with `oleft = 0`,
  `oright = end = 4`; `oright < right` is false and `oright == right`, and
  `"X" < "C"` is false, so continue. At `i = 2`, `other = B.0` with the same
  anchors; `"X" < "B"` is false, continue. `i = 3` is the end → insert at
  `destIdx = 3`, i.e. `[A C B X]`. If `X.0`'s agent were `"1"`, it would break
  at `i = 1` and land before both.
- **6.1**: `Fugue` (and possibly `automerge`-ignored) tests; yjsmod/fugue are
  not interchangeable with yjs in the tie-break cases (`withTails2`).
- **7.1**: `a0` splits the placeholder before `a`'s content; `a1` attaches
  after `a`'s content. Sync9's tree print (`go run ./cmd/demo`) shows the
  before/after markers.
- **8.1**: Yjs sends the delta against the peer's state vector, plus the full
  delete set; repeated application is idempotent, so re-sending is safe when
  unsure.
- **9.1**: retry only after integrating a new update because pending items are
  blocked on ids that the new update may have just supplied (`CanInsertNow`
  flips from false to true).
