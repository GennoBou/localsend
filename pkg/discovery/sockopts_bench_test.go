package discovery

import (
	"net"
	"testing"
)

func collectIPsUnallocated(interfaces []net.Interface, addrsMap map[string][]net.Addr) []net.IP {
	var ips []net.IP
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagMulticast == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs := addrsMap[iface.Name]
		for _, a := range addrs {
			ipNet, ok := a.(*net.IPNet)
			if ok && ipNet.IP.To4() != nil && isPrivateIP(ipNet.IP) {
				ips = append(ips, ipNet.IP)
			}
		}
	}
	return ips
}

func collectIPsPreallocated(interfaces []net.Interface, addrsMap map[string][]net.Addr) []net.IP {
	ips := make([]net.IP, 0, len(interfaces))
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagMulticast == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs := addrsMap[iface.Name]
		for _, a := range addrs {
			ipNet, ok := a.(*net.IPNet)
			if ok && ipNet.IP.To4() != nil && isPrivateIP(ipNet.IP) {
				ips = append(ips, ipNet.IP)
			}
		}
	}
	return ips
}

func setupBenchmarkInterfaces() ([]net.Interface, map[string][]net.Addr) {
	interfaces := []net.Interface{
		{Name: "eth0", Flags: net.FlagUp | net.FlagMulticast},
		{Name: "eth1", Flags: net.FlagUp | net.FlagMulticast},
		{Name: "eth2", Flags: net.FlagUp | net.FlagMulticast},
		{Name: "eth3", Flags: net.FlagUp | net.FlagMulticast},
		{Name: "lo", Flags: net.FlagUp | net.FlagLoopback},
		{Name: "down0", Flags: 0},
	}

	addrsMap := map[string][]net.Addr{
		"eth0":  {&net.IPNet{IP: net.ParseIP("192.168.1.10"), Mask: net.CIDRMask(24, 32)}},
		"eth1":  {&net.IPNet{IP: net.ParseIP("10.0.0.15"), Mask: net.CIDRMask(8, 32)}},
		"eth2":  {&net.IPNet{IP: net.ParseIP("172.16.0.20"), Mask: net.CIDRMask(12, 32)}},
		"eth3":  {&net.IPNet{IP: net.ParseIP("192.168.2.30"), Mask: net.CIDRMask(24, 32)}},
		"lo":    {&net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)}},
		"down0": {&net.IPNet{IP: net.ParseIP("192.168.3.40"), Mask: net.CIDRMask(24, 32)}},
	}

	return interfaces, addrsMap
}

func BenchmarkCollectIPsUnallocated(b *testing.B) {
	interfaces, addrsMap := setupBenchmarkInterfaces()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = collectIPsUnallocated(interfaces, addrsMap)
	}
}

func BenchmarkCollectIPsPreallocated(b *testing.B) {
	interfaces, addrsMap := setupBenchmarkInterfaces()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = collectIPsPreallocated(interfaces, addrsMap)
	}
}
