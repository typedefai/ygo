package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRange_IsEmpty(t *testing.T) {
	assert.Equal(t, false, NewRange(3, 5).IsEmpty())
	assert.Equal(t, true, NewRange(3, 3).IsEmpty())
	assert.Equal(t, true, NewRange(3, 2).IsEmpty())
}

func TestRange_Contains(t *testing.T) {
	assert.Equal(t, true, NewRange(3, 5).Contains(4))
	assert.Equal(t, false, NewRange(3, 5).Contains(2))
	assert.Equal(t, true, NewRange(3, 3).Contains(3))
}

func TestOrderRange_RangesLen(t *testing.T) {
	assert.Equal(t, 1, NewOrderRange(1, 2).RangesLen())
	assert.Equal(t, 2, FromVec([]Range{
		*NewRange(1, 2),
		*NewRange(2, 3)}).RangesLen())
}

func TestOrderRange_IsEmpty(t *testing.T) {
	assert.Equal(t, false, NewOrderRange(1, 2).IsEmpty())
	assert.Equal(t, true, NewOrderRange(1, 1).IsEmpty())
	assert.Equal(t, true, NewOrderRange(5, 1).IsEmpty())

	assert.Equal(t, false, FromVec([]Range{
		*NewRange(1, 2),
		*NewRange(2, 3)}).IsEmpty())
	assert.Equal(t, true, FromVec([]Range{}).IsEmpty())
}

func TestOrderRange_Contains(t *testing.T) {
	assert.Equal(t, false, NewOrderRange(1, 2).Contains(3))
	assert.Equal(t, true, NewOrderRange(1, 3).Contains(2))
	r := FromVec([]Range{
		*NewRange(1, 2),
		*NewRange(5, 10)})
	assert.Equal(t, false, r.Contains(3))
	assert.Equal(t, true, r.Contains(6))
}

func TestOrderRange_DiffRange(t *testing.T) {
	r1 := FromVec([]Range{*NewRange(0, 10), *NewRange(20, 30)})
	r2 := FromVec([]Range{*NewRange(0, 11), *NewRange(20, 30)})
	diff := r1.DiffRange(r2)
	assert.Equal(t, 1, len(diff))
	assert.Equal(t, uint64(10), diff[0].Start)
	assert.Equal(t, uint64(11), diff[0].End)
}

func TestCheckRangeCovered_GivenNotIncludes(t *testing.T) {
	assert.Equal(t, false, checkRangeCovered(
		&[]Range{*NewRange(0, 1)},
		&[]Range{*NewRange(2, 3)}))
	assert.Equal(t, false, checkRangeCovered(
		&[]Range{*NewRange(0, 1)},
		&[]Range{*NewRange(1, 3)}))

	assert.Equal(t, false, checkRangeCovered(
		&[]Range{*NewRange(1, 2), *NewRange(2, 3), *NewRange(3, 4)},
		&[]Range{*NewRange(0, 3)}))
}

func TestCheckRangeCovered_GivenIncludes(t *testing.T) {
	assert.Equal(t, true, checkRangeCovered(
		&[]Range{*NewRange(0, 1)},
		&[]Range{*NewRange(0, 3)}))
	assert.Equal(t, true, checkRangeCovered(
		&[]Range{*NewRange(1, 2)},
		&[]Range{*NewRange(0, 3)}))
	assert.Equal(t, true, checkRangeCovered(
		&[]Range{*NewRange(1, 2), *NewRange(2, 3)},
		&[]Range{*NewRange(0, 3)}))
	assert.Equal(t, true, checkRangeCovered(
		&[]Range{*NewRange(0, 1), *NewRange(2, 3)},
		&[]Range{*NewRange(0, 2), *NewRange(2, 4)}))
	assert.Equal(t, true, checkRangeCovered(
		&[]Range{*NewRange(1, 2), *NewRange(2, 3), *NewRange(3, 4)},
		&[]Range{*NewRange(0, 2), *NewRange(2, 4)}))
}

func TestDiffRange_GivenNotIncludes(t *testing.T) {
	r1 := diffRange(
		&[]Range{*NewRange(0, 1)},
		&[]Range{*NewRange(2, 3)})
	assert.Equal(t, 0, len(r1))
}

func TestDiffRange_GivenHasSingleRangeDiff(t *testing.T) {
	r1 := diffRange(
		&[]Range{*NewRange(0, 10)},
		&[]Range{*NewRange(0, 11)})
	assert.Equal(t, 1, len(r1))
	assert.Equal(t, uint64(10), r1[0].Start)
	assert.Equal(t, uint64(11), r1[0].End)
}

func TestDiffRange_GivenHasSingleRangeDiff_MultipleRanges(t *testing.T) {
	r1 := diffRange(
		&[]Range{*NewRange(0, 10), *NewRange(20, 30)},
		&[]Range{*NewRange(0, 11), *NewRange(20, 30)})
	assert.Equal(t, 1, len(r1))
	assert.Equal(t, uint64(10), r1[0].Start)
	assert.Equal(t, uint64(11), r1[0].End)
}

func TestDiffRange_GivenHasMultipleRangeDiff_OldFragmented(t *testing.T) {
	r1 := diffRange(
		&[]Range{*NewRange(0, 3), *NewRange(5, 7),
			*NewRange(8, 10), *NewRange(16, 18), *NewRange(21, 23)},
		&[]Range{*NewRange(0, 12), *NewRange(15, 23)})
	assert.Equal(t, 5, len(r1))
	assertRange(t, []uint64{3, 5, 7, 8, 10, 12, 15, 16, 18, 21}, &r1)
}

func TestDiffRange_GivenHasMultipleRangeDiff_NewFragmented(t *testing.T) {
	r1 := diffRange(
		&[]Range{*NewRange(1, 6), *NewRange(8, 12)},
		&[]Range{*NewRange(0, 12), *NewRange(15, 23), *NewRange(24, 28)})
	assert.Equal(t, 4, len(r1))
	assertRange(t, []uint64{0, 1, 6, 8, 15, 23, 24, 28}, &r1)
}

func assertRange(t *testing.T, expected []uint64, actual *[]Range) {
	i := 0
	for i < len(*actual) {
		start := expected[i*2]
		end := expected[i*2+1]
		r := (*actual)[i]
		assert.Equal(t, start, r.Start)
		assert.Equal(t, end, r.End)
		i += 1
	}
}
