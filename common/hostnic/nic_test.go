package hostnic

import "testing"

func TestContainer(t *testing.T) {
	for _, name := range []string{
		"veth0a1b2c", "cali1234abcd", "lxc123", "docker0", "br-1a2b3c4d5e6f",
		"cni0", "flannel.1", "tunl0", "vxlan.calico", "cilium_host", "kube-ipvs0",
	} {
		if !Container(name) {
			t.Fatalf("container nic %s", name)
		}
	}
	for _, name := range []string{"eth0", "ens192", "eno1", "bond0", "br0", "em1", "lo"} {
		if Container(name) {
			t.Fatalf("host nic %s", name)
		}
	}
}
