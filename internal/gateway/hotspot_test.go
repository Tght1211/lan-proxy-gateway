package gateway

import (
	"errors"
	"github.com/tght/lan-proxy-gateway/internal/firewall"
	"github.com/tght/lan-proxy-gateway/internal/hotspot"
	"testing"
)

func sharingFixture() hotspot.Status {
	return hotspot.Status{Supported: true, Available: true, Interface: "bridge100", Members: []string{"ap1"}, IP: "192.168.2.1", CIDR: "192.168.2.0/24", Uplink: "en0"}
}

func TestHotspotLifecycleNeverChangesHostGlobals(t *testing.T) {
	fp := &fakePlatform{forwardOn: true}
	fw := &fakeFirewall{}
	g := newGateway(t, fp, fw)
	s := sharingFixture()
	g.detectHotspot = func() hotspot.Status { return s }
	c := firewall.Config{Hotspot: true, RedirPort: 17892, DNSPort: 1053, TCPRedirect: true}
	if err := g.Enable(c); err != nil {
		t.Fatal(err)
	}
	if !g.HotspotStatus().Applied || fw.applied.Iface != "bridge100" || fw.applied.GatewayIP != "192.168.2.1" || len(fw.applied.LANCIDRs) != 1 || fw.applied.LANCIDRs[0] != "192.168.2.0/24" {
		t.Fatalf("wrong scope: %+v", fw.applied)
	}
	if !fw.applied.DNSHijack || !fw.applied.IPv6Block {
		t.Fatal("hotspot DNS and IPv6 must be scoped")
	}
	s.Available = false
	if err := g.Enable(c); err != nil {
		t.Fatal(err)
	}
	if g.HotspotStatus().Applied || !contains(fw.calls, "Remove") {
		t.Fatal("lost hotspot must remove capture")
	}
	s = sharingFixture()
	s.Interface = "bridge101"
	s.IP = "192.168.3.1"
	s.CIDR = "192.168.3.0/24"
	if err := g.Enable(c); err != nil {
		t.Fatal(err)
	}
	if fw.applied.Iface != "bridge101" || fw.applied.LANCIDRs[0] != "192.168.3.0/24" {
		t.Fatal("did not follow changed sharing network")
	}
	if err := g.Disable(); err != nil {
		t.Fatal(err)
	}
	if len(fp.calls) > 0 {
		t.Fatalf("changed host globals: %v", fp.calls)
	}
	if err := g.Enable(c); err == nil {
		t.Fatal("late config watcher must not reinstall rules after stop")
	}
}

func TestMissingHotspotDoesNotCaptureDefaultLAN(t *testing.T) {
	fp := &fakePlatform{}
	fw := &fakeFirewall{}
	g := newGateway(t, fp, fw)
	g.detectHotspot = func() hotspot.Status { return hotspot.Status{Supported: true} }
	if err := g.Enable(firewall.Config{Hotspot: true}); err != nil {
		t.Fatal(err)
	}
	if len(fp.calls) != 0 || len(fw.calls) != 0 {
		t.Fatal("waiting must leave network untouched")
	}
}

type failingFirewall struct{ fakeFirewall }

func (f *failingFirewall) Apply(c firewall.Config) (firewall.Report, error) {
	return firewall.Report{}, errors.New("load failed")
}
func TestHotspotApplyFailureCleansOnlyOwnedRules(t *testing.T) {
	fp := &fakePlatform{forwardOn: true}
	fw := &failingFirewall{}
	g := newForTest(fp, fw)
	g.detectHotspot = sharingFixture
	if err := g.Enable(firewall.Config{Hotspot: true}); err == nil {
		t.Fatal("expected error")
	}
	if g.HotspotStatus().Applied || !contains(fw.calls, "Remove") || len(fp.calls) != 0 {
		t.Fatal("failure did not clean up safely")
	}
}
