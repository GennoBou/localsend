package discovery

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/GennoBou/localsend/pkg/protocol"
	"github.com/pion/mdns/v2"
	"golang.org/x/net/ipv4"
)

// listenMulticastUDP binds to the mDNS multicast address with SO_REUSEADDR enabled
// to allow port sharing on the local system.
func listenMulticastUDP(network string, address string) (*net.UDPConn, error) {
	addr, err := net.ResolveUDPAddr(network, address)
	if err != nil {
		return nil, err
	}

	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				// Enable port reuse so multiple applications can bind to 5353
				_ = syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
			})
		},
	}

	packetConn, err := lc.ListenPacket(context.Background(), network, addr.String())
	if err != nil {
		return nil, err
	}
	return packetConn.(*net.UDPConn), nil
}

// createMulticastConnV4 initializes an ipv4.PacketConn for mDNS.
func createMulticastConnV4() (*ipv4.PacketConn, *net.UDPConn, error) {
	udpConn, err := listenMulticastUDP("udp4", mdns.DefaultAddressIPv4)
	if err != nil {
		return nil, nil, err
	}
	return ipv4.NewPacketConn(udpConn), udpConn, nil
}

// StartMDNSListener starts an mDNS listener to browse for LocalSend devices.
// It runs asynchronously and context cancellation will clean up resources.
func StartMDNSListener(ctx context.Context, onDiscover func(protocol.Device)) error {
	pcV4, rawConn, err := createMulticastConnV4()
	if err != nil {
		return err
	}

	// Create an mDNS server connection to browse for services.
	// Omitting WithRecordTypes allows processing all DNS-SD records (PTR, SRV, TXT, A, AAAA).
	server, err := mdns.NewServer(pcV4, nil)
	if err != nil {
		rawConn.Close()
		return fmt.Errorf("failed to create mDNS browsing server: %w", err)
	}

	// Register the discovered service handler
	server.OnServiceDiscovered(func(evt mdns.ServiceEvent) {
		dev := parseDeviceFromServiceEvent(evt)
		if dev.Fingerprint != "" {
			onDiscover(dev)
		}
	})

	// Start browsing in a background goroutine
	go func() {
		// Browse blocks until context is canceled
		_ = server.Browse(ctx, "_localsend._tcp")
	}()

	// Cleanup goroutine
	go func() {
		<-ctx.Done()
		_ = server.Close()
		_ = rawConn.Close()
	}()

	return nil
}

// RegisterMDNSService registers the local device as an mDNS service.
// Returns a Closer that unregisters the service when closed.
func RegisterMDNSService(ctx context.Context, myDevice protocol.Device) (io.Closer, error) {
	pcV4, rawConn, err := createMulticastConnV4()
	if err != nil {
		return nil, err
	}

	// Construct unique service instance name: "Alias (first 8 chars of Fingerprint)"
	instanceName := myDevice.Alias
	if len(myDevice.Fingerprint) >= 8 {
		instanceName = fmt.Sprintf("%s (%s)", myDevice.Alias, myDevice.Fingerprint[:8])
	}
	if len(instanceName) > 63 {
		instanceName = myDevice.Alias
		if len(instanceName) > 63 {
			instanceName = instanceName[:63]
		}
	}

	// Compile TXT record entries
	txt := []mdns.TXTEntry{
		mdns.NewTXTString("alias", myDevice.Alias),
		mdns.NewTXTString("version", protocol.ProtocolVersion),
		mdns.NewTXTString("deviceModel", myDevice.DeviceModel),
		mdns.NewTXTString("deviceType", string(myDevice.DeviceType)),
		mdns.NewTXTString("fingerprint", myDevice.Fingerprint),
		mdns.NewTXTString("protocol", myDevice.Protocol),
		mdns.NewTXTString("download", strconv.FormatBool(myDevice.Download)),
	}

	// Define host name (e.g., localsend-a1b2c3d4.local.)
	var hostName string
	if len(myDevice.Fingerprint) >= 8 {
		hostName = fmt.Sprintf("localsend-%s.local.", myDevice.Fingerprint[:8])
	} else {
		hostName = "localsend-device.local."
	}

	svc := mdns.ServiceInstance{
		Instance: instanceName,
		Service:  "_localsend._tcp",
		Domain:   "local",
		Host:     hostName,
		Port:     uint16(myDevice.Port),
		Text:     txt,
	}

	// Establish server and register service
	server, err := mdns.NewServer(pcV4, nil,
		mdns.WithLocalNames(hostName),
		mdns.WithService(svc),
	)
	if err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("failed to create mDNS advertising server: %w", err)
	}

	closer := &mdnsCloser{
		server:  server,
		rawConn: rawConn,
	}

	// Unregister when context is canceled
	go func() {
		<-ctx.Done()
		_ = closer.Close()
	}()

	return closer, nil
}

type mdnsCloser struct {
	server  *mdns.Conn
	rawConn *net.UDPConn
	once    sync.Once
}

func (c *mdnsCloser) Close() error {
	var err error
	c.once.Do(func() {
		_ = c.server.Close()
		err = c.rawConn.Close()
	})
	return err
}

// parseDeviceFromServiceEvent parses an mDNS ServiceEvent into a protocol.Device struct.
func parseDeviceFromServiceEvent(evt mdns.ServiceEvent) protocol.Device {
	dev := protocol.Device{
		Port:     int(evt.Instance.Port),
		Protocol: "https", // Default to HTTPS
	}

	// Parse TXT records
	for _, entry := range evt.Instance.Text {
		val := string(entry.Value)
		switch strings.ToLower(entry.Key) {
		case "alias":
			dev.Alias = val
		case "version":
			dev.Version = val
		case "devicemodel":
			dev.DeviceModel = val
		case "devicetype":
			dev.DeviceType = protocol.DeviceType(val)
		case "fingerprint":
			dev.Fingerprint = val
		case "protocol":
			dev.Protocol = val
		case "download":
			dev.Download = (val == "true" || val == "1")
		}
	}

	// Set IP address
	if evt.Addr != (netip.Addr{}) {
		dev.IP = evt.Addr.String()
	}

	// Basic validation: must have fingerprint and alias
	if dev.Fingerprint == "" || dev.Alias == "" {
		return protocol.Device{}
	}

	return dev
}

// DiscoveryMode represents the network discovery mode.
type DiscoveryMode string

const (
	DiscoveryModeHybrid    DiscoveryMode = "hybrid"
	DiscoveryModeMulticast DiscoveryMode = "multicast"
	DiscoveryModeMDNS      DiscoveryMode = "mdns"
)

// StartListeners starts network discovery listeners based on the selected mode.
func StartListeners(ctx context.Context, myDevice protocol.Device, mode DiscoveryMode, onDiscover func(protocol.Device), onAnnounce func(protocol.Device)) error {
	var errs []string

	if mode == DiscoveryModeHybrid || mode == DiscoveryModeMulticast {
		err := StartMulticastListener(ctx, myDevice, onDiscover, onAnnounce)
		if err != nil {
			errs = append(errs, fmt.Sprintf("multicast: %v", err))
		}
	}

	if mode == DiscoveryModeHybrid || mode == DiscoveryModeMDNS {
		err := StartMDNSListener(ctx, onDiscover)
		if err != nil {
			errs = append(errs, fmt.Sprintf("mdns: %v", err))
		}
	}

	if len(errs) > 0 {
		// In hybrid mode, succeed if at least one protocol is active.
		if mode == DiscoveryModeHybrid && len(errs) < 2 {
			return nil
		}
		return fmt.Errorf("failed to start listeners: %s", strings.Join(errs, "; "))
	}

	return nil
}

// AdvertisingCloser wraps the active advertisers (multicast and/or mDNS).
type AdvertisingCloser struct {
	mdnsCloser io.Closer
	once       sync.Once
}

// Close stops advertising.
func (a *AdvertisingCloser) Close() error {
	var err error
	a.once.Do(func() {
		if a.mdnsCloser != nil {
			err = a.mdnsCloser.Close()
		}
	})
	return err
}

// StartAdvertising advertises the device on the network based on the selected mode.
// For multicast, it sends a multicast packet.
// For mDNS, it registers the service.
func StartAdvertising(ctx context.Context, myDevice protocol.Device, mode DiscoveryMode, announce bool) (io.Closer, error) {
	adv := &AdvertisingCloser{}

	if mode == DiscoveryModeHybrid || mode == DiscoveryModeMulticast {
		// Send initial UDP multicast announce
		_ = SendAnnounce(myDevice, announce)
	}

	if mode == DiscoveryModeHybrid || mode == DiscoveryModeMDNS {
		// Register mDNS service
		closer, err := RegisterMDNSService(ctx, myDevice)
		if err != nil {
			return nil, fmt.Errorf("failed to register mDNS service: %w", err)
		}
		adv.mdnsCloser = closer
	}

	return adv, nil
}

