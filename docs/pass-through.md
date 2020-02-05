# Running Antrea In Pass-Through Mode

Antrea supports chaining with other CNI implementations such as EKS, AKS. In this mode, 
Antrea enforces k8s network policies, and delegates Pod IP management
and network connectivity to the primary CNI.

The pass-through mode is designed to be compatible with all major CNI implementations, 
with emphasis to support cloud provider managed clusters such asAKS, EKS, or potentially
some flavors of GKE clusters.  

## Deploy and Test Antrea In Cloud 
Assuming you already have a cloud managed k8s cluster, and have KUBECONFIG environment
variable set to configuration file of that k8s cluster. You may also refer to [here](cloud.md)
to create an AKS or EKS cluster on the flight.

Create an Antrea manifest for pass-through mode, for instance
```bash
./hack/generate-manifest.sh --encap-mode passThrough > ~/my-yaml/antrea-passthrough.yaml
```

Adjust defaultMTU, and serviceCIDR values of antrea-agent.conf in the manifest to your cluster, and 
apply the manifest to the cluster, for instance
```bash
kubectl apply -f ~/my-yaml/antrea-passthrough.yaml 
```

Now Antrea should be plugged into the k8s cluster and enforces any network policies.
You can now validate Antrea network policy engine by run k8s conformance and network policy tests.
```bash
./ci/run-k8s-e2e-tests.sh --e2e-all
```
You may need to install a lower version of sonobuoy 
[here](https://github.com/vmware-tanzu/sonobuoy/releases/), and set environment variable SONOBUOY to
path to the executable, if you k8s cluster is lower version;


### Caveats
When install Antrea after a k8s cluster already has some Pods running, these pre-installed Pods
will not have Antrea managing their network policies. If they re-initiate, they then are under Antrea.
For instance,
```bash
kubectl scale deployment coredns --replicas 0 -n kube-system
kubectl scale deployment coredns --replicas 2 -n kube-system
```

## Design

### Connecting to CNI Topology
Antrea is designed to work as network policy plug-in for the following CNI topology models. For
as long as a CNI implementation fits into one of these two models, Antrea may be inserted for
network policy in that CNI's environment.

#### Switched CNI Topology
<img src="/docs/assets/l2-cni.svg" width="600" alt="Antrea Switched CNI">

The above diagram depicts a switched CNI network topology on the left, and what it looks like 
after Antrea via OVS bridge is inserted into the data path.

In a typical switched CNI topology such as azure AKS, each Pod is connected to a bridge via an veth
-pair. 
The bridge typically has up-link access to worker Node external, but it is not required for Antrea 
to work. Intra-Node Pod traffic are switched within the bridge, and inter-Node Pod traffic are 
switched by bridge via up-link to/from outsider the worker Node. When Pod traffic requires iptables
 help 
for load balancing and SNAT, the traffic up-calls host network IP stack without leaving the
bridge. For instance on a linux Node, bridge iptables up-call is enabled via  
```
sysctl net.bridge.bridge-nf-call-iptables=1
```

When a Pod is instantiated, the containerd runtime first calls the primary CNI to configure
Pod's IP and route table, DNS etc, and to connect Pod to the bridge. 
Due to chaining, containerd runtime then calls Antrea, and Antrea moves the veth-pair to 
OVS bridge and creates a single patch port (another vethpair) connecting OVS bridge the host
 network
bridge. The right side of diagram depicts new network connectivity after OVS bridge has been
inserted into the Pod traffic's data path. This allows Antrea agent to program OVS flows that
implements network policies.

The OVS bridge operates as an bump-on-the-wire entity in that it
1) Does not modify Pod traffic in either L2 or L3 layer.
2) Guarantees traffic received by either Pods or host network bridge are exactly the same
(with exception of network policy application) as if there is no OVS bridge.
3) And does not introduce any new MAC address into the host network bridge, as those traffic
may be switched out of local Node, and subject to traffic spoof-guarding unbeknown to Antrea.

To satisfy these requirements, Antrea requires no knowledge of Pod's network configurations nor
how primary CNI network are connected, it simply needs to program the following OVS flows in 
the OVS bridge:
1) ARPs are free flowing as if they are in a regular L2 switch
2) IP packets with destination MAC matches to an local Pod are switched to that local Pod
3) All other IP packets are switched to host network bridge via patch port, where they may 
be switch out out of local Node via up-link, or be arriving at local Node host network 
spaces.

These flows should satisfy all Pod traffic patterns with exception of Pod-to-Service traffic
that we will discuss later.

### Routed CNI Topology

<img src="/docs/assets/l3-cni.svg" width="600" alt="Antrea Switched CNI">

The above diagram depicts a routed CNI network topology on the left, and what it looks like 
after Antrea via OVS bridge is inserted into the data path.

In a routed CNI network topology, such as AWS EKS, each Pod connects to host network via an point-to
-point like device, such as (but not limited to) an veth-pair . On the host network, on each PtP
 device, a host route with
corresponding Pod's IP address as destination is created. On each Pod, routes are configured
to ensure all outgoing traffic is sent over this PtP devices, and incoming traffic is received
on this PtP device. This is a spoke-and-hub model, where to/from Pod traffic, even within the
same Node must first come to host network and being routed by it.

With Antrea inserted into the data path on the right, the Antrea agent attaches Pod's PtP devices
to the OVS bridge, and moves the host route to the Pod to gw0 interface from PtP devices. 

In this model, Antrea needs to satisfy that 
1) All IP packets received and sent on each Pod and on gw0 via OVS bridge are exactly the same if
 they are received and sent on Pod via PtP devices.

There are no L2 requirements as all MACS stay within the OVS. 
Antrea requires no knowledge of Pod network configurations nor
how CNI network are connected, it simply needs to program the following OVS flows in the OVS bridge:
1) An default ARP responder flow that answers any ARP request with the same MAC. Its sole
purpose is so that a Pod's neighbor may be resolved, so that IP packets may be sent by that Pod.
2) IP packets are routed based on its destination IP if it matches any local Pod's IP
3) All other IP packets are routed to host network via gw0 interface.

These flows should satisfy all Pod traffic patterns with exception of Pod-to-Service traffic
that we will address later.

## Handling Pod-To-Service
The discussion in this section is relevant also to Pod-to-Service traffic in noEncap traffic
mode. Antrea applies same principle to handle Pod-to-service traffic in all traffic modes where
 there are no traffic encapsulation.

Antrea uses kube-proxy for load balancing at present, and supports also Pod level 
network policy enforcement.
This means that for any Pod-to-service traffic, it needs to be 
1) first sent to host network for load balancing (DNAT), then
2) sent back to OVS bridge for network policy processing
3) if DNATed destination in a) is inter-Node Pod or external network entity, is sent back
to host network space yet again to be forwarded via host network.

We refers the last traffic pattern as re-entrance traffic because in this pattern, a IP packet 
enters host network space twice -- first for load balancing, and second for routing/switching.

Denote VIP as cluster IP of a service, SP_IP/DP_IP as respective client and server Pod IP; 
VPort as service port of a service, TPort as target port of server Pod, and SPort original 
source port. The re-entrance service request to host network in first and second entrance, and
 its reply would be like this

```
request/service: SP_IP/SPort->VIP/VPort  :: LB(DNAT) :: SP_IP/SPort->DP_IP/TPort :: Route :: gw0/OVS
request/forwrding: SP_IP/SPort->DP_IP/TPort :: Route :: uplink/out
reply: DP_IP/TPort -> SP_IP/SPort :: LB (unDNAT) :: VIP/VPort->SP_IP/Sport :: Route/Switch :: OVS
```

#### Routing 
Note that the request with destination IP DP_IP requires to be routed differently in service and 
forwarding case. 
Antrea creates a customized antrea_service route table, it is used in conjunction with ip-rule and iptables to specifically handle service traffic. It works as follows
1) At Antrea initialization, an iptable rule is created in mangle table that marks any traffic with service IP from gw0, 
2) At Antrea initialization, an ip-rule is added to select antrea_service route table as routing table if traffic is marked.
3) At Antrea initialization, an default route entry is added to antrea_service route table to
 forward all traffic to gw0.

The outcome may be something like
```bash
ip neigh | grep gw0
169.254.253.1 dev gw0 lladdr 12:34:56:78:9a:bc PERMANENT

ip route show table 300 #tbl_idx=300 is antrea_service
default via 169.254.253.1 dev gw0 onlink 

ip rule | grep gw0
300:	from all fwmark 0x800/0x800 iif gw0 lookup 300 

iptables -t mangle  -L ANTREA-MANGLE 
Chain ANTREA-MANGLE (1 references)
target     prot opt source               destination         
MARK       all  --  anywhere             10.0.0.0/16          /* Antrea: mark service traffic */ MARK or 0x800
MARK       all  --  anywhere            !10.0.0.0/16          /* Antrea: unmark post LB service traffic */ MARK and 0x0
```

The above configuration allows Pod-to-service traffic uses different route table after load balance,
and to be steered back OVS bridge for network policy enforcements.

#### Conntrack
Note also that the re-entrance traffic, an request for service, after load balancing, routed to
 OVS bridge via gw0, has exactly the same 5-tuple when it re-enters the host network for forwarding.

The request re-entering with same 5-tuples confuses linux connection tracking. It considered the 
re-entry packet as a new connection flow that has a conflicting source port with connection 
source port allocated in load balancing. In turn, the re-entrance packet generates another 
SNAT connection. The effect is that service IP DNAT connection is not discovered by the reply, 
and no UnDNAT takes place, and the reply is not recognized, and therefore is dropped by requesting
 Pod.
   
Antrea uses the following mechanism to handle Pod-to-service traffic re-entrance to host network
 space, and bypass connection tracking.
1) In OVS bridge, adds flow that marks any re-entrance traffic with a special source MAC.
2) In OVS bridge, adds flow that causes any re-entrance traffic to bypasses conntrack in OVS
 zone.
3) In host network space raw iptable, adds a rule that if matching special MAC, bypass connection
 tracking in host connection zone.

#### Network Policy Consideration
Note that in re-entrance mode, the original reply packets do not make into OVS, it is 
unDATted in host network before reaching OVS.  Does it has any impact on network policy
 enforcement? 
 
It is Ok from policy enforcement perspective, 
Antrea enforces network policy by allowing or disallowing initial connection packets (for
 instance, TCP sync), to go through and to establish connection tracking. Once a connection is established,
it relies on conntrack to admit or reject packets for that connection. 
This still holds true for re-entrance traffic, 
except that conntrack takes place not within OVS conntrack zone, 
but with host network's default conntrack zone.

It have some effect on statistics collection. If original reply traffic reaches 
 OVS bridge as in the encap traffic mode case, OVS
bridge knows any reply traffic dropped by OVS zone conntrack and recording them accordingly. With
re-entrance traffic, the reply traffic with original server Pod IPs do not reach OVS bridge, and 
any dropped traffic by host network conntrack are unknown to OVS/Antrea.  

## Additional Works
1) Smoother transition in/out of Antrea in policy mode, k8s deployment shall be easily 
scaled up and down after/before Antrea insertion to allow Pods be added added to Antrea after 
installation, and reconnect to old CNI topology after Antrea is uninstalled.
2) Reconciliation during mode change. Up to this point, encap is the pre-dominate mode. As more
traffic mode is used, it is reasonable for a cluster to change traffic encapsulation on the 
flight, Some components routeClient, ovsConfigClient(??) may not be ready for it. 

  

   
