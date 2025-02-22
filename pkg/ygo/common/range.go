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

func IsContinuousRange(lhs *Range, rhs *Range) bool {
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

// func (r *IdRange) IsContinuous() bool {
// 	return r.continuous != nil
// }

// func (r *IdRange) IsFragmented() bool {
// 	return r.fragmented != nil
// }

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

// func (r *IdRange) Contains(clock uint32) bool {
// 	if r.IsContinuous() {
// 		return r.continuous.Contains(clock)
// 	} else {
// 		for _, i := range *r.fragmented {
// 			if i.Contains(clock) {
// 				return true
// 			}
// 		}
// 		return false
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
