package version

import "testing"

func TestCompare(t *testing.T) {
	if Compare("v1.2.0", "v1.1.9") <= 0 {
		t.Fatal("expected v1.2.0 to be newer")
	}
	if Compare("v1.2.0", "1.2.0") != 0 {
		t.Fatal("expected equivalent versions")
	}
}
