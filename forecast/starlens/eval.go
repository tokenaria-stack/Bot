package starlens

import "fmt"

// Result is one evaluation of a parent through a lens.
// Pictures omit each field's own clauses.
// Pass applies every clause.
type Result struct {
	Pass       []StarRef
	Population int
	LensPass   int
	base       map[string][]int
	rows       []Row
	parent     []int
}

// NumberPicture is the continuous values on the self-excluded base.
// Missing is a count. Those rows are not zeros in Values.
type NumberPicture struct {
	Base    int
	Missing int
	Values  []float64
}

// EventPicture is one R level, or the stop, on the self-excluded base.
// Rate is Reached or Touched over Base. It is a count ratio.
// Base zero leaves RateOK false.
type EventPicture struct {
	Base       int
	Reached    int
	NotReached int
	Undefined  int
	Rate       float64
	RateOK     bool
}

// StatusPicture counts stored statuses on the base that excludes status clauses.
type StatusPicture struct {
	Base   int
	Counts map[string]int
}

// Evaluate applies clauses to parent. An empty clause list passes the parent.
// The viewport is not an argument.
func Evaluate(parent Population, table []Row, clauses []Clause) (Result, error) {
	for i := range clauses {
		if err := clauses[i].validate(); err != nil {
			return Result{}, err
		}
	}
	parentIdx := make([]int, 0, len(parent.Members))
	for _, member := range parent.Members {
		row, err := rowAt(table, member)
		if err != nil {
			return Result{}, err
		}
		parentIdx = append(parentIdx, row.Index)
	}
	keys := map[string]struct{}{}
	for _, clause := range clauses {
		keys[clause.key()] = struct{}{}
	}
	base := map[string][]int{}
	for key := range keys {
		idx, err := filter(table, parentIdx, clauses, key)
		if err != nil {
			return Result{}, err
		}
		base[key] = idx
	}
	passIdx, err := filter(table, parentIdx, clauses, "")
	if err != nil {
		return Result{}, err
	}
	pass := make([]StarRef, len(passIdx))
	for i, idx := range passIdx {
		pass[i] = table[idx].Ref()
	}
	return Result{
		Pass: pass, Population: len(parent.Members), LensPass: len(pass),
		base: base, rows: table, parent: parentIdx,
	}, nil
}

func rowAt(table []Row, member StarRef) (Row, error) {
	if member.Index < 0 || member.Index >= len(table) {
		return Row{}, fmt.Errorf("starlens: member index %d", member.Index)
	}
	row := table[member.Index]
	if row.Index != member.Index || row.DecisionAt != member.DecisionAt || row.Side != member.Side {
		return Row{}, fmt.Errorf("starlens: member %d does not match the table", member.Index)
	}
	return row, nil
}

// filter keeps parent rows that satisfy every clause except those with skip key.
// An empty skip applies every clause.
func filter(table []Row, parent []int, clauses []Clause, skip string) ([]int, error) {
	out := make([]int, 0, len(parent))
	for _, idx := range parent {
		ok := true
		for _, clause := range clauses {
			if skip != "" && clause.key() == skip {
				continue
			}
			holds, err := clause.holds(table[idx])
			if err != nil {
				return nil, err
			}
			if !holds {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, idx)
		}
	}
	return out, nil
}

// NumberPicture returns field X with X's clauses removed.
func (r Result) NumberPicture(field string) (NumberPicture, error) {
	if _, ok := knownField(field); !ok {
		return NumberPicture{}, fmt.Errorf("starlens: picture field %q", field)
	}
	idx := r.base["n:"+field]
	if idx == nil {
		idx = r.parent
	}
	pic := NumberPicture{Base: len(idx)}
	for _, i := range idx {
		n, _ := r.rows[i].Number(field)
		if !n.OK {
			pic.Missing++
			continue
		}
		pic.Values = append(pic.Values, n.Value)
	}
	return pic, nil
}

// RPicture returns one R level with that level's clauses removed.
func (r Result) RPicture(level int) (EventPicture, error) {
	if level < 1 || level > 3 {
		return EventPicture{}, fmt.Errorf("starlens: R level %d", level)
	}
	idx := r.base[fmt.Sprintf("r:%d", level)]
	if idx == nil {
		idx = r.parent
	}
	var pic EventPicture
	pic.Base = len(idx)
	for _, i := range idx {
		state, err := r.rows[i].level(level)
		if err != nil {
			return EventPicture{}, err
		}
		switch state {
		case Reached:
			pic.Reached++
		case NotReachedValidR:
			pic.NotReached++
		default:
			pic.Undefined++
		}
	}
	if pic.Base > 0 {
		pic.Rate = float64(pic.Reached) / float64(pic.Base)
		pic.RateOK = true
	}
	return pic, nil
}

// StopPicture returns the stop event with stop clauses removed.
func (r Result) StopPicture() EventPicture {
	idx := r.base["stop"]
	if idx == nil {
		idx = r.parent
	}
	var pic EventPicture
	pic.Base = len(idx)
	for _, i := range idx {
		switch r.rows[i].Stop {
		case StopTouched:
			pic.Reached++
		case StopNotTouched:
			pic.NotReached++
		default:
			pic.Undefined++
		}
	}
	if pic.Base > 0 {
		pic.Rate = float64(pic.Reached) / float64(pic.Base)
		pic.RateOK = true
	}
	return pic
}

// StatusPicture returns status counts with status clauses removed.
func (r Result) StatusPicture() StatusPicture {
	idx := r.base["status"]
	if idx == nil {
		idx = r.parent
	}
	pic := StatusPicture{Base: len(idx), Counts: map[string]int{}}
	for _, i := range idx {
		pic.Counts[r.rows[i].Status]++
	}
	return pic
}

// ViewCount counts pass members whose decision time is in [start, end).
// The range is presentation. It does not change Pass.
func ViewCount(pass []StarRef, start, end int64) int {
	n := 0
	for _, member := range pass {
		if member.DecisionAt >= start && member.DecisionAt < end {
			n++
		}
	}
	return n
}

// CountBins counts values into the bins a caller supplies.
// edges are inclusive right edges, in increasing order.
// The last bin is every value above the last edge.
// The edges are not stored on a population.
func CountBins(values []float64, edges []float64) ([]int, error) {
	if len(edges) == 0 {
		return nil, fmt.Errorf("starlens: histogram edges are empty")
	}
	for i := 1; i < len(edges); i++ {
		if !(edges[i] > edges[i-1]) {
			return nil, fmt.Errorf("starlens: histogram edges are not increasing")
		}
	}
	bins := make([]int, len(edges)+1)
	for _, v := range values {
		placed := false
		for i, edge := range edges {
			if v <= edge {
				bins[i]++
				placed = true
				break
			}
		}
		if !placed {
			bins[len(edges)]++
		}
	}
	return bins, nil
}
