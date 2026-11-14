package model

import (
	"reflect"
	"sort"
	"testing"
)

func TestProxySnapshotGroups(t *testing.T) {
	for _, tc := range []struct {
		name    string
		proxies ProxySnapshot
		groups  map[string]Group
		prefix  []string
		extras  []string
	}{
		{"empty", nil, map[string]Group{}, nil, nil},
		{"empty group is excluded", ProxySnapshot{"empty": {Type: "Selector"}, "leaf": {Type: "ss"}}, map[string]Group{}, nil, nil},
		{
			"without GLOBAL",
			ProxySnapshot{"a": {Name: "wrong", Type: "Selector", Now: "leaf", All: []string{"leaf"}, History: []Delay{{Delay: 42}}}},
			map[string]Group{"a": {Name: "a", Type: "Selector", Now: "leaf", All: []string{"leaf"}, History: []Delay{{Delay: 42}}}},
			nil, []string{"a"},
		},
		{
			"GLOBAL order ignores leaves and missing entries",
			ProxySnapshot{"GLOBAL": {All: []string{"b", "leaf", "missing", "a"}}, "a": {All: []string{"leaf"}}, "b": {All: []string{"missing"}}},
			map[string]Group{"GLOBAL": {Name: "GLOBAL", All: []string{"b", "leaf", "missing", "a"}}, "a": {Name: "a", All: []string{"leaf"}}, "b": {Name: "b", All: []string{"missing"}}},
			[]string{"b", "a"}, []string{"GLOBAL"},
		},
		{
			"incomplete order supplements every remaining group",
			ProxySnapshot{"GLOBAL": {All: []string{"b"}}, "a": {All: []string{"leaf"}}, "b": {All: []string{"leaf"}}},
			map[string]Group{"GLOBAL": {Name: "GLOBAL", All: []string{"b"}}, "a": {Name: "a", All: []string{"leaf"}}, "b": {Name: "b", All: []string{"leaf"}}},
			[]string{"b"}, []string{"GLOBAL", "a"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			groups, order := tc.proxies.Groups()
			if !reflect.DeepEqual(groups, tc.groups) || len(order) != len(tc.prefix)+len(tc.extras) {
				t.Fatalf("groups=%+v order=%v", groups, order)
			}
			for i, name := range tc.prefix {
				if order[i] != name {
					t.Fatalf("order=%v, want prefix=%v", order, tc.prefix)
				}
			}
			extras := append([]string(nil), order[len(tc.prefix):]...)
			sort.Strings(extras)
			if !reflect.DeepEqual(extras, tc.extras) {
				t.Fatalf("unordered extras=%v, want %v", extras, tc.extras)
			}
		})
	}
}
