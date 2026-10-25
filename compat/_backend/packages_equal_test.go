package integrations

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	"golang.org/x/exp/maps"
)

// Copied only while the backend still embeds builtin_integrations/: every
// package must serve exactly what the embedded copy serves.
func TestIntegrationPackagesEqualBuiltins(t *testing.T) {
	packages := renderedPackages(t)

	got, want := maps.Keys(packages), maps.Keys(builtInIntegrations)
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("package ids %v, backend builtins %v", got, want)
	}

	for id, builtin := range builtInIntegrations {
		if a, b := canonical(t, packages[id]), canonical(t, builtin); !reflect.DeepEqual(a, b) {
			t.Errorf("%s: package differs from the backend builtin", id)
			for k := range b {
				if !reflect.DeepEqual(a[k], b[k]) {
					t.Errorf("%s: field %s differs", id, k)
				}
			}
		}
	}
}

func canonical(t *testing.T, d IntegrationDetails) map[string]any {
	t.Helper()
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
