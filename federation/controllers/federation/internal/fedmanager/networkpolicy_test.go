package fedmanager

import (
	"context"

	mock "github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/vmware-tanzu/antrea/federation/controllers/config"
	"github.com/vmware-tanzu/antrea/federation/controllers/federation/clusterclient"
	mockclient "github.com/vmware-tanzu/antrea/federation/controllers/federation/clusterclient/testing"
	antreanetworking "github.com/vmware-tanzu/antrea/pkg/apis/crd/v1alpha1"
	antreacore "github.com/vmware-tanzu/antrea/pkg/apis/crd/v1alpha2"
)

var _ = Describe("Networkpolicy", func() {
	type Expectations struct {
		ee *antreacore.ExternalEntity
		ns *v1.Namespace
	}

	var (
		mockCtrl                 *mock.Controller
		mockClusterClient        *mockclient.MockClusterClient
		anotherMockClusterClient *mockclient.MockClusterClient
		fedManager               *federationManager
		fedID                    clusterclient.FederationID
		clusterID                clusterclient.ClusterID
		anotherClusterID         clusterclient.ClusterID
		nsLabelMatch             map[string]string
		eeLabelMatch             map[string]string
		testns                   string
		expectations             *Expectations
	)

	AfterEach(func() {
		mockCtrl.Finish()
	})

	BeforeEach(func() {
		mockCtrl = mock.NewController(GinkgoT())
		fedID = "test-fed"
		testns = "test-ns"
		nsLabelMatch = map[string]string{
			"test-key": "test-val",
		}
		eeLabelMatch = map[string]string{
			"test-key": "test-val",
		}

		expectations = &Expectations{
			ee: &antreacore.ExternalEntity{
				//				TypeMeta: metav1.TypeMeta{
				//					Kind: "ExternalEntity",
				//				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-ee",
					Namespace:   testns,
					Labels:      eeLabelMatch,
					Annotations: map[string]string{config.FederationIDAnnotation: string(fedID)},
				},
				Spec: antreacore.ExternalEntitySpec{
					ExternalNode: "test-external-node",
				},
			},
			ns: &v1.Namespace{
				TypeMeta: metav1.TypeMeta{Kind: "Namespace"},
				ObjectMeta: metav1.ObjectMeta{
					Name:   testns,
					Labels: nsLabelMatch,
				},
			},
		}

		// Setup fedManager already have one managed, and one un-managed cluster added.
		fedManager = NewFederationManager(fedID, logf.NullLogger{}).(*federationManager)
		mockCtrl = mock.NewController(GinkgoT())
		clusterID = "test-cluster"
		mockClusterClient = mockclient.NewMockClusterClient(mockCtrl)
		anotherClusterID = "another-test-cluster"
		anotherMockClusterClient = mockclient.NewMockClusterClient(mockCtrl)
		fedManager.clusters = map[clusterclient.ClusterID]*federationClient{
			clusterID: {
				isManagedBy:   true,
				ClusterClient: mockClusterClient,
			},
			anotherClusterID: {
				ClusterClient: anotherMockClusterClient,
				resourceStop:  make(chan struct{}, 1),
				leaderStop:    make(chan struct{}, 1),
			},
		}
		_ = fedManager.clusterResources.Add(
			clusterclient.Resource{
				Object:  expectations.ns,
				Cluster: clusterID,
			})
		mockClusterClient.EXPECT().GetClusterID().AnyTimes().Return(clusterID)
		anotherMockClusterClient.EXPECT().GetClusterID().AnyTimes().Return(anotherClusterID)
	})

	createANP := func(ingress, egress bool) clusterclient.Resource {

		anp := &antreanetworking.NetworkPolicy{
			TypeMeta: metav1.TypeMeta{
				Kind: "NetworkPolicy",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-np",
				Namespace: testns,
			},
		}
		getPeer := func() *antreanetworking.NetworkPolicyPeer {
			peer := &antreanetworking.NetworkPolicyPeer{}
			if nsLabelMatch != nil {
				peer.NamespaceSelector = &metav1.LabelSelector{
					MatchLabels: nsLabelMatch,
				}
			}
			if eeLabelMatch != nil {
				peer.ExternalEntitySelector = &metav1.LabelSelector{
					MatchLabels: eeLabelMatch,
				}
			}
			return peer
		}
		if ingress {
			anp.Spec.Ingress = []antreanetworking.Rule{
				{From: []antreanetworking.NetworkPolicyPeer{*getPeer()}},
			}
		}
		if egress {
			anp.Spec.Egress = []antreanetworking.Rule{
				{To: []antreanetworking.NetworkPolicyPeer{*getPeer()}},
			}
		}
		return clusterclient.Resource{
			Object:  anp,
			Cluster: clusterID,
		}
	}

	createACNP := func(ingress, egress bool) clusterclient.Resource {
		acnp := &antreanetworking.ClusterNetworkPolicy{
			TypeMeta: metav1.TypeMeta{
				Kind: "NetworkPolicy",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-np",
				Namespace: testns,
			},
		}
		getPeer := func() *antreanetworking.NetworkPolicyPeer {
			peer := &antreanetworking.NetworkPolicyPeer{}
			if nsLabelMatch != nil {
				peer.NamespaceSelector = &metav1.LabelSelector{
					MatchLabels: nsLabelMatch,
				}
			}
			if eeLabelMatch != nil {
				peer.ExternalEntitySelector = &metav1.LabelSelector{
					MatchLabels: eeLabelMatch,
				}
			}
			return peer
		}
		if ingress {
			acnp.Spec.Ingress = []antreanetworking.Rule{
				{From: []antreanetworking.NetworkPolicyPeer{*getPeer()}},
			}
		}
		if egress {
			acnp.Spec.Egress = []antreanetworking.Rule{
				{To: []antreanetworking.NetworkPolicyPeer{*getPeer()}},
			}
		}
		return clusterclient.Resource{
			Object:  acnp,
			Cluster: clusterID,
		}
	}

	eeFromDifferentNS := func() {
		expectations.ee.Namespace = "another-test-ns"
		_ = fedManager.clusterResources.Add(
			clusterclient.Resource{
				Object: &v1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "another-test-ns",
					},
					TypeMeta: metav1.TypeMeta{Kind: "Namespace"},
				},
				Cluster: clusterID,
			})
	}

	verify := func(
		isANP, // anp or acnp.
		ingress, // Specify selector on anp/acnp Ingress.
		egress, // Specify selector on anp/acnp Egress.
		matchEELabels, // EE labels should match anp/acnp selector.
		propagate bool, // Expect EE to be propagated.
		srrCluster *clusterclient.ClusterID, // EE's source cluster.
	) {
		np := createANP(ingress, egress)
		if !isANP {
			np = createACNP(ingress, egress)
		}
		err := fedManager.handleNetworkPolicyResource(np)
		Expect(err).ToNot(HaveOccurred())
		mockClusterClient.EXPECT().List(mock.Any(), mock.AssignableToTypeOf(&antreacore.ExternalEntityList{}), mock.Any()).Return(nil)
		fedManager.processNetworkPolicyChange()
		if propagate {
			mockClusterClient.EXPECT().Get(mock.Any(),
				client.ObjectKey{
					Namespace: expectations.ee.Namespace,
					Name:      expectations.ee.Name,
				},
				mock.AssignableToTypeOf(expectations.ee)).
				Return(errors.NewNotFound(schema.GroupResource{}, expectations.ee.Name))
			mockClusterClient.EXPECT().Create(mock.Any(), expectations.ee, mock.Any()).Return(nil)

		}
		in := expectations.ee.DeepCopy()
		in.Annotations = nil
		in.Kind = "ExternalEntity"
		if !matchEELabels {
			in.Labels = map[string]string{"another-test-key": "another-test-val"}
		}
		err = fedManager.handleExternalEntityResource(clusterclient.Resource{
			Object:  in,
			Cluster: *srrCluster,
		})
		Expect(err).ToNot(HaveOccurred())
	}

	Context("ExternalEntity Propagation", func() {
		Context("NetworkPolicy with valid Namespace and ExternalEntity Selectors", func() {
			table.DescribeTable("Antrea NetworkPolicy",
				func(ingress, egress, matchEELabels, propagate bool, srrCluster *clusterclient.ClusterID) {
					verify(true, ingress, egress, matchEELabels, propagate, srrCluster)
				},
				table.Entry("ExternalEntity is propagated if ingress Namespace and ExternalEntity Selector matches",
					true, false, true, true, &anotherClusterID),
				table.Entry("ExternalEntity is propagated if egress Namespace and ExternalEntity Selector matches",
					false, true, true, true, &anotherClusterID),
				table.Entry("ExternalEntity is not propagated if ExternalEntity Selector does not matches",
					false, true, false, false, &anotherClusterID),
				table.Entry("ExternalEntity is not propagated if ExternalEntity is from the same cluster",
					false, true, true, false, &clusterID),
			)

			table.DescribeTable("Antrea ClusterNetworkPolicy",
				func(ingress, egress, matchEELabels, propagate bool, srrCluster *clusterclient.ClusterID) {
					verify(false, ingress, egress, matchEELabels, propagate, srrCluster)
				},
				table.Entry("ExternalEntity is propagated if ingress Namespace and ExternalEntity Selector matches",
					true, false, true, true, &anotherClusterID),
				table.Entry("ExternalEntity is propagated if egress Namespace and ExternalEntity Selector matches",
					false, true, true, true, &anotherClusterID),
				table.Entry("ExternalEntity is not propagated if ExternalEntity Selector does not matches",
					false, true, false, false, &anotherClusterID),
				table.Entry("ExternalEntity is not propagated if ExternalEntity is from the same cluster",
					false, true, true, false, &clusterID),
			)
		})

		Context("NetworkPolicy with nil ExternalEntity Selectors", func() {
			JustBeforeEach(func() {
				eeLabelMatch = nil
			})
			table.DescribeTable("Antrea NetworkPolicy",
				func(ingress, egress, matchEELabels, propagate bool, srrCluster *clusterclient.ClusterID) {
					verify(true, ingress, egress, matchEELabels, propagate, srrCluster)
				},
				table.Entry("ExternalEntity is not propagated when NetworkPolicy has no ingress ExternalEntity Selector",
					true, false, true, false, &anotherClusterID),
				table.Entry("ExternalEntity is not propagated when NetworkPolicy has no egress ExternalEntity Selector",
					false, true, true, false, &anotherClusterID),
			)

			table.DescribeTable("Antrea Cluster NetworkPolicy",
				func(ingress, egress, matchEELabels, propagate bool, srrCluster *clusterclient.ClusterID) {
					verify(false, ingress, egress, matchEELabels, propagate, srrCluster)
				},
				table.Entry("ExternalEntity is not propagated when NetworkPolicy has no ingress ExternalEntity Selector",
					true, false, true, false, &anotherClusterID),
				table.Entry("ExternalEntity is not propagated when NetworkPolicy has no egress ExternalEntity Selector",
					false, true, true, false, &anotherClusterID),
			)
		})

		Context("NetworkPolicy with empty ExternalEntity Selectors", func() {
			JustBeforeEach(func() {
				eeLabelMatch = map[string]string{}
			})

			table.DescribeTable("Antrea NetworkPolicy",
				func(ingress, egress, matchEELabels, propagate bool, srrCluster *clusterclient.ClusterID) {
					verify(true, ingress, egress, matchEELabels, propagate, srrCluster)
				},
				table.Entry("ExternalEntity is propagated if NetworkPolicy has allow all ingress ExternalEntity Selector",
					true, false, true, true, &anotherClusterID),
				table.Entry("ExternalEntity is propagated if NetworkPolicy has allow all ingress ExternalEntity Selector",
					false, true, true, true, &anotherClusterID),
			)
			table.DescribeTable("Antrea Cluster NetworkPolicy",
				func(ingress, egress, matchEELabels, propagate bool, srrCluster *clusterclient.ClusterID) {
					verify(false, ingress, egress, matchEELabels, propagate, srrCluster)
				},
				table.Entry("ExternalEntity is propagated if NetworkPolicy has allow all ingress ExternalEntity Selector",
					true, false, true, true, &anotherClusterID),
				table.Entry("ExternalEntity is propagated if NetworkPolicy has allow all ingress ExternalEntity Selector",
					false, true, true, true, &anotherClusterID),
			)

		})

		Context("NetworkPolicy with nil Namespace Selectors", func() {
			JustBeforeEach(func() {
				nsLabelMatch = nil
			})

			table.DescribeTable("Antrea NetworkPolicy",
				func(ingress, egress, sameNS, propagate bool, srrCluster *clusterclient.ClusterID) {
					if !sameNS {
						eeFromDifferentNS()
					}
					verify(true, ingress, egress, true, propagate, srrCluster)
				},
				table.Entry("ExternalEntity is propagated if NetworkPolicy and ExternalEntity share the same Namespace (Ingress)",
					true, false, true, true, &anotherClusterID),
				table.Entry("ExternalEntity is propagated if NetworkPolicy and ExternalEntity share the same Namespace (Egress)",
					false, true, true, true, &anotherClusterID),
				table.Entry("ExternalEntity is not propagated if NetworkPolicy and ExternalEntity have different Namespaces (Ingress)",
					true, false, false, false, &anotherClusterID),
				table.Entry("ExternalEntity is propagated if NetworkPolicy and ExternalEntity have different Namespaces (Egress)",
					false, true, false, false, &anotherClusterID),
			)
			table.DescribeTable("Antrea Cluster NetworkPolicy",
				func(ingress, egress, sameNS, propagate bool, srrCluster *clusterclient.ClusterID) {
					if !sameNS {
						eeFromDifferentNS()
					}
					verify(false, ingress, egress, true, propagate, srrCluster)
				},
				table.Entry("ExternalEntity is propagated (Ingress)",
					true, false, false, true, &anotherClusterID),
				table.Entry("ExternalEntity is propagated (Egress)",
					false, true, true, true, &anotherClusterID),
			)
		})
	})

	Context("ExternalEntity created before NetworkPolicy", func() {
		It("ExternalEntity is propagated", func() {
			in := expectations.ee.DeepCopy()
			in.Annotations = nil
			in.Kind = "ExternalEntity"
			err := fedManager.processResource(clusterclient.Resource{
				Object:  in,
				Cluster: anotherClusterID,
			})
			Expect(err).ToNot(HaveOccurred())

			mockClusterClient.EXPECT().List(mock.Any(), mock.AssignableToTypeOf(&antreacore.ExternalEntityList{}), mock.Any()).Return(nil)
			mockClusterClient.EXPECT().Get(mock.Any(),
				client.ObjectKey{
					Namespace: expectations.ee.Namespace,
					Name:      expectations.ee.Name,
				},
				mock.AssignableToTypeOf(expectations.ee)).
				Return(errors.NewNotFound(schema.GroupResource{}, expectations.ee.Name))
			mockClusterClient.EXPECT().Create(mock.Any(), expectations.ee, mock.Any()).Return(nil)
			np := createANP(true, false)
			err = fedManager.processResource(np)
			Expect(err).ToNot(HaveOccurred())
			fedManager.processNetworkPolicyChange()
		})
	})

	Context("ExternalEntity created before matching Namespace labels", func() {
		JustBeforeEach(func() {
			_ = fedManager.clusterResources.Delete(
				clusterclient.Resource{
					Object:  expectations.ns,
					Cluster: clusterID,
				})
			expectations.ns.Labels = map[string]string{}
			_ = fedManager.clusterResources.Add(
				clusterclient.Resource{
					Object:  expectations.ns,
					Cluster: clusterID,
				})
		})
		It("ExternalEntity is propagated", func() {
			verify(true, false, true, true, true, &anotherClusterID)
			expectations.ns.Labels = nsLabelMatch
			err := fedManager.processResource(clusterclient.Resource{
				Object:  expectations.ns,
				Cluster: clusterID,
			})
			Expect(err).ToNot(HaveOccurred())
			mockClusterClient.EXPECT().List(mock.Any(), mock.AssignableToTypeOf(&antreacore.ExternalEntityList{}), mock.Any()).Return(nil)
			fedManager.processNetworkPolicyChange()
		})
	})

	Context("Client is removed after its source ExternalEntity is propagated", func() {
		JustBeforeEach(func() {
			_ = fedManager.clusterResources.Add(clusterclient.Resource{
				Object:  expectations.ee,
				Cluster: anotherClusterID,
			})
		})
		It("ExternalEntity delete is propagated", func() {
			mockClusterClient.EXPECT().List(mock.Any(), mock.AssignableToTypeOf(&antreacore.ExternalEntityList{}), mock.Any()).Return(nil).
				Do(func(_ context.Context, eeList *antreacore.ExternalEntityList, _ client.ListOption) {
					eeList.Items = []antreacore.ExternalEntity{*expectations.ee}
				})
			mockClusterClient.EXPECT().Delete(mock.Any(), expectations.ee, mock.Any()).Return(nil)
			err := fedManager.processClientEvent(clientEvent{isAdd: false, client: anotherMockClusterClient})
			Expect(err).ToNot(HaveOccurred())
			fedManager.processNetworkPolicyChange()
		})
	})

})
