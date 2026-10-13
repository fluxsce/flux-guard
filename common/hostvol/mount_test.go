package hostvol

import "testing"

func TestContainer(t *testing.T) {
	if !Container("/s/ac1bc307-31c0-4164-bfa7-84e359f7ef53/volume-subpaths/fluxwms-config/wms/0") {
		t.Fatal("volume-subpaths should be a container volume")
	}
	if !Container("/var/lib/docker/overlay2/abc") {
		t.Fatal("docker overlay should be a container volume")
	}
	if !Container(`/var/lib/kubelet/pods/x`) {
		t.Fatal("kubelet pod should be a container volume")
	}
	for _, mount := range []string{"/", "/home", "/var", "/boot", "/data", `C:`} {
		if Container(mount) {
			t.Fatalf("host mount %s", mount)
		}
	}
}

func TestPartition(t *testing.T) {
	for _, name := range []string{"vda1", "sda2", "xvda1", "/dev/nvme0n1p1", "mmcblk0p1"} {
		if !Partition(name) {
			t.Fatalf("%s should be a partition", name)
		}
	}
	for _, name := range []string{"vda", "sda", "nvme0n1", "mmcblk0", "dm-0", "md0", "C:"} {
		if Partition(name) {
			t.Fatalf("%s should be a whole disk", name)
		}
	}
}

func TestBlockName(t *testing.T) {
	if got := BlockName("/dev/sda1", "/"); got != "sda1" {
		t.Fatalf("sda1=%s", got)
	}
	if got := BlockName("/dev/mapper/ubuntu--vg-root", "/"); got != "ubuntu--vg-root" {
		t.Fatalf("mapper=%s", got)
	}
	if got := BlockName("C:", "C:"); got != "C:" {
		t.Fatalf("windows=%s", got)
	}
	if got := BlockName("", "/data"); got != "/data" {
		t.Fatalf("fallback=%s", got)
	}
}
