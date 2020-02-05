// Copyright 2019 Antrea Authors
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

package agent

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"

	"github.com/vmware-tanzu/antrea/pkg/agent/config"
	"github.com/vmware-tanzu/antrea/pkg/agent/iptables"
	"github.com/vmware-tanzu/antrea/pkg/agent/route"
	"github.com/vmware-tanzu/antrea/pkg/agent/util"
)

func ExecOutputTrim(cmd string) (string, error) {
	out, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		return "", err
	}
	return strings.Join(strings.Fields(string(out)), ""), nil
}

var (
	_, podCIDR, _       = net.ParseCIDR("10.10.10.0/24")
	nodeIP, nodeLink, _ = util.GetIPNetDeviceFromIP(func() net.IP {
		conn, _ := net.Dial("udp", "8.8.8.8:80")
		defer func() { _ = conn.Close() }()
		return conn.LocalAddr().(*net.UDPAddr).IP
	}())
	localPeerIP       = ip.NextIP(nodeIP.IP)
	remotePeerIP      = net.ParseIP("50.50.50.1")
	_, serviceCIDR, _ = net.ParseCIDR("200.200.0.0/16")
	gwIP              = net.ParseIP("10.10.10.1")
	gwMAC, _          = net.ParseMAC("12:34:56:78:bb:cc")
	gwName            = "gw0"
	svcTblIdx         = route.AntreaServiceTableIdx
	svcTblName        = route.AntreaServiceTable
	mainTblIdx        = 254
	gwConfig          = &config.GatewayConfig{IP: gwIP, MAC: gwMAC, Name: gwName}
	nodeConfig        = &config.NodeConfig{
		Name:          "test",
		PodCIDR:       nil,
		NodeIPAddr:    nodeIP,
		GatewayConfig: gwConfig,
	}
)

func TestRouteTable(t *testing.T) {
	if _, incontainer := os.LookupEnv("INCONTAINER"); !incontainer {
		// test changes file system, routing table. Run in contain only
		t.Skipf("Skip test runs only in container")
	}

	// create dummy gw interface
	gwLink := &netlink.Dummy{}
	gwLink.Name = gwName
	err := netlink.LinkAdd(gwLink)
	require.Nil(t, err)
	err = netlink.LinkSetUp(gwLink)
	require.Nil(t, err)

	defer func() {
		_ = netlink.LinkDel(gwLink)
	}()

	nodeConfig.GatewayConfig.Name = gwLink.Attrs().Name
	nodeConfig.GatewayConfig.LinkIndex = gwLink.Attrs().Index
	refRouteTablesStr, _ := ExecOutputTrim("cat /etc/iproute2/rt_tables")
	tcs := []struct {
		// variations
		mode     config.TrafficEncapModeType
		podCIDR  *net.IPNet
		peerCIDR string
		peerIP   net.IP
		// expectations
		expSvcTbl bool
		expIPRule bool
		expRoutes map[int]netlink.Link // keyed on rt id, and val indicates outbound dev
	}{
		{mode: config.TrafficEncapModeEncap, podCIDR: podCIDR, peerCIDR: "10.10.20.0/24", peerIP: localPeerIP,
			expSvcTbl: false, expIPRule: false, expRoutes: map[int]netlink.Link{mainTblIdx: gwLink}},
		{mode: config.TrafficEncapModeNoEncap, podCIDR: podCIDR, peerCIDR: "10.10.30.0/24", peerIP: localPeerIP,
			expSvcTbl: true, expIPRule: true, expRoutes: map[int]netlink.Link{svcTblIdx: gwLink, mainTblIdx: nodeLink}},
		{mode: config.TrafficEncapModeNoEncap, podCIDR: podCIDR, peerCIDR: "10.10.40.0/24", peerIP: remotePeerIP,
			expSvcTbl: true, expIPRule: true, expRoutes: map[int]netlink.Link{svcTblIdx: gwLink}},
		{mode: config.TrafficEncapModeHybrid, podCIDR: podCIDR, peerCIDR: "10.10.50.0/24", peerIP: localPeerIP,
			expSvcTbl: true, expIPRule: true, expRoutes: map[int]netlink.Link{svcTblIdx: gwLink, mainTblIdx: nodeLink}},
		{mode: config.TrafficEncapModeHybrid, podCIDR: podCIDR, peerCIDR: "10.10.60.0/24", peerIP: remotePeerIP,
			expSvcTbl: true, expIPRule: true, expRoutes: map[int]netlink.Link{svcTblIdx: gwLink, mainTblIdx: gwLink}},
	}

	for _, tc := range tcs {
		nodeConfig.PodCIDR = tc.podCIDR
		t.Logf("Running test with mode %s peer cidr %s peer ip %s node config %s", tc.mode, tc.peerCIDR, tc.peerIP, nodeConfig)
		routeClient := route.NewClient(tc.mode)
		err = routeClient.Initialize(nodeConfig)
		require.Nil(t, err)
		// Call initialize twice and verify no duplicates
		err = routeClient.Initialize(nodeConfig)
		require.Nil(t, err)

		// verify route tables
		expRouteTablesStr := refRouteTablesStr
		if tc.expSvcTbl {
			expRouteTablesStr = fmt.Sprintf("%s%d%s", refRouteTablesStr, route.ServiceRtTable.Idx, route.ServiceRtTable.Name)
		}
		routeTables, err := ExecOutputTrim("cat /etc/iproute2/rt_tables")
		require.Nil(t, err)
		require.Equal(t, expRouteTablesStr, routeTables)

		if tc.expSvcTbl && tc.podCIDR != nil {
			expRouteStr := fmt.Sprintf("%s dev %s scope link", tc.podCIDR, gwName)
			expRouteStr = strings.Join(strings.Fields(expRouteStr), "")
			ipRoute, _ := ExecOutputTrim(fmt.Sprintf("ip route show table %d | grep %s", svcTblIdx, tc.podCIDR))
			require.Equal(t, expRouteStr, ipRoute)
		}

		// verify ip rules
		expIPRulesStr := ""
		if tc.expIPRule {
			expIPRulesStr = fmt.Sprintf("%d: from all fwmark %#x/%#x iif %s lookup %s",
				route.AntreaIPRulePriority, iptables.RtTblSelectorValue, iptables.RtTblSelectorValue,
				gwName, svcTblName)
			expIPRulesStr = strings.Join(strings.Fields(expIPRulesStr), "")
		}
		ipRule, _ := ExecOutputTrim(fmt.Sprintf("ip rule | grep %x", iptables.RtTblSelectorValue))
		require.Equal(t, expIPRulesStr, ipRule)

		// verify routes
		var peerCIDR *net.IPNet
		var nhCIDRIP net.IP
		if len(tc.peerCIDR) > 0 {
			_, peerCIDR, _ = net.ParseCIDR(tc.peerCIDR)
			nhCIDRIP = ip.NextIP(peerCIDR.IP)
		}
		routes, err := routeClient.AddPeerCIDRRoute(peerCIDR, gwLink.Index, tc.peerIP, nhCIDRIP)
		require.Equal(t, len(routes), len(tc.expRoutes))

		for tblIdx, link := range tc.expRoutes {
			nhIP := nhCIDRIP
			onlink := "onlink"
			if link.Attrs().Name != gwName {
				nhIP = tc.peerIP
				onlink = ""
			}
			expRouteStr := fmt.Sprintf("%s via %s dev %s %s", peerCIDR, nhIP, link.Attrs().Name, onlink)
			expRouteStr = strings.Join(strings.Fields(expRouteStr), "")
			ipRoute, _ := ExecOutputTrim(fmt.Sprintf("ip route show table %d | grep %s", tblIdx, tc.peerCIDR))
			if len(ipRoute) > len(expRouteStr) {
				ipRoute = ipRoute[:len(expRouteStr)]
			}
			require.Equal(t, expRouteStr, ipRoute)
		}

		// test list route
		rtMap, err := routeClient.ListPeerCIDRRoute()
		require.Nil(t, err)

		t.Logf("list route %s", rtMap)
		expRtCount := 2
		if !tc.mode.SupportsNoEncap() {
			expRtCount = 1
		}
		require.Equal(t, expRtCount, len(rtMap))
		require.Contains(t, rtMap, tc.peerCIDR)
		require.Equal(t, len(tc.expRoutes), len(rtMap[tc.peerCIDR]))

		// test delete route
		err = routeClient.DeletePeerCIDRRoute(rtMap[tc.peerCIDR])
		require.Nil(t, err)

		if tc.mode != config.TrafficEncapModeEncap {
			err = routeClient.RemoveServiceRouting()
			require.Nil(t, err)
		}

		// verify route table cleanup works
		routeTables, err = ExecOutputTrim("cat /etc/iproute2/rt_tables")
		require.Nil(t, err)
		require.Equal(t, refRouteTablesStr, routeTables)
		// verify no ip rule
		output, err := ExecOutputTrim(fmt.Sprintf("ip rule | grep %x", iptables.RtTblSelectorValue))
		require.Errorf(t, err, output)

		// verify no routes
		output, err = ExecOutputTrim(fmt.Sprintf("ip route show table %s", route.AntreaServiceTable))
		require.Errorf(t, err, output)
	}
}

func TestRouteTablePassThrough(t *testing.T) {
	if _, incontainer := os.LookupEnv("INCONTAINER"); !incontainer {
		// test changes file system, routing table. Run in contain only
		t.Skipf("Skip test runs only in container")
	}

	testCases := []struct {
		isL2 bool
	}{
		{isL2: true},
		{isL2: false},
	}
	for _, tc := range testCases {
		t.Logf("Running pass-through, l2 = %v ", tc.isL2)
		// create dummy gw interface
		gwLink := &netlink.Dummy{}
		gwLink.Name = gwName
		err := netlink.LinkAdd(gwLink)
		require.Nilf(t, err, fmt.Sprintf("%v", err))
		err = netlink.LinkSetUp(gwLink)
		require.Nil(t, err)

		routeClient := route.NewClient(config.TrafficEncapModePassThrough)
		err = routeClient.Initialize(nodeConfig)
		require.Nil(t, err)
		err = routeClient.SetupPassThrough(tc.isL2)
		require.Nil(t, err)
		//verify gw IP
		gwName := nodeConfig.GatewayConfig.Name
		gwIPOut, err := ExecOutputTrim(fmt.Sprintf("ip addr show %s", gwName))
		require.Nil(t, err)
		gwIP := net.IPNet{
			IP:   nodeConfig.NodeIPAddr.IP,
			Mask: net.CIDRMask(32, 32),
		}
		require.Contains(t, gwIPOut, gwIP.String())
		// verify sysctl
		if tc.isL2 {
			gwSysctlOut, err := ExecOutputTrim(fmt.Sprintf("sysctl -a | grep %s", gwName))
			require.Nil(t, err)
			expectGwCtls := []string{
				fmt.Sprintf("net.ipv4.conf.%s.rp_filter=2", gwName),
				fmt.Sprintf("net.ipv4.conf.%s.drop_gratuitous_arp=1", gwName),
				fmt.Sprintf("net.ipv4.conf.%s.arp_ignore=8", gwName),
			}
			for _, ctl := range expectGwCtls {
				require.Contains(t, gwSysctlOut, ctl)
			}
		}
		// verify default routes and neigh
		expRoute := strings.Join(strings.Fields(
			"default via 169.254.253.1 dev gw0 onlink"), "")
		routeOut, err := ExecOutputTrim(fmt.Sprintf("ip route show table %d", svcTblIdx))
		require.Equal(t, expRoute, routeOut)
		expNeigh := strings.Join(strings.Fields(
			"169.254.253.1 dev gw0 lladdr 12:34:56:78:9a:bc PERMANENT"), "")
		neighOut, err := ExecOutputTrim(fmt.Sprintf("ip neigh | grep %s", gwName))
		require.Equal(t, expNeigh, neighOut)

		if !tc.isL2 {
			cLink := &netlink.Dummy{}
			cLink.Name = "containerLink"
			err = netlink.LinkAdd(cLink)
			require.Nilf(t, err, fmt.Sprintf("%v", err))
			err = netlink.LinkSetUp(cLink)
			require.Nil(t, err)

			_, ipAddr, _ := net.ParseCIDR("10.10.1.1/32")
			_, hostRt, _ := net.ParseCIDR("10.10.1.2/32")
			err = netlink.AddrAdd(cLink, &netlink.Addr{IPNet: ipAddr})
			require.Nil(t, err)
			rt := &netlink.Route{
				LinkIndex: cLink.Index,
				Scope:     netlink.SCOPE_LINK,
				Dst:       hostRt,
			}
			err = netlink.RouteAdd(rt)
			require.Nilf(t, err, fmt.Sprintf("%v", err))
			t.Logf("route %v indx %d, iindx %d added", rt, rt.LinkIndex, rt.ILinkIndex)

			// verify route is migrated.
			err = routeClient.MigrateRoutesToGw(cLink.Name)
			require.Nil(t, err)
			expRoute = strings.Join(strings.Fields(
				fmt.Sprintf("%s dev %s scope link", hostRt.IP, gwName)), "")
			output, _ := ExecOutputTrim(fmt.Sprintf("ip route show"))
			require.Containsf(t, output, expRoute, output)
			output, _ = ExecOutputTrim(fmt.Sprintf("ip add show %s", gwName))
			require.Containsf(t, output, ipAddr.String(), output)

			// verify route being removed after unmigrate
			err = routeClient.UnMigrateRoutesFromGw(hostRt, "")
			require.Nil(t, err)
			output, _ = ExecOutputTrim(fmt.Sprintf("ip route show"))
			require.NotContainsf(t, output, expRoute, output)
			// note unmigrate does not remove ip addresses given to gw0
			output, _ = ExecOutputTrim(fmt.Sprintf("ip add show %s", gwName))
			require.Containsf(t, output, ipAddr.String(), output)
		}
		_ = netlink.LinkDel(gwLink)
	}
}
