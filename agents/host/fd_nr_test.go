package host

import "testing"

func TestParseFileNr(t *testing.T) {
	allocated, maximum, ok := parseFileNr("9760\t0\t1048576\n")
	if !ok || allocated != 9760 || maximum != 1048576 {
		t.Fatalf("three columns: allocated=%d maximum=%d ok=%v", allocated, maximum, ok)
	}
	allocated, maximum, ok = parseFileNr("0 0 9223372036854775807")
	if !ok || allocated != 0 || maximum != 9223372036854775807 {
		t.Fatalf("zero allocated: allocated=%d maximum=%d ok=%v", allocated, maximum, ok)
	}
	allocated, maximum, ok = parseFileNr("1200 65536")
	if !ok || allocated != 1200 || maximum != 65536 {
		t.Fatalf("two columns: allocated=%d maximum=%d ok=%v", allocated, maximum, ok)
	}
	if _, _, ok = parseFileNr("nope"); ok {
		t.Fatal("expected parse failure")
	}
}
