package setting

import "testing"

func TestTenantByIngestToken(t *testing.T) {
	PutIngest("default", DevIngestToken())
	tenantID, ok := TenantByIngestToken(DevIngestPlain)
	if !ok || tenantID != "default" {
		t.Fatalf("dev token tenant=%s ok=%v", tenantID, ok)
	}
	if _, ok := TenantByIngestToken("agt_other"); ok {
		t.Fatal("foreign token accepted")
	}
	stopped := DevIngestToken()
	stopped.StatusFlag = "N"
	PutIngest("default", stopped)
	if _, ok := TenantByIngestToken(DevIngestPlain); ok {
		t.Fatal("disabled token accepted")
	}
	if got := RevealIngest(stopped); got != DevIngestPlain {
		t.Fatalf("reveal disabled %q", got)
	}
	stopped.TokenEnc = ""
	if got := RevealIngest(stopped); got != "" {
		t.Fatal("missing cipher revealed")
	}
}
