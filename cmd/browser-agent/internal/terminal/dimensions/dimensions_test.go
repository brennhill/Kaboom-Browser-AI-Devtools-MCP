// dimensions_test.go — Verifies safe terminal dimension conversion.

package dimensions

import "testing"

func TestResolveDefaultsAndBoundsDimensions(t *testing.T) {
	t.Parallel()
	for _, value := range [][2]int{{-1, 24}, {80, -1}, {65536, 24}, {80, 65536}} {
		if _, _, ok := Resolve(value[0], value[1]); ok {
			t.Fatalf("Resolve(%d, %d) accepted invalid dimensions", value[0], value[1])
		}
	}
	if cols, rows, ok := Resolve(0, 0); !ok || cols != 0 || rows != 0 {
		t.Fatalf("Resolve(0, 0) = (%d, %d, %v)", cols, rows, ok)
	}
	if cols, rows, ok := Resolve(65535, 65535); !ok || cols != 65535 || rows != 65535 {
		t.Fatalf("Resolve(max) = (%d, %d, %v)", cols, rows, ok)
	}
}
