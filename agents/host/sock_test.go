package host

import "testing"

func TestCountTCPTable(t *testing.T) {
	text := "  sl  local_address rem_address   st\n" +
		"   0: 0100007F:0050 00000000:0000 0A 00000000:00000000 00:00000000 00000000 0 0 1 1 0000000000000000 100 0 0 10 0\n" +
		"   1: 0100007F:1F90 0200007F:0CEA 01 00000000:00000000 00:00000000 00000000 0 0 2 1 0000000000000000 20 4 30 10 -1\n" +
		"   2: 0100007F:1F91 0200007F:0CEB 06 00000000:00000000 00:00000000 00000000 0 0 3 1 0000000000000000 20 4 30 10 -1\n" +
		"   3: 0100007F:1F92 0200007F:0CEC 08 00000000:00000000 00:00000000 00000000 0 0 4 1 0000000000000000 20 4 30 10 -1\n"
	var snap socketSnap
	countTCPTable(text, &snap)
	if snap.listen != 1 || snap.established != 1 || snap.timeWait != 1 || snap.closeWait != 1 || !snap.ok {
		t.Fatalf("snap=%+v", snap)
	}
	parseSockstat("TCP: inuse 4 orphan 0 tw 1 alloc 8 mem 1\nUDP: inuse 2 mem 1\n", &snap)
	if snap.tcpInUse != 4 || snap.udpInUse != 2 || !snap.udpOK {
		t.Fatalf("sockstat=%+v", snap)
	}
	n, ok := parseTCPRetrans("Tcp: RtoAlgorithm CurrEstab RetransSegs\nTcp: 1 3 9\n")
	if !ok || n != 9 {
		t.Fatalf("retrans=%d ok=%v", n, ok)
	}
}
