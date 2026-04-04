package common

type Range struct {
	Start uint64
	End   uint64
}

func NewRange(start uint64, end uint64) *Range {
	return &Range{start, end}
}

func (r *Range) IsEmpty() bool {
	return !(r.Start < r.End)
}

func (r *Range) Contains(item uint64) bool {
	return item >= r.Start && item <= r.End
}

type OrderRange struct {
	Ranges []Range
}

func NewDefaultOrderRange() *OrderRange {
	return &OrderRange{
		Ranges: []Range{{0, 0}},
	}
}

func NewOrderRange(start uint64, end uint64) *OrderRange {
	return &OrderRange{
		Ranges: []Range{{start, end}},
	}
}

func FromRange(r Range) *OrderRange {
	return &OrderRange{
		Ranges: []Range{r},
	}
}

func FromVec(vec []Range) *OrderRange {
	return &OrderRange{
		Ranges: vec,
	}
}

func isContinuousRange(lhs *Range, rhs *Range) bool {
	return lhs.End >= rhs.Start && lhs.Start <= rhs.End
}

func (o *OrderRange) RangesLen() int {
	return len(o.Ranges)
}

func (o *OrderRange) isFragmented() bool {
	return len(o.Ranges) != 1
}

func (o *OrderRange) IsEmpty() bool {
	if o.isFragmented() {
		return len(o.Ranges) == 0
	} else {
		return o.Ranges[0].IsEmpty()
	}
}

func (o *OrderRange) Contains(clock uint64) bool {
	for _, r := range o.Ranges {
		if r.Start <= clock && clock <= r.End {
			return true
		}
	}
	return false
}

func isRangeCovered(oldRange *Range, newVec *[]Range) bool {
	for _, newRange := range *newVec {
		if newRange.Contains(oldRange.Start) && newRange.Contains(oldRange.End) {
			return true
		}
	}
	return false
}

func checkRangeCovered(oldVect *[]Range, newVec *[]Range) bool {
	for _, oldRange := range *oldVect {
		if !isRangeCovered(&oldRange, newVec) {
			return false
		}
	}
	return true
}

// diff_range returns the difference between the old range and the new
// range. current range must be covered by the new range
func diffRange(oldVec *[]Range, newVec *[]Range) []Range {
	if !checkRangeCovered(oldVec, newVec) {
		return []Range{}
	}

	diffs := []Range{}
	oldIndex := 0
	for _, n := range *newVec {
		/**
		----------- ---------------- ----
		             ~~~~   ~~~~~~    ~~
					    overlaps
		*/
		overlapRanges := []Range{}
		for oldIndex < len(*oldVec) && (*oldVec)[oldIndex].Start <= n.End {
			overlapRanges = append(overlapRanges, (*oldVec)[oldIndex])
			oldIndex += 1
		}
		if len(overlapRanges) == 0 {
			diffs = append(diffs, n)
		} else {
			lastEnd := overlapRanges[0].Start
			if lastEnd > n.Start {
				diffs = append(diffs, *NewRange(n.Start, lastEnd))
			}
			for _, o := range overlapRanges {
				if o.Start > lastEnd {
					diffs = append(diffs, *NewRange(lastEnd, o.Start))
				}
				lastEnd = o.End
			}
			if n.End > lastEnd {
				diffs = append(diffs, *NewRange(lastEnd, n.End))
			}
		}
	}
	return diffs
}

func (o *OrderRange) DiffRange(newRange *OrderRange) []Range {
	return diffRange(&o.Ranges, &newRange.Ranges)
}

func (o *OrderRange) Squash() {
	if len(o.Ranges) > 1 {

	}
}

func max(a uint64, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}

func min(a uint64, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

func pushInner(list *[]Range, newRange *Range) {
	if len(*list) == 0 {
		*list = append(*list, *newRange)
	} else {
		merged := false
		for i := range *list {
			if isContinuousRange(&(*list)[i], newRange) {
				(*list)[i].Start = min((*list)[i].Start, newRange.Start)
				(*list)[i].End = max((*list)[i].End, newRange.End)
				merged = true
				break
			}
		}
		if !merged {
			// insert in sorted order
			inserted := false
			for i := range *list {
				if newRange.Start < (*list)[i].Start {
					result := make([]Range, 0, len(*list)+1)
					result = append(result, (*list)[:i]...)
					result = append(result, *newRange)
					result = append(result, (*list)[i:]...)
					*list = result
					inserted = true
					break
				}
			}
			if !inserted {
				*list = append(*list, *newRange)
			}
		}
	}
}

// Push new range to current one.
// Range will be merged if overlap exists or turned into fragment if it's
// not continuous.
func (o *OrderRange) Push(newRange *Range) {
	if o.isFragmented() {
		if o.IsEmpty() {
			o.Ranges = []Range{*newRange}
		} else {
			pushInner(&o.Ranges, newRange)
		}
	} else {
		r := &o.Ranges[0]
		if r.Start == r.End {
			o.Ranges[0] = *newRange
		} else if isContinuousRange(r, newRange) {
			r.End = max(r.End, newRange.End)
			r.Start = min(r.Start, newRange.Start)
		} else {
			if r.Start < newRange.Start {
				o.Ranges = []Range{*r, *newRange}
			} else {
				o.Ranges = []Range{*newRange, *r}
			}
		}
	}
}

func squashAround(list *[]Range, idx uint) {
	if idx > 0 {

	}
}

// func (r *IdRange) Invert() IdRange {
// 	if r.IsContinuous() {
// 		return NewContinuous(0, r.continuous.Start)
// 	} else {
// 		inv := []Range{}
// 		var start uint32 = 0
// 		for _, i := range *r.fragmented {
// 			if i.Start > start {
// 				inv = append(inv, NewRange(start, i.Start))
// 			}
// 			start = i.End
// 		}
// 		len := len(inv)
// 		switch len {
// 		case 0:
// 			return NewContinuous(0, 0)
// 		case 1:
// 			return NewContinuous(inv[0].Start, inv[0].End)
// 		default:
// 			return NewFragmented(&inv)
// 		}
// 	}
// }

// func (r *IdRange) pushContinuous(rg Range) {
// 	if r.continuous.End >= r.continuous.Start {
// 		if rg.Start > r.continuous.End {
// 			//     start     end
// 			//                   rg.start      rg.end
// 			r.fragmented = &[]Range{*r.continuous, rg}
// 			r.continuous = nil
// 		} else {
// 			if rg.Start < r.continuous.Start {
// 				r.continuous.Start = rg.Start
// 			}
// 			if rg.End > r.continuous.End {
// 				r.continuous.End = rg.End
// 			}
// 		}

// 	} else {
// 		//             start     end
// 		//     rg.end
// 		r.fragmented = &[]Range{rg, *r.continuous}
// 		r.continuous = nil
// 	}
// }

// func (r *IdRange) pushFragmented(rg Range) {
// 	if len((*r.fragmented)) == 0 {
// 		r.fragmented = nil
// 		r.continuous = &rg
// 	} else {
// 		lastIdx := len((*r.fragmented)) - 1
// 		last := &(*r.fragmented)[lastIdx]
// 		if !tryJoin(last, &rg) {
// 			ranges := append((*r.fragmented), rg)
// 			r.fragmented = &ranges
// 		}
// 	}
// }

// func (r *IdRange) push(rg Range) {
// 	if r.IsContinuous() {
// 		r.pushContinuous(rg)
// 	} else {
// 		r.pushFragmented(rg)
// 	}
// }

// func tryJoin(a *Range, b *Range) bool {
// 	if disjoint(a, b) {
// 		return false
// 	} else {
// 		if b.Start < a.Start {
// 			a.Start = b.Start
// 		}
// 		if b.End > a.End {
// 			a.End = b.End
// 		}
// 		return true
// 	}
// }

// func disjoint(a *Range, b *Range) bool {
// 	return a.Start > b.End || b.Start > a.End
// }
