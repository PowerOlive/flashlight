package statreporter

import (
	"fmt"
	"sort"
)

const (
	increments = "increments"
	gauges     = "gauges"
)

const (
	set = iota
	add = iota
)

// DimGroup represents a group of dimensions for
type DimGroup struct {
	dims map[string]string
}

// UpdateBuilder is an intermediary data structure used in preparing an update
// for submission to statreporter.
type UpdateBuilder struct {
	dg       *DimGroup
	category string
	key      string
}

type update struct {
	dg       *DimGroup
	category string
	action   int
	key      string
	val      int64
}

// Dim constructs a DimGroup starting with a single dimension.
func Dim(key string, value string) *DimGroup {
	return &DimGroup{map[string]string{key: value}}
}

// And creates a new DimGroup that adds the given dim to the existing ones in
// the group.
func (dg *DimGroup) And(key string, value string) *DimGroup {
	newDims := map[string]string{key: value}
	for k, v := range dg.dims {
		newDims[k] = v
	}
	return &DimGroup{newDims}
}

// String returns a string representation of this DimGroup with keys in
// alphabetical order, making it suitable for using as a key representing this
// DimGroup.
func (dg *DimGroup) String() string {
	// Sort keys
	keys := make([]string, len(dg.dims))
	i := 0
	for key, _ := range dg.dims {
		keys[i] = key
		i = i + 1
	}
	sort.Strings(keys)

	// Build string
	s := ""
	for i, key := range keys {
		sep := ","
		if i == 0 {
			sep = ""
		}
		s = fmt.Sprintf("%s%s%s=%s", s, sep, key, dg.dims[key])
	}
	return s
}

func (dg *DimGroup) Increment(key string) *UpdateBuilder {
	return &UpdateBuilder{
		dg,
		increments,
		key,
	}
}

func (dg *DimGroup) Gauge(key string) *UpdateBuilder {
	return &UpdateBuilder{
		dg,
		gauges,
		key,
	}
}

func (b *UpdateBuilder) Add(val int64) {
	postUpdate(&update{
		b.dg,
		b.category,
		add,
		b.key,
		val,
	})
}

func (b *UpdateBuilder) Set(val int64) {
	postUpdate(&update{
		b.dg,
		b.category,
		set,
		b.key,
		val,
	})
}
