package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"
	"encoding/binary"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/d2g/dhcp4"
	"github.com/d2g/dhcp4client"
	"github.com/vishvananda/netlink"
)

const resendDelay0 = 4 * time.Second
const resendDelayMax = 32 * time.Second
const resendCount = 3

const (
	leaseStateBound = iota
	leaseStateRenewing
	leaseStateRebinding
)
func parseRouter(opts dhcp4.Options) net.IP {
	if opts, ok := opts[dhcp4.OptionRouter]; ok {
		if len(opts) == 4 {
			return net.IP(opts)
		}
	}
	return nil
}

func classfulSubnet(sn net.IP) net.IPNet {
	return net.IPNet{
		IP:   sn,
		Mask: sn.DefaultMask(),
	}
}

func parseRoutes(opts dhcp4.Options) []*types.Route {
	// StaticRoutes format: pairs of:
	// Dest = 4 bytes; Classful IP subnet
	// Router = 4 bytes; IP address of router

	routes := []*types.Route{}
	if opt, ok := opts[dhcp4.OptionStaticRoute]; ok {
		for len(opt) >= 8 {
			sn := opt[0:4]
			r := opt[4:8]
			rt := &types.Route{
				Dst: classfulSubnet(sn),
				GW:  r,
			}
			routes = append(routes, rt)
			opt = opt[8:]
		}
	}

	return routes
}

func parseCIDRRoutes(opts dhcp4.Options) []*types.Route {
	// See RFC4332 for format (http://tools.ietf.org/html/rfc3442)

	routes := []*types.Route{}
	if opt, ok := opts[dhcp4.OptionClasslessRouteFormat]; ok {
		for len(opt) >= 5 {
			width := int(opt[0])
			if width > 32 {
				// error: can't have more than /32
				return nil
			}
			// network bits are compacted to avoid zeros
			octets := 0
			if width > 0 {
				octets = (width-1)/8 + 1
			}

			if len(opt) < 1+octets+4 {
				// error: too short
				return nil
			}

			sn := make([]byte, 4)
			copy(sn, opt[1:octets+1])

			gw := net.IP(opt[octets+1 : octets+5])

			rt := &types.Route{
				Dst: net.IPNet{
					IP:   net.IP(sn),
					Mask: net.CIDRMask(width, 32),
				},
				GW: gw,
			}
			routes = append(routes, rt)

			opt = opt[octets+5:]
		}
	}
	return routes
}

func parseSubnetMask(opts dhcp4.Options) net.IPMask {
	mask, ok := opts[dhcp4.OptionSubnetMask]
	if !ok {
		return nil
	}

	return net.IPMask(mask)
}

func parseDuration(opts dhcp4.Options, code dhcp4.OptionCode, optName string) (time.Duration, error) {
	val, ok := opts[code]
	if !ok {
		return 0, fmt.Errorf("option %v not found", optName)
	}
	if len(val) != 4 {
		return 0, fmt.Errorf("option %v is not 4 bytes", optName)
	}

	secs := binary.BigEndian.Uint32(val)
	return time.Duration(secs) * time.Second, nil
}

func parseLeaseTime(opts dhcp4.Options) (time.Duration, error) {
	return parseDuration(opts, dhcp4.OptionIPAddressLeaseTime, "LeaseTime")
}

func parseRenewalTime(opts dhcp4.Options) (time.Duration, error) {
	return parseDuration(opts, dhcp4.OptionRenewalTimeValue, "RenewalTime")
}

func parseRebindingTime(opts dhcp4.Options) (time.Duration, error) {
	return parseDuration(opts, dhcp4.OptionRebindingTimeValue, "RebindingTime")
}

// DHCPLease is for DHCP
type DHCPLease struct {
	clientID      string
	ack           *dhcp4.Packet
	opts          dhcp4.Options
	link          netlink.Link
	renewalTime   time.Time
	rebindingTime time.Time
	expireTime    time.Time
	stop          chan struct{}
	wg            sync.WaitGroup
}

// AcquireLease gets an DHCP lease and then maintains it in the background
func AcquireLease(clientID, netns, ifName string) (*DHCPLease, error) {
	errCh := make(chan error, 1)
	l := &DHCPLease{
		clientID: clientID,
		stop:     make(chan struct{}),
	}

	log.Printf("%v: acquiring lease", clientID)

	l.wg.Add(1)
	go func() {
		errCh <- ns.WithNetNSPath(netns, func(_ ns.NetNS) error {
			defer l.wg.Done()

			link, err := netlink.LinkByName(ifName)
			if err != nil {
				return fmt.Errorf("error looking up %q: %v", ifName, err)
			}

			l.link = link

			if err = l.acquire(); err != nil {
				return err
			}

			log.Printf("%v: lease acquired, expiration is %v", l.clientID, l.expireTime)

			errCh <- nil

			l.maintain()
			return nil
		})
	}()

	if err := <-errCh; err != nil {
		return nil, err
	}

	return l, nil
}

// Stop is for DHCP
func (l *DHCPLease) Stop() {
	close(l.stop)
	l.wg.Wait()
}

func (l *DHCPLease) acquire() error {
	c, err := newDHCPClient(l.link)
	if err != nil {
		return err
	}
	defer c.Close()

	if (l.link.Attrs().Flags & net.FlagUp) != net.FlagUp {
		log.Printf("Link %q down. Attempting to set up", l.link.Attrs().Name)
		if err = netlink.LinkSetUp(l.link); err != nil {
			return err
		}
	}

	pkt, err := backoffRetry(func() (*dhcp4.Packet, error) {
		ok, ack, err := c.Request()
		switch {
		case err != nil:
			return nil, err
		case !ok:
			return nil, fmt.Errorf("DHCP server NACK'd own offer")
		default:
			return &ack, nil
		}
	})
	if err != nil {
		return err
	}

	return l.commit(pkt)
}

func (l *DHCPLease) commit(ack *dhcp4.Packet) error {
	opts := ack.ParseOptions()

	leaseTime, err := parseLeaseTime(opts)
	if err != nil {
		return err
	}

	rebindingTime, err := parseRebindingTime(opts)
	if err != nil || rebindingTime > leaseTime {
		// Per RFC 2131 Section 4.4.5, it should default to 85% of lease time
		rebindingTime = leaseTime * 85 / 100
	}

	renewalTime, err := parseRenewalTime(opts)
	if err != nil || renewalTime > rebindingTime {
		// Per RFC 2131 Section 4.4.5, it should default to 50% of lease time
		renewalTime = leaseTime / 2
	}

	now := time.Now()
	l.expireTime = now.Add(leaseTime)
	l.renewalTime = now.Add(renewalTime)
	l.rebindingTime = now.Add(rebindingTime)
	l.ack = ack
	l.opts = opts

	return nil
}

func (l *DHCPLease) maintain() {
	state := leaseStateBound

	for {
		var sleepDur time.Duration

		switch state {
		case leaseStateBound:
			sleepDur = l.renewalTime.Sub(time.Now())
			if sleepDur <= 0 {
				log.Printf("%v: renewing lease", l.clientID)
				state = leaseStateRenewing
				continue
			}

		case leaseStateRenewing:
			if err := l.renew(); err != nil {
				log.Printf("%v: %v", l.clientID, err)

				if time.Now().After(l.rebindingTime) {
					log.Printf("%v: renawal time expired, rebinding", l.clientID)
					state = leaseStateRebinding
				}
			} else {
				log.Printf("%v: lease renewed, expiration is %v", l.clientID, l.expireTime)
				state = leaseStateBound
			}

		case leaseStateRebinding:
			if err := l.acquire(); err != nil {
				log.Printf("%v: %v", l.clientID, err)

				if time.Now().After(l.expireTime) {
					log.Printf("%v: lease expired, bringing interface DOWN", l.clientID)
					l.downIface()
					return
				}
			} else {
				log.Printf("%v: lease rebound, expiration is %v", l.clientID, l.expireTime)
				state = leaseStateBound
			}
		}

		select {
		case <-time.After(sleepDur):

		case <-l.stop:
			if err := l.release(); err != nil {
				log.Printf("%v: failed to release DHCP lease: %v", l.clientID, err)
			}
			return
		}
	}
}

func (l *DHCPLease) downIface() {
	if err := netlink.LinkSetDown(l.link); err != nil {
		log.Printf("%v: failed to bring %v interface DOWN: %v", l.clientID, l.link.Attrs().Name, err)
	}
}

func (l *DHCPLease) renew() error {
	c, err := newDHCPClient(l.link)
	if err != nil {
		return err
	}
	defer c.Close()

	pkt, err := backoffRetry(func() (*dhcp4.Packet, error) {
		ok, ack, err := c.Renew(*l.ack)
		switch {
		case err != nil:
			return nil, err
		case !ok:
			return nil, fmt.Errorf("DHCP server did not renew lease")
		default:
			return &ack, nil
		}
	})
	if err != nil {
		return err
	}

	l.commit(pkt)
	return nil
}

// DHCPLease handles leasing of the DHCP
func (l *DHCPLease) release() error {
	log.Printf("%v: releasing lease", l.clientID)

	c, err := newDHCPClient(l.link)
	if err != nil {
		return err
	}
	defer c.Close()

	if err = c.Release(*l.ack); err != nil {
		return fmt.Errorf("failed to send DHCPRELEASE")
	}

	return nil
}

//IPNet is for subnets
func (l *DHCPLease) IPNet() (*net.IPNet, error) {
	mask := parseSubnetMask(l.opts)
	if mask == nil {
		return nil, fmt.Errorf("DHCP option Subnet Mask not found in DHCPACK")
	}

	return &net.IPNet{
		IP:   l.ack.YIAddr(),
		Mask: mask,
	}, nil
}

// Gateway is the IP gateway
func (l *DHCPLease) Gateway() net.IP {
	return parseRouter(l.opts)
}

// Routes is for routing
func (l *DHCPLease) Routes() []*types.Route {
	routes := parseRoutes(l.opts)
	return append(routes, parseCIDRRoutes(l.opts)...)
}

// jitter returns a random value within [-span, span) range
func jitter(span time.Duration) time.Duration {
	return time.Duration(float64(span) * (2.0*rand.Float64() - 1.0))
}

func backoffRetry(f func() (*dhcp4.Packet, error)) (*dhcp4.Packet, error) {
	var baseDelay time.Duration = resendDelay0

	for i := 0; i < resendCount; i++ {
		pkt, err := f()
		if err == nil {
			return pkt, nil
		}

		log.Print(err)

		time.Sleep(baseDelay + jitter(time.Second))

		if baseDelay < resendDelayMax {
			baseDelay *= 2
		}
	}

	return nil, nil
}

func newDHCPClient(link netlink.Link) (*dhcp4client.Client, error) {
	pktsock, err := dhcp4client.NewPacketSock(link.Attrs().Index)
	if err != nil {
		return nil, err
	}

	return dhcp4client.New(
		dhcp4client.HardwareAddr(link.Attrs().HardwareAddr),
		dhcp4client.Timeout(5*time.Second),
		dhcp4client.Broadcast(false),
		dhcp4client.Connection(pktsock),
	)
}
