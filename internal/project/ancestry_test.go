package project

import "testing"

func TestCrissCrossMergeBasesRequireExplicitReconciliation(t *testing.T) {
	local := map[string][]string{"a": {}, "b": {"a"}, "c": {"a"}, "d": {"b", "c"}}
	remote := map[string][]string{"a": {}, "b": {"a"}, "c": {"a"}, "e": {"c", "b"}}
	if _, e := mergeBase(local, remote); e == nil {
		t.Fatal("ambiguous criss-cross bases silently selected")
	}
	remote["d"] = []string{"b", "c"}
	remote["e"] = []string{"d"}
	id, e := mergeBase(local, remote)
	if e != nil || id != "d" {
		t.Fatal("latest common revision not selected", id, e)
	}
}
