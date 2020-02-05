// Copyright 2020 Antrea Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package route

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/containernetworking/plugins/pkg/utils/sysctl"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"k8s.io/klog"

	"github.com/vmware-tanzu/antrea/pkg/agent/config"
	"github.com/vmware-tanzu/antrea/pkg/agent/iptables"
	"github.com/vmware-tanzu/antrea/pkg/agent/util"
)

const (
	// AntreaServiceTable is route table name for Antrea service traffic.
	AntreaServiceTable = "Antrea-service"
	// AntreaServiceTableIdx is route table index for Antrea service traffic.
	AntreaServiceTableIdx = 300
	routeTableConfigPath  = "/etc/iproute2/rt_tables"
	// AntreaIPRulePriority is Antrea IP rule priority
	AntreaIPRulePriority = 300
)

type Client interface {
	Initialize(nodeConfig *config.NodeConfig) error
	AddPeerCIDRRoute(peerPodCIDR *net.IPNet, gwLinkIdx int, peerNodeIP, peerGwIP net.IP) ([]*netlink.Route, error)
	ListPeerCIDRRoute() (map[string][]*netlink.Route, error)
	DeletePeerCIDRRoute(routes []*netlink.Route) error
	RemoveServiceRouting() error
	SetupPassThrough(isL2 bool) error
	MigrateRoutesToGw(linkName string) error
	UnMigrateRoutesFromGw(route *net.IPNet, linkName string) error
}

// Client is route client.
type client struct {
	nodeConfig *config.NodeConfig
	encapMode  config.TrafficEncapModeType
}

type serviceRtTableConfig struct {
	Idx  int
	Name string
}

func (s *serviceRtTableConfig) String() string {
	return fmt.Sprintf("%s: idx %d", s.Name, s.Idx)
}

func (s *serviceRtTableConfig) IsMainTable() bool {
	return s.Name == "main"
}

var (
	// ServiceRtTable contains Antrea service route table information.
	ServiceRtTable = &serviceRtTableConfig{Idx: 254, Name: "main"}
)

// NewClient returns a route client
func NewClient(encapMode config.TrafficEncapModeType) Client {
	return &client{encapMode: encapMode}
}

// Initialize sets up route tables for Antrea.
func (c *client) Initialize(nodeConfig *config.NodeConfig) error {
	c.nodeConfig = nodeConfig
	if c.encapMode.SupportsNoEncap() {
		ServiceRtTable.Idx = AntreaServiceTableIdx
		ServiceRtTable.Name = AntreaServiceTable
	}

	if ServiceRtTable.IsMainTable() {
		_ = c.RemoveServiceRouting()
		return nil
	}
	f, err := os.OpenFile(routeTableConfigPath, os.O_RDWR|os.O_APPEND, 0)
	if err != nil {
		klog.Fatalf("Unable to create service route table(open):  %v", err)
	}
	defer func() { _ = f.Close() }()

	oldTablesRaw := make([]byte, 1024)
	bLen, err := f.Read(oldTablesRaw)
	if err != nil {
		klog.Fatalf("Unable to create service route table(read): %v", err)
	}
	oldTables := string(oldTablesRaw[:bLen])
	newTable := fmt.Sprintf("%d %s", ServiceRtTable.Idx, ServiceRtTable.Name)

	if strings.Index(oldTables, newTable) == -1 {
		if _, err := f.WriteString(newTable); err != nil {
			klog.Fatalf("Failed to add antrea service route table: %v", err)
		}
	}

	gwConfig := c.nodeConfig.GatewayConfig
	if !c.encapMode.IsPassThrough() {
		// Add local podCIDR if applicable to service rt table.
		route := &netlink.Route{
			LinkIndex: gwConfig.LinkIndex,
			Scope:     netlink.SCOPE_LINK,
			Dst:       c.nodeConfig.PodCIDR,
			Table:     ServiceRtTable.Idx,
		}
		if err := netlink.RouteReplace(route); err != nil {
			klog.Fatalf("Failed to add link route to service table: %v", err)
		}
	}

	// create ip rule to select route table
	ipRule := netlink.NewRule()
	ipRule.IifName = c.nodeConfig.GatewayConfig.Name
	ipRule.Mark = iptables.RtTblSelectorValue
	ipRule.Mask = iptables.RtTblSelectorValue
	ipRule.Table = ServiceRtTable.Idx
	ipRule.Priority = AntreaIPRulePriority

	ruleList, err := netlink.RuleList(netlink.FAMILY_V4)
	if err != nil {
		klog.Fatalf("Failed to get ip rule: %v", err)
	}
	// Check for ip rule presence.
	for _, rule := range ruleList {
		if rule == *ipRule {
			return nil
		}
	}
	err = netlink.RuleAdd(ipRule)
	if err != nil {
		klog.Fatalf("Failed to create ip rule for service route table: %v", err)
	}
	return nil
}

// AddPeerCIDRRoute adds routes to route tables for Antrea use.
func (c *client) AddPeerCIDRRoute(peerPodCIDR *net.IPNet, gwLinkIdx int, peerNodeIP, peerGwIP net.IP) ([]*netlink.Route, error) {
	if peerPodCIDR == nil {
		return nil, fmt.Errorf("empty peer pod CIDR")
	}

	// install routes
	routes := []*netlink.Route{
		{
			Dst:       peerPodCIDR,
			Flags:     int(netlink.FLAG_ONLINK),
			LinkIndex: gwLinkIdx,
			Gw:        peerGwIP,
			Table:     ServiceRtTable.Idx,
		},
	}

	// If service route table and main route table is not the same , add
	// peer CIDR to main route table too (i.e in NoEncap and hybrid mode)
	if !ServiceRtTable.IsMainTable() {
		if c.encapMode.NeedsEncapToPeer(peerNodeIP, c.nodeConfig.NodeIPAddr) {
			// need overlay tunnel
			routes = append(routes, &netlink.Route{
				Dst:       peerPodCIDR,
				Flags:     int(netlink.FLAG_ONLINK),
				LinkIndex: gwLinkIdx,
				Gw:        peerGwIP,
			})
		} else if !c.encapMode.NeedsRoutingToPeer(peerNodeIP, c.nodeConfig.NodeIPAddr) {
			routes = append(routes, &netlink.Route{
				Dst: peerPodCIDR,
				Gw:  peerNodeIP,
			})
		}
		// If Pod traffic needs underlying routing support, it is handled by host default route.
	}

	// clean up function if any route add failed
	deleteRtFn := func() {
		for _, route := range routes {
			_ = netlink.RouteDel(route)
		}
	}

	var err error = nil
	for _, route := range routes {
		if err := netlink.RouteReplace(route); err != nil {
			deleteRtFn()
			err = fmt.Errorf("failed to install route to peer %s with netlink: %v", peerNodeIP, err)
			return nil, err
		}
	}
	return routes, err
}

// ListPeerCIDRRoute returns list of routes from peer and local CIDRs
func (c *client) ListPeerCIDRRoute() (map[string][]*netlink.Route, error) {
	// get all routes on gw0 from service table.
	filter := &netlink.Route{
		Table:     ServiceRtTable.Idx,
		LinkIndex: c.nodeConfig.GatewayConfig.LinkIndex}
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, filter, netlink.RT_FILTER_TABLE|netlink.RT_FILTER_OIF)
	if err != nil {
		return nil, err
	}

	rtMap := make(map[string][]*netlink.Route)
	for _, rt := range routes {
		// rt is reference to actual data, as it changes,
		// it cannot be used for assignment
		tmpRt := rt
		rtMap[rt.Dst.String()] = append(rtMap[rt.Dst.String()], &tmpRt)
	}

	if !ServiceRtTable.IsMainTable() {
		// get all routes on gw0 from main table.
		filter.Table = 0
		routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, filter, netlink.RT_FILTER_OIF)
		if err != nil {
			return nil, err
		}
		for _, rt := range routes {
			// rt is reference to actual data, as it changes,
			// it cannot be used for assignment
			tmpRt := rt
			rtMap[rt.Dst.String()] = append(rtMap[rt.Dst.String()], &tmpRt)
		}

		// now get all routes gw0 on other interfaces from main table.
		routes, err = netlink.RouteListFiltered(netlink.FAMILY_V4, nil, 0)
		if err != nil {
			return nil, err
		}
		for _, rt := range routes {
			if rt.Dst == nil {
				continue
			}
			// insert the route if it is CIDR route and has not been added already.
			// routes with same dst are different if table or linkIndex differs.
			if rl, ok := rtMap[rt.Dst.String()]; ok && (rl[len(rl)-1].LinkIndex != rt.LinkIndex || rl[len(rl)-1].Table != rt.Table) {
				tmpRt := rt
				rtMap[rt.Dst.String()] = append(rl, &tmpRt)
			}
		}
	}
	return rtMap, nil
}

// DeletePeerCIDRRoute deletes routes.
func (c *client) DeletePeerCIDRRoute(routes []*netlink.Route) error {
	for _, r := range routes {
		klog.V(4).Infof("Deleting route %v", r)
		if err := netlink.RouteDel(r); err != nil && err != unix.ESRCH {
			return err
		}
	}
	return nil
}

func (c *client) readRtTable() (string, error) {
	f, err := os.OpenFile(routeTableConfigPath, os.O_RDONLY, 0)
	if err != nil {
		return "", fmt.Errorf("route table(open): %w", err)
	}
	defer func() { _ = f.Close() }()

	tablesRaw := make([]byte, 1024)
	bLen, err := f.Read(tablesRaw)
	if err != nil {
		return "", fmt.Errorf("route table(read): %w", err)
	}
	return string(tablesRaw[:bLen]), nil
}

// RemoveServiceRouting removes service routing setup.
func (c *client) RemoveServiceRouting() error {
	// remove service table
	tables, err := c.readRtTable()
	if err != nil {
		return err
	}
	newTable := fmt.Sprintf("%d %s", AntreaServiceTableIdx, AntreaServiceTable)
	if strings.Index(tables, newTable) != -1 {
		tables = strings.Replace(tables, newTable, "", -1)
		f, err := os.OpenFile(routeTableConfigPath, os.O_WRONLY|os.O_TRUNC, 0)
		if err != nil {
			return fmt.Errorf("route table(open): %w", err)
		}
		defer func() { _ = f.Close() }()
		if _, err = f.WriteString(tables); err != nil {
			return fmt.Errorf("route table(write): %w", err)
		}
	}

	// flush service table
	filter := &netlink.Route{
		Table:     AntreaServiceTableIdx,
		LinkIndex: c.nodeConfig.GatewayConfig.LinkIndex}
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, filter, netlink.RT_FILTER_TABLE|netlink.RT_FILTER_OIF)
	if err != nil {
		return fmt.Errorf("route table(list): %w", err)
	}
	for _, route := range routes {
		if err = netlink.RouteDel(&route); err != nil {
			return fmt.Errorf("route delete: %w", err)
		}
	}

	// delete ip rule for service table
	ipRule := netlink.NewRule()
	ipRule.IifName = c.nodeConfig.GatewayConfig.Name
	ipRule.Mark = iptables.RtTblSelectorValue
	ipRule.Mask = iptables.RtTblSelectorValue
	ipRule.Table = AntreaServiceTableIdx
	ipRule.Priority = AntreaIPRulePriority
	if err = netlink.RuleDel(ipRule); err != nil {
		return fmt.Errorf("ip rule delete: %w", err)
	}
	return nil
}

// resolveDefaultRouteNHMAC resolves the MAC of default route next
// hop on service route table.
func (c *client) resolveDefaultRouteNHMAC() (net.HardwareAddr, error) {
	// This MAC is relevant to L2 mode only.
	// TODO, for now uses AKS default MAC.
	return net.ParseMAC("12:34:56:78:9a:bc")
}

// SetupPassThrough configures routing needed by traffic pass-through mode.
func (c *client) SetupPassThrough(isL2 bool) error {
	gwLink := util.GetNetLink(c.nodeConfig.GatewayConfig.Name)
	_, gwIP, _ := net.ParseCIDR(fmt.Sprintf("%s/32", c.nodeConfig.NodeIPAddr.IP.String()))
	if err := netlink.AddrReplace(gwLink, &netlink.Addr{IPNet: gwIP}); err != nil {
		klog.Fatalf("Failed to add address %s to gw %s: %v", gwIP, gwLink.Attrs().Name, err)
	}
	if isL2 {
		ctrls := []struct {
			fmt string
			val string
		}{
			// relax rp_filter so that packets from gw0 are allowed
			{"net.ipv4.conf.%s.rp_filter", "2"},
			// drop gratuitous arp
			{"net.ipv4.conf.%s.drop_gratuitous_arp", "1"},
			// do not reply arp
			{"net.ipv4.conf.%s.arp_ignore", "8"},
		}
		for _, ctl := range ctrls {
			ctlStr := fmt.Sprintf(ctl.fmt, gwLink.Attrs().Name)
			if _, err := sysctl.Sysctl(ctlStr, ctl.val); err != nil {
				klog.Fatalf("Failed to set sysctl %s: %v", ctl, err)
			}
		}
	}

	// add default route to service table.
	_, defaultRt, _ := net.ParseCIDR("0/0")
	nhIP := net.ParseIP("169.254.253.1")
	route := &netlink.Route{
		LinkIndex: gwLink.Attrs().Index,
		Table:     ServiceRtTable.Idx,
		Flags:     int(netlink.FLAG_ONLINK),
		Dst:       defaultRt,
		Gw:        nhIP,
	}
	if err := netlink.RouteReplace(route); err != nil {
		klog.Fatalf("Failed to add default route to service table: %v", err)
	}

	// add static neighbor to next hop so that no ARPING is ever required on gw0
	nhMAC, _ := c.resolveDefaultRouteNHMAC()
	neigh := &netlink.Neigh{
		LinkIndex:    gwLink.Attrs().Index,
		Family:       netlink.FAMILY_V4,
		State:        netlink.NUD_PERMANENT,
		IP:           nhIP,
		HardwareAddr: nhMAC,
	}
	if err := netlink.NeighSet(neigh); err != nil {
		klog.Fatalf("Failed to add neigh %v to gw %s: %v", neigh, gwLink.Attrs().Name, err)
	}
	return nil
}

// MigrateRoutesToGw moves routes (including assigned IP addresses if any) from link linkName to
// host gateway.
func (c *client) MigrateRoutesToGw(linkName string) error {
	gwLink := util.GetNetLink(c.nodeConfig.GatewayConfig.Name)
	link, err := netlink.LinkByName(linkName)
	if err != nil {
		return fmt.Errorf("failed to get link %s: %w", linkName, err)
	}

	// Swap route first then address, otherwise route gets removed when address is removed.
	routes, err := netlink.RouteList(link, netlink.FAMILY_V4)
	if err != nil {
		return fmt.Errorf("failed to get routes for link %s: %w", linkName, err)
	}
	for _, route := range routes {
		route.LinkIndex = gwLink.Attrs().Index
		if err = netlink.RouteReplace(&route); err != nil {
			return fmt.Errorf("failed to add route %v to link %s: %w", &route, gwLink.Attrs().Name, err)
		}
	}

	// swap address if any
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return fmt.Errorf("failed to get addresses for %s: %w", linkName, err)
	}
	for _, addr := range addrs {
		if err = netlink.AddrDel(link, &addr); err != nil {
			klog.Errorf("failed to delete addr %v from %s: %w", addr, link, err)
		}
		tmpAddr := &netlink.Addr{IPNet: addr.IPNet}
		if err = netlink.AddrReplace(gwLink, tmpAddr); err != nil {
			return fmt.Errorf("failed to add addr %v to gw %s: %w", addr, gwLink.Attrs().Name, err)
		}
	}
	return nil
}

// UnMigrateRoutesToGw move route from gw to link linkName if provided; otherwise route is deleted
func (c *client) UnMigrateRoutesFromGw(route *net.IPNet, linkName string) error {
	gwLink := util.GetNetLink(c.nodeConfig.GatewayConfig.Name)
	var link netlink.Link = nil
	var err error
	if len(linkName) > 0 {
		link, err = netlink.LinkByName(linkName)
		if err != nil {
			return fmt.Errorf("failed to get link %s: %w", linkName, err)
		}
	}
	routes, err := netlink.RouteList(gwLink, netlink.FAMILY_V4)
	if err != nil {
		return fmt.Errorf("failed to get routes for link %s: %w", gwLink.Attrs().Name, err)
	}
	for _, rt := range routes {
		if route.String() == rt.Dst.String() {
			if link != nil {
				rt.LinkIndex = link.Attrs().Index
				return netlink.RouteReplace(&rt)
			}
			return netlink.RouteDel(&rt)
		}
	}
	return nil
}
