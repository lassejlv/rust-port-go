package contract

import semver "github.com/lassejlv/rust-port-go/port/dependencies/semver/v1_0_28/src"

type CloneRequest struct {
	Kind                string `json:"kind"`
	Length              int    `json:"length"`
	SourceCapacity      int    `json:"source_capacity"`
	DestinationCapacity int    `json:"destination_capacity"`
	Text                string `json:"text"`
}

func CloneCorpus() []CloneRequest {
	var result []CloneRequest
	for _, length := range []int{0, 1, 2, 4, 8, 16, 18, 24, 33, 65, 100} {
		for _, extra := range []int{0, 7, 100} {
			for _, capacity := range []int{0, 100, 200} {
				result = append(result, CloneRequest{Kind: "req", Length: length, SourceCapacity: length + extra, DestinationCapacity: capacity})
			}
		}
	}
	for _, pair := range [][2]string{{"version", "1.2.3-alpha.456+metadata"}, {"version", "0.0.0"}, {"comparator", ">=1.2.3-alpha"}, {"comparator", "<2"}, {"pre", ""}, {"pre", "alpha.123"}, {"build", ""}, {"build", "build.0123"}, {"op", ""}} {
		result = append(result, CloneRequest{Kind: pair[0], Text: pair[1]})
	}
	return result
}

func ObserveClone(q CloneRequest) map[string]any {
	if q.Kind == "req" {
		values := make([]semver.Comparator, q.Length, q.SourceCapacity)
		for i := range values {
			values[i] = semver.Comparator{Op: semver.Exact, Major: uint64(i + 1)}
		}
		source := semver.VersionReq{Comparators: values}
		clone := source.Clone()
		destination := semver.VersionReq{Comparators: make([]semver.Comparator, 0, q.DestinationCapacity)}
		destination.CloneFrom(source)
		out := map[string]any{"clone_eq": source.Equal(clone), "clone_from_eq": source.Equal(destination), "clone_capacity": cap(clone.Comparators), "from_capacity": cap(destination.Comparators), "source_capacity": cap(source.Comparators), "length": q.Length}
		independent := true
		if q.Length > 0 {
			clone.Comparators[0].Major = 900
			destination.Comparators[0].Major = 800
			independent = source.Comparators[0].Major == 1 && clone.Comparators[0].Major == 900 && destination.Comparators[0].Major == 800
		}
		out["independent"] = independent
		return out
	}
	var cloned, replaced bool
	switch q.Kind {
	case "version":
		source, e := semver.ParseVersion(q.Text)
		if e != nil {
			panic(e)
		}
		clone := source.Clone()
		destination := semver.NewVersion(8, 9, 10)
		destination.CloneFrom(source)
		cloned = clone.Equal(source)
		replaced = destination.Equal(source)
	case "comparator":
		source, e := semver.ParseComparator(q.Text)
		if e != nil {
			panic(e)
		}
		clone := source.Clone()
		destination := semver.Comparator{Op: semver.Less, Major: 900}
		destination.CloneFrom(source)
		cloned = clone.Equal(source)
		replaced = destination.Equal(source)
	case "pre":
		source, e := semver.NewPrerelease(q.Text)
		if e != nil {
			panic(e)
		}
		clone := source.Clone()
		destination, _ := semver.NewPrerelease("old")
		destination.CloneFrom(source)
		cloned = clone.Equal(source)
		replaced = destination.Equal(source)
	case "build":
		source, e := semver.NewBuildMetadata(q.Text)
		if e != nil {
			panic(e)
		}
		clone := source.Clone()
		destination, _ := semver.NewBuildMetadata("old")
		destination.CloneFrom(source)
		cloned = clone.Equal(source)
		replaced = destination.Equal(source)
	case "op":
		cloned, replaced = true, true
		for source := semver.Exact; source <= semver.Wildcard; source++ {
			clone := source.Clone()
			destination := semver.Less
			destination.CloneFrom(source)
			cloned = cloned && clone.Equal(source)
			replaced = replaced && destination.Equal(source)
		}
	default:
		panic("unknown clone case")
	}
	return map[string]any{"clone_eq": cloned, "clone_from_eq": replaced}
}
