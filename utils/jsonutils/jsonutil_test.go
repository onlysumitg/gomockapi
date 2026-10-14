package jsonutils

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestJsonToFlatMapTopLevelArray(t *testing.T) {
	flat, err := JsonToFlatMap(`[{"key":"value","key2":"value2","list":[1,2]}]`)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]any{
		"[0].key":     "value",
		"[0].key2":    "value2",
		"[0].list[0]": 1.0,
		"[0].list[1]": 2.0,
	}
	if len(flat) != len(want) {
		t.Fatalf("got %v", flat)
	}
	for k, v := range want {
		if flat[k].Value != v {
			t.Errorf("%s: got %v, want %v", k, flat[k].Value, v)
		}
	}
}

func TestJsonToMapPlaceholderMatchesFlatKeys(t *testing.T) {
	cases := []string{
		`[{"key":"value","key2":"value2","list":[1,[true,null]]}]`,
		`{"a":"x","b":{"c":[{"d":1},"e"]}}`,
	}
	for _, in := range cases {
		flat, err := JsonToFlatMap(in)
		if err != nil {
			t.Fatal(err)
		}
		placeholder, err := JsonToMapPlaceholder(in)
		if err != nil {
			t.Fatal(err)
		}
		asBytes, _ := json.Marshal(placeholder)
		s := string(asBytes)

		// every flat key must appear exactly once as a placeholder
		for k := range flat {
			if strings.Count(s, `"{{`+k+`}}"`) != 1 {
				t.Errorf("%s: placeholder for %q missing in %s", in, k, s)
			}
		}
		if strings.Count(s, "{{") != len(flat) {
			t.Errorf("%s: %d placeholders, %d flat keys: %s", in, strings.Count(s, "{{"), len(flat), s)
		}
	}
}
