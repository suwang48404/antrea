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

// +build linux

package util

import (
	"fmt"
	"net"
	"syscall"

	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ns"
	"golang.org/x/sys/unix"
	"k8s.io/klog"

	"github.com/vishvananda/netlink"
)

// GetIPNetDeviceFromIP returns a local IP/mask and associated device from IP.
func GetIPNetDeviceFromIP(localIP net.IP) (*net.IPNet, netlink.Link, error) {
	linkList, err := netlink.LinkList()
	if err != nil {
		return nil, nil, err
	}

	for _, link := range linkList {
		addrList, err := netlink.AddrList(link, unix.AF_INET)
		if err != nil {
			klog.Errorf("Failed to get addr list for device %s", link)
			continue
		}
		for _, addr := range addrList {
			if addr.IP.Equal(localIP) {
				return addr.IPNet, link, nil
			}
		}
	}
	return nil, nil, fmt.Errorf("unable to find local IP and device")
}

// GetNetLink returns dev link from name.
func GetNetLink(dev string) netlink.Link {
	link, err := netlink.LinkByName(dev)
	if err != nil {
		klog.Fatalf("cannot find dev %s: %w", dev, err)
	}
	return link
}

// GetPeerLinkBridge returns peer device and its attached bridge (if applicable)
// for device dev in network space indicated by nsPath
func GetNSPeerDevBridge(nsPath, dev string) (*net.Interface, string, error) {
	var peerIdx int
	netNS, err := ns.GetNS(nsPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get NS for path %s: %w", nsPath, err)
	}
	if err := netNS.Do(func(_ ns.NetNS) error {
		_, peerIdx, err = ip.GetVethPeerIfindex(dev)
		if err != nil {
			return fmt.Errorf("failed to get peer idx for dev %s in container %s: %w", dev, nsPath, err)
		}
		return nil
	}); err != nil {
		return nil, "", err
	}

	peerIntf, err := net.InterfaceByIndex(peerIdx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get interface for idx %d: %w", peerIdx, err)
	}
	peerLink, err := netlink.LinkByIndex(peerIdx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get link for idx %d: %w", peerIdx, err)
	}

	// not attached to an bridge.
	if peerLink.Attrs().MasterIndex <= 0 {
		return peerIntf, "", nil
	}

	bridgeLink, err := netlink.LinkByIndex(peerLink.Attrs().MasterIndex)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get master link for dev %s: %w", peerLink.Attrs().Name, err)
	}
	bridge, ok := bridgeLink.(*netlink.Bridge)
	if !ok {
		// master link is not bridge
		return peerIntf, "", nil
	}
	return peerIntf, bridge.Name, nil
}

// getNLBridge returns netlink Bridge with name bridgeName.
func getNLBridge(bridgeName string) (*netlink.Bridge, error) {
	bridgeLink, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return nil, fmt.Errorf("failed to get bridge link %s: %w", bridgeName, err)
	}
	bridge, ok := bridgeLink.(*netlink.Bridge)
	if !ok {
		return nil, fmt.Errorf("link %s(%s) is not a bridge", bridgeName, bridgeLink.Type())
	}
	return bridge, nil
}

// DetachDevFromBridge detaches device dev from bridge bridgeName.
func DetachDevFromBridge(dev *net.Interface, bridgeName string) error {
	bridge, err := getNLBridge(bridgeName)
	if err != nil {
		return err
	}
	devLink, err := netlink.LinkByIndex(dev.Index)
	if err != nil {
		return fmt.Errorf("failed to get dev link %s: %w", dev.Name, err)
	}
	if devLink.Attrs().MasterIndex != bridge.Index {
		return fmt.Errorf("dev %s(masterIdx=%d) is not attached bridge %s(idx=%d)",
			dev.Name, devLink.Attrs().MasterIndex, bridge.Name, bridge.Index)
	}
	if err := netlink.LinkSetNoMaster(devLink); err != nil {
		return fmt.Errorf("failed to detach dev %s from bridge %s: %w", dev.Name, bridgeName, err)
	}
	return nil
}

// AttachDevFromBridge attaches device dev to bridge bridgeName.
func AttachDevToBridge(dev *net.Interface, bridgeName string) error {
	bridge, err := getNLBridge(bridgeName)
	if err != nil {
		return err
	}
	devLink, err := netlink.LinkByIndex(dev.Index)
	if err != nil {
		return fmt.Errorf("failed to get dev link %s: %w", dev.Name, err)
	}
	if err := netlink.LinkSetMaster(devLink, bridge); err != nil {
		return fmt.Errorf("failed to attach dev %s to bridge %s: %w", dev.Name, bridgeName, err)

	}
	return nil
}

// GetNSDevInterface returns interface of dev in namespace nsPath.
func GetNSDevInterface(nsPath, dev string) (*net.Interface, error) {
	netNS, err := ns.GetNS(nsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get NS for path %s: %w", nsPath, err)
	}
	var intf *net.Interface
	if err := netNS.Do(func(_ ns.NetNS) error {
		intf, err = net.InterfaceByName(dev)
		if err != nil {
			return fmt.Errorf("failed to get interface %s in container %s: %w", dev, nsPath, err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return intf, nil
}

// CreateBridge creates a bridge.
func CreateBridge(name string) error {
	br := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name:   name,
			MTU:    1500,
			TxQLen: -1,
		},
	}
	err := netlink.LinkAdd(br)
	if err != nil && err != syscall.EEXIST {
		return err
	}
	return nil
}

// ConnectPatchPortToBridge creates vethpair with endpoint patchPortName and peerPatchPortName,
// and attach peerPatchPort to bridge bridgeName.
func ConnectPatchPortToBridge(bridgeName, patchPortName, peerPatchPortName string, MTU int) error {
	curNs, err := ns.GetCurrentNS()
	if err != nil {
		return fmt.Errorf("get current ns failed: %w", err)
	}
	veth, _, err := ip.SetupVethWithName(patchPortName, peerPatchPortName, MTU, curNs)
	if err != nil {
		return fmt.Errorf("veth-pair create failed: %w", err)
	}
	if err := AttachDevToBridge(&veth, bridgeName); err != nil {
		return err
	}
	return nil
}
