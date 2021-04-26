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
)

var _ = Describe("Service", func() {

	type Expectations struct {
		clusterSvc *v1.Service
		clusterEps *v1.Endpoints
		fedSvc     *v1.Service
		fedEps     *v1.Endpoints
		ns         *v1.Namespace
	}
	var (
		mockCtrl                 *mock.Controller
		mockClusterClient        *mockclient.MockClusterClient
		anotherMockClusterClient *mockclient.MockClusterClient

		fedManager            *federationManager
		fedID                 clusterclient.FederationID
		clusterID             clusterclient.ClusterID
		anotherClusterID      clusterclient.ClusterID
		fedSvcName            string
		clusterSvcName        string
		testns                string
		nodeIP                string
		LBIP                  string
		epsIP                 string
		nodePort              int32
		svcPort               int32
		epsPort               int32
		expectations          *Expectations
		fedEpsBySvcType       map[v1.ServiceType]*v1.Endpoints
		clusterSvcByType      map[v1.ServiceType]*v1.Service
		fedOnDifferentCluster bool
		nodeName              string
	)

	AfterEach(func() {
		mockCtrl.Finish()
	})

	BeforeEach(func() {
		fedID = "test-fed"
		fedSvcName = "fed-svc-test"
		clusterSvcName = "test-svc"
		nodeName = "test-node"
		testns = "test-ns"

		svcPort = 443
		epsPort = 8443
		nodePort = 8888
		nodeIP = "20.20.20.20"
		LBIP = "30.30.30.30"
		epsIP = "1.1.1.1"
		expectations = &Expectations{
			clusterEps: &v1.Endpoints{
				TypeMeta: metav1.TypeMeta{Kind: "Endpoints"},
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterSvcName,
					Namespace: testns,
				},
				Subsets: []v1.EndpointSubset{{
					Addresses: []v1.EndpointAddress{{IP: epsIP}},
					Ports:     []v1.EndpointPort{{Port: epsPort}},
				}},
			},
			fedSvc: &v1.Service{
				TypeMeta: metav1.TypeMeta{Kind: "Service"},
				ObjectMeta: metav1.ObjectMeta{
					Name:        fedSvcName,
					Namespace:   testns,
					Annotations: map[string]string{config.FederationIDAnnotation: string(fedID)},
				},
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{{Port: svcPort}},
					Type:  v1.ServiceTypeClusterIP,
				},
				Status: v1.ServiceStatus{},
			},
			ns: &v1.Namespace{
				TypeMeta:   metav1.TypeMeta{Kind: "Namespace"},
				ObjectMeta: metav1.ObjectMeta{Name: testns},
			},
		}
		clusterSvcByType = map[v1.ServiceType]*v1.Service{
			v1.ServiceTypeClusterIP: {
				TypeMeta: metav1.TypeMeta{Kind: "Service"},
				ObjectMeta: metav1.ObjectMeta{
					Name:        clusterSvcName,
					Namespace:   testns,
					Annotations: map[string]string{config.UserFedServiceAnnotation: fedSvcName},
				},
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{{Port: svcPort}},
					Type:  v1.ServiceTypeClusterIP,
				},
				Status: v1.ServiceStatus{},
			},
			v1.ServiceTypeNodePort: {
				TypeMeta: metav1.TypeMeta{Kind: "Service"},
				ObjectMeta: metav1.ObjectMeta{
					Name:        clusterSvcName,
					Namespace:   testns,
					Annotations: map[string]string{config.UserFedServiceAnnotation: fedSvcName},
				},
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{{Port: svcPort, NodePort: nodePort}},
					Type:  v1.ServiceTypeNodePort,
				},
				Status: v1.ServiceStatus{},
			},
			v1.ServiceTypeLoadBalancer: {
				TypeMeta: metav1.TypeMeta{Kind: "Service"},
				ObjectMeta: metav1.ObjectMeta{
					Name:        clusterSvcName,
					Namespace:   testns,
					Annotations: map[string]string{config.UserFedServiceAnnotation: fedSvcName},
				},
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{{Port: svcPort}},
					Type:  v1.ServiceTypeLoadBalancer,
				},
				Status: v1.ServiceStatus{LoadBalancer: v1.LoadBalancerStatus{
					Ingress: []v1.LoadBalancerIngress{{IP: LBIP}}}},
			},
		}
		fedEpsBySvcType = map[v1.ServiceType]*v1.Endpoints{
			v1.ServiceTypeClusterIP: {
				TypeMeta: metav1.TypeMeta{Kind: "Endpoints"},
				ObjectMeta: metav1.ObjectMeta{
					Name:        fedSvcName,
					Namespace:   testns,
					Annotations: map[string]string{config.FederationIDAnnotation: string(fedID)},
				},
				Subsets: []v1.EndpointSubset{{
					Addresses: []v1.EndpointAddress{{IP: epsIP}},
					Ports:     []v1.EndpointPort{{Port: epsPort}},
				}},
			},
			v1.ServiceTypeNodePort: {
				TypeMeta: metav1.TypeMeta{Kind: "Endpoints"},
				ObjectMeta: metav1.ObjectMeta{
					Name:        fedSvcName,
					Namespace:   testns,
					Annotations: map[string]string{config.FederationIDAnnotation: string(fedID)},
				},
				Subsets: []v1.EndpointSubset{{
					Addresses: []v1.EndpointAddress{{IP: nodeIP, NodeName: &nodeName}},
					Ports:     []v1.EndpointPort{{Port: nodePort}},
				}},
			},
			v1.ServiceTypeLoadBalancer: {
				TypeMeta: metav1.TypeMeta{Kind: "Endpoints"},
				ObjectMeta: metav1.ObjectMeta{
					Name:        fedSvcName,
					Namespace:   testns,
					Annotations: map[string]string{config.FederationIDAnnotation: string(fedID)},
				},
				Subsets: []v1.EndpointSubset{{
					Addresses: []v1.EndpointAddress{{IP: LBIP}},
					Ports:     []v1.EndpointPort{{Port: svcPort}},
				}},
			},
		}
		fedOnDifferentCluster = false

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

	// Returns the clusterID of the Service.
	getSvcClusterID := func() clusterclient.ClusterID {
		if fedOnDifferentCluster {
			return anotherClusterID
		}
		return clusterID
	}

	// Returns the mock client of the Service.
	getSvcMockClient := func() *mockclient.MockClusterClient {
		if fedOnDifferentCluster {
			return anotherMockClusterClient
		}
		return mockClusterClient
	}

	verifyCache := func(exp *Expectations) {
		cache := fedManager.clusterResources.List()
		if exp.clusterSvc == nil || len(exp.clusterSvc.Kind) == 0 {
			exp.clusterSvc = nil
			exp.fedSvc = nil
			exp.clusterEps = nil
			exp.fedEps = nil
		} else if len(exp.clusterEps.Kind) == 0 {
			exp.fedEps = nil
			exp.clusterEps = &v1.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterSvcName,
					Namespace: testns,
				},
				TypeMeta: metav1.TypeMeta{
					Kind: "Endpoints",
				}}
		}
		rscs := make([]interface{}, 0)
		if exp.ns != nil {
			rscs = append(rscs, clusterclient.Resource{Cluster: clusterID, Object: exp.ns})
		}
		if exp.clusterSvc != nil {
			rscs = append(rscs, clusterclient.Resource{
				Object:  exp.clusterSvc,
				Cluster: getSvcClusterID(),
			})
		}
		if exp.clusterEps != nil {
			rscs = append(rscs, clusterclient.Resource{
				Object:  exp.clusterEps,
				Cluster: getSvcClusterID(),
			})
		}

		Expect(len(cache)).To(Equal(len(rscs)))
		Expect(cache).To(ContainElements(rscs...))
		cache = fedManager.fedResources.List()
		rscs = nil
		if exp.fedSvc != nil {
			rscs = append(rscs, clusterclient.Resource{
				Object: exp.fedSvc,
			})
		}
		if exp.fedEps != nil {
			rscs = append(rscs, clusterclient.Resource{
				Object: exp.fedEps,
			})
			if !fedOnDifferentCluster {
				rscs = append(rscs, clusterclient.Resource{
					Object:  fedEpsBySvcType[v1.ServiceTypeClusterIP],
					Cluster: clusterID,
				})
			}
		}
		Expect(len(cache)).To(Equal(len(rscs)))
		Expect(cache).To(ContainElements(rscs...))
	}

	populateCache := func(exp *Expectations) {
		_ = fedManager.clusterResources.Add(
			clusterclient.Resource{
				Object:  exp.clusterSvc.DeepCopy(),
				Cluster: getSvcClusterID(),
			})
		_ = fedManager.clusterResources.Add(
			clusterclient.Resource{
				Object:  exp.clusterEps.DeepCopy(),
				Cluster: getSvcClusterID(),
			})
		_ = fedManager.fedResources.Add(
			clusterclient.Resource{
				Object: exp.fedSvc.DeepCopy(),
			})
		_ = fedManager.fedResources.Add(
			clusterclient.Resource{
				Object: exp.fedEps.DeepCopy(),
			})
		if !fedOnDifferentCluster {
			_ = fedManager.fedResources.Add(
				clusterclient.Resource{
					Object:  exp.fedEps.DeepCopy(),
					Cluster: clusterID,
				})
		}
	}

	verifyAddService := func(exp *Expectations, generate, update bool) {
		if generate {
			// Expectations for generating federated Resources.
			getSvcMockClient().EXPECT().Get(mock.Any(),
				client.ObjectKey{Name: exp.clusterEps.Name, Namespace: exp.clusterEps.Namespace},
				mock.AssignableToTypeOf(&v1.Endpoints{})).Do(
				func(_ context.Context, _ client.ObjectKey, eps *v1.Endpoints) {
					exp.clusterEps.DeepCopyInto(eps)
				})
		}
		if update {
			// Expectations for updating to destination cluster.
			mockClusterClient.EXPECT().Get(mock.Any(),
				client.ObjectKey{Name: exp.fedSvc.Name, Namespace: exp.fedSvc.Namespace}, mock.AssignableToTypeOf(&v1.Service{})).
				Return(errors.NewNotFound(schema.GroupResource{}, exp.fedSvc.Name))
			mockClusterClient.EXPECT().Get(mock.Any(),
				client.ObjectKey{Name: exp.fedEps.Name, Namespace: exp.fedEps.Namespace}, mock.AssignableToTypeOf(&v1.Endpoints{})).
				Return(errors.NewNotFound(schema.GroupResource{}, exp.fedEps.Name))
			if exp.clusterSvc.Spec.Type == v1.ServiceTypeNodePort {
				getSvcMockClient().EXPECT().List(mock.Any(), mock.AssignableToTypeOf(&v1.NodeList{}), mock.Any()).Do(
					func(_ context.Context, nodeList *v1.NodeList, _ client.ListOption) {
						nodeList.Items = []v1.Node{{
							ObjectMeta: metav1.ObjectMeta{Name: nodeName},
							Status: v1.NodeStatus{
								Conditions: []v1.NodeCondition{{
									Type:   v1.NodeReady,
									Status: v1.ConditionTrue,
								}},
								Addresses: []v1.NodeAddress{{
									Type:    v1.NodeExternalIP,
									Address: nodeIP,
								}},
							},
						}}
					})
			}
			mockClusterClient.EXPECT().Create(mock.Any(), mock.AssignableToTypeOf(exp.fedSvc), mock.Any()).Do(
				func(_ context.Context, svc *v1.Service, _ client.CreateOption) {
					svc.Kind = exp.fedSvc.Kind
					Expect(svc).To(Equal(exp.fedSvc))
				})
			fedEps := exp.fedEps
			if !fedOnDifferentCluster {
				// As cluster Service and federated Service are in the same cluster,
				// therefore cluster Service Endpoints is also federated Service's Endpoints.
				fedEps = fedEpsBySvcType[v1.ServiceTypeClusterIP]
			}
			mockClusterClient.EXPECT().Create(mock.Any(), mock.AssignableToTypeOf(exp.fedEps), mock.Any()).Do(
				func(_ context.Context, eps *v1.Endpoints, _ client.CreateOption) {
					Expect(eps.Subsets).To(Equal(fedEps.Subsets))
				})
		}
	}

	verifyChangeServiceType := func(exp *Expectations, oldFedEps *v1.Endpoints) {
		mockClusterClient.EXPECT().Get(mock.Any(),
			client.ObjectKey{Name: exp.fedEps.Name, Namespace: exp.fedEps.Namespace}, mock.AssignableToTypeOf(&v1.Endpoints{})).Do(
			func(_ context.Context, _ client.ObjectKey, eps *v1.Endpoints) {
				oldFedEps.DeepCopyInto(eps)
			})
		if exp.clusterSvc.Spec.Type == v1.ServiceTypeNodePort {
			getSvcMockClient().EXPECT().List(mock.Any(), mock.AssignableToTypeOf(&v1.NodeList{}), mock.Any()).Do(
				func(_ context.Context, nodeList *v1.NodeList, _ client.ListOption) {
					nodeList.Items = []v1.Node{{
						ObjectMeta: metav1.ObjectMeta{Name: nodeName},
						Status: v1.NodeStatus{
							Conditions: []v1.NodeCondition{{
								Type:   v1.NodeReady,
								Status: v1.ConditionTrue,
							}},
							Addresses: []v1.NodeAddress{{
								Type:    v1.NodeExternalIP,
								Address: nodeIP,
							}},
						},
					}}
				})
		}
		fedEps := exp.fedEps
		if !fedOnDifferentCluster {
			// As cluster Service and federated Service are in the same cluster,
			// therefore cluster Service Endpoints is also federated Service's Endpoints.
			fedEps = fedEpsBySvcType[v1.ServiceTypeClusterIP]
		}
		mockClusterClient.EXPECT().Update(mock.Any(), mock.AssignableToTypeOf(exp.fedEps), mock.Any()).Do(
			func(_ context.Context, eps *v1.Endpoints, _ client.UpdateOption) {
				Expect(eps.Subsets).To(Equal(fedEps.Subsets))
			})
	}

	verifyUpdateEndpoints := func(exp *Expectations) {
		if exp.clusterSvc.Spec.Type == v1.ServiceTypeNodePort {
			getSvcMockClient().EXPECT().List(mock.Any(), mock.AssignableToTypeOf(&v1.NodeList{}), mock.Any()).Do(
				func(_ context.Context, nodeList *v1.NodeList, _ client.ListOption) {
					nodeList.Items = []v1.Node{{
						ObjectMeta: metav1.ObjectMeta{Name: nodeName},
						Status: v1.NodeStatus{
							Conditions: []v1.NodeCondition{{
								Type:   v1.NodeReady,
								Status: v1.ConditionTrue,
							}},
							Addresses: []v1.NodeAddress{{
								Type:    v1.NodeExternalIP,
								Address: nodeIP,
							}},
						},
					}}
				})
		}
		mockClusterClient.EXPECT().Get(mock.Any(),
			client.ObjectKey{Name: exp.fedEps.Name, Namespace: exp.fedEps.Namespace}, mock.AssignableToTypeOf(&v1.Endpoints{})).Do(
			func(_ context.Context, _ client.ObjectKey, eps *v1.Endpoints) {
				exp.fedEps.DeepCopyInto(eps)
			})
		fedEps := exp.fedEps
		if !fedOnDifferentCluster || exp.clusterSvc.Spec.Type == v1.ServiceTypeClusterIP {
			// As cluster Service and federated Service are in the same cluster,
			// therefore cluster Service Endpoints is also federated Service's Endpoints.
			fedEps = fedEpsBySvcType[v1.ServiceTypeClusterIP]
			fedEps.Subsets[0].Addresses[0].IP = "2.2.2.2"
		}
		mockClusterClient.EXPECT().Update(mock.Any(), mock.AssignableToTypeOf(exp.fedEps), mock.Any()).Do(
			func(_ context.Context, eps *v1.Endpoints, _ client.UpdateOption) {
				Expect(eps.Subsets).To(Equal(fedEps.Subsets))
			})
	}

	verifyDeleteService := func(exp *Expectations) {
		mockClusterClient.EXPECT().Delete(mock.Any(), mock.AssignableToTypeOf(exp.fedEps), mock.Any()).Do(
			func(_ context.Context, eps *v1.Endpoints, _ client.DeleteOption) {
				Expect(eps.ObjectMeta).To(Equal(exp.fedEps.ObjectMeta))
			})
		mockClusterClient.EXPECT().Delete(mock.Any(), mock.AssignableToTypeOf(exp.fedSvc), mock.Any()).Do(
			func(_ context.Context, svc *v1.Service, _ client.DeleteOption) {
				Expect(svc.ObjectMeta).To(Equal(exp.fedSvc.ObjectMeta))
			})
	}

	verifyDeleteEndpoints := func(exp *Expectations) {
		getSvcMockClient().EXPECT().Get(mock.Any(),
			client.ObjectKey{Name: exp.clusterEps.Name, Namespace: exp.clusterEps.Namespace}, mock.AssignableToTypeOf(exp.clusterEps)).
			Return(errors.NewNotFound(schema.GroupResource{}, exp.clusterEps.Name))
		mockClusterClient.EXPECT().Delete(mock.Any(), mock.AssignableToTypeOf(exp.fedEps), mock.Any()).Do(
			func(_ context.Context, eps *v1.Endpoints, _ client.DeleteOption) {
				Expect(eps.ObjectMeta).To(Equal(exp.fedEps.ObjectMeta))
			})
	}

	testAddService := func() {
		verifyAddService(expectations, true, true)
		err := fedManager.processResource(clusterclient.Resource{
			Object:  expectations.clusterSvc,
			Cluster: getSvcClusterID(),
		})
		Expect(err).ToNot(HaveOccurred())
		verifyCache(expectations)
	}

	testAddServiceNoNS := func() {
		verifyAddService(expectations, true, false)
		err := fedManager.processResource(clusterclient.Resource{
			Object:  expectations.clusterSvc,
			Cluster: getSvcClusterID(),
		})
		Expect(err).ToNot(HaveOccurred())
		verifyCache(expectations)
	}

	testAddNS := func() {
		populateCache(expectations)
		verifyAddService(expectations, false, true)
		err := fedManager.processResource(clusterclient.Resource{
			Object:  expectations.ns,
			Cluster: clusterID,
		})
		Expect(err).ToNot(HaveOccurred())
		verifyCache(expectations)
	}
	testChangeServiceType := func(newType v1.ServiceType) {
		populateCache(expectations)
		expectations.clusterSvc = clusterSvcByType[newType]
		oldFedEps := expectations.fedEps
		expectations.fedEps = fedEpsBySvcType[newType]
		verifyChangeServiceType(expectations, oldFedEps)
		err := fedManager.processResource(clusterclient.Resource{
			Object:  expectations.clusterSvc,
			Cluster: getSvcClusterID(),
		})
		Expect(err).ToNot(HaveOccurred())
		verifyCache(expectations)
	}

	testUpdateEndpoints := func() {
		populateCache(expectations)
		verifyUpdateEndpoints(expectations)
		expectations.clusterEps.Subsets[0].Addresses[0].IP = "2.2.2.2"
		err := fedManager.processResource(clusterclient.Resource{
			Object:  expectations.clusterEps,
			Cluster: getSvcClusterID(),
		})
		Expect(err).ToNot(HaveOccurred())
		verifyCache(expectations)
	}

	testDeleteService := func() {
		populateCache(expectations)
		verifyDeleteService(expectations)
		expectations.clusterSvc = &v1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: clusterSvcName, Namespace: testns},
		}
		expectations.clusterEps = &v1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{Name: clusterSvcName, Namespace: testns},
		}
		err := fedManager.processResource(clusterclient.Resource{
			Object:  expectations.clusterSvc,
			Cluster: getSvcClusterID(),
		})
		Expect(err).ToNot(HaveOccurred())
		err = fedManager.processResource(clusterclient.Resource{
			Object:  expectations.clusterEps,
			Cluster: getSvcClusterID(),
		})
		Expect(err).ToNot(HaveOccurred())
		verifyCache(expectations)
	}

	testDeleteEndpoints := func() {
		populateCache(expectations)
		verifyDeleteEndpoints(expectations)
		expectations.clusterEps = &v1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{Name: clusterSvcName, Namespace: testns},
		}
		err := fedManager.processResource(clusterclient.Resource{
			Object:  expectations.clusterEps,
			Cluster: getSvcClusterID(),
		})
		Expect(err).ToNot(HaveOccurred())
		verifyCache(expectations)
	}

	testRemoveClient := func() {
		populateCache(expectations)
		cp := *expectations
		verifyDeleteService(&cp)
		expectations.clusterEps = nil
		expectations.clusterSvc = nil
		expectations.fedEps = nil
		expectations.fedSvc = nil
		err := fedManager.processClientEvent(clientEvent{isAdd: false, client: anotherMockClusterClient})
		Expect(err).ToNot(HaveOccurred())
		verifyCache(expectations)
	}

	Context("Service from same cluster", func() {
		JustBeforeEach(func() {
			fedOnDifferentCluster = false
		})
		table.DescribeTable("Add new Service",
			func(svcType v1.ServiceType) {
				expectations.clusterSvc = clusterSvcByType[svcType]
				expectations.fedEps = fedEpsBySvcType[svcType]
				testAddService()
			},
			table.Entry("Type is ClusterIP", v1.ServiceTypeClusterIP),
			table.Entry("Type is NodePort", v1.ServiceTypeNodePort),
			table.Entry("Type is LoadBalancer", v1.ServiceTypeLoadBalancer))

		table.DescribeTable("Change Service type",
			func(svcType, newType v1.ServiceType) {
				expectations.clusterSvc = clusterSvcByType[svcType]
				expectations.fedEps = fedEpsBySvcType[svcType]
				testChangeServiceType(newType)
			},
			table.Entry("Type is ClusterIP, new Type is NodePort", v1.ServiceTypeClusterIP, v1.ServiceTypeNodePort),
			table.Entry("Type is NodePort, new Type is LoadBalancer", v1.ServiceTypeNodePort, v1.ServiceTypeLoadBalancer),
			table.Entry("Type is LoadBalancer, new type is ClusterIP", v1.ServiceTypeLoadBalancer, v1.ServiceTypeClusterIP))

		table.DescribeTable("Update Service's Endpoints",
			func(svcType v1.ServiceType) {
				expectations.clusterSvc = clusterSvcByType[svcType]
				expectations.fedEps = fedEpsBySvcType[svcType]
				testUpdateEndpoints()
			},
			table.Entry("Type is ClusterIP", v1.ServiceTypeClusterIP),
			table.Entry("Type is NodePort", v1.ServiceTypeNodePort),
			table.Entry("Type is LoadBalancer", v1.ServiceTypeLoadBalancer))

		table.DescribeTable("Delete Service",
			func(svcType v1.ServiceType) {
				expectations.clusterSvc = clusterSvcByType[svcType]
				expectations.fedEps = fedEpsBySvcType[svcType]
				testDeleteService()
			},
			table.Entry("Type is ClusterIP", v1.ServiceTypeClusterIP),
			table.Entry("Type is NodePort", v1.ServiceTypeNodePort),
			table.Entry("Type is LoadBalancer", v1.ServiceTypeLoadBalancer))

		table.DescribeTable("Delete Endpoints only",
			func(svcType v1.ServiceType) {
				expectations.clusterSvc = clusterSvcByType[svcType]
				expectations.fedEps = fedEpsBySvcType[svcType]
				testDeleteEndpoints()
			},
			table.Entry("Type is ClusterIP", v1.ServiceTypeClusterIP),
			table.Entry("Type is NodePort", v1.ServiceTypeNodePort),
			table.Entry("Type is LoadBalancer", v1.ServiceTypeLoadBalancer))
	})

	Context("Service from different cluster", func() {
		JustBeforeEach(func() {
			fedOnDifferentCluster = true
		})
		table.DescribeTable("Add new Service",
			func(svcType v1.ServiceType) {
				expectations.clusterSvc = clusterSvcByType[svcType]
				expectations.fedEps = fedEpsBySvcType[svcType]
				testAddService()
			},
			table.Entry("Type is ClusterIP", v1.ServiceTypeClusterIP),
			table.Entry("Type is NodePort", v1.ServiceTypeNodePort),
			table.Entry("Type is LoadBalancer", v1.ServiceTypeLoadBalancer))

		table.DescribeTable("Change Service type",
			func(svcType, newType v1.ServiceType) {
				expectations.clusterSvc = clusterSvcByType[svcType]
				expectations.fedEps = fedEpsBySvcType[svcType]
				testChangeServiceType(newType)
			},
			table.Entry("Type is ClusterIP, new Type is NodePort", v1.ServiceTypeClusterIP, v1.ServiceTypeNodePort),
			table.Entry("Type is NodePort, new Type is LoadBalancer", v1.ServiceTypeNodePort, v1.ServiceTypeLoadBalancer),
			table.Entry("Type is LoadBalancer, new type is ClusterIP", v1.ServiceTypeLoadBalancer, v1.ServiceTypeClusterIP))

		table.DescribeTable("Update Service's Endpoints",
			func(svcType v1.ServiceType) {
				expectations.clusterSvc = clusterSvcByType[svcType]
				expectations.fedEps = fedEpsBySvcType[svcType]
				testUpdateEndpoints()
			},
			table.Entry("Type is ClusterIP", v1.ServiceTypeClusterIP),
			table.Entry("Type is NodePort", v1.ServiceTypeNodePort),
			table.Entry("Type is LoadBalancer", v1.ServiceTypeLoadBalancer))

		table.DescribeTable("Delete Service",
			func(svcType v1.ServiceType) {
				expectations.clusterSvc = clusterSvcByType[svcType]
				expectations.fedEps = fedEpsBySvcType[svcType]
				testDeleteService()
			},
			table.Entry("Type is ClusterIP", v1.ServiceTypeClusterIP),
			table.Entry("Type is NodePort", v1.ServiceTypeNodePort),
			table.Entry("Type is LoadBalancer", v1.ServiceTypeLoadBalancer))

		table.DescribeTable("Delete Endpoints only",
			func(svcType v1.ServiceType) {
				expectations.clusterSvc = clusterSvcByType[svcType]
				expectations.fedEps = fedEpsBySvcType[svcType]
				testDeleteEndpoints()
			},
			table.Entry("Type is ClusterIP", v1.ServiceTypeClusterIP),
			table.Entry("Type is NodePort", v1.ServiceTypeNodePort),
			table.Entry("Type is LoadBalancer", v1.ServiceTypeLoadBalancer))

		Context("Namespace is not present in Cluster when generate federated Service", func() {
			It("Federated Resources not propagated", func() {
				_ = fedManager.clusterResources.Delete(
					clusterclient.Resource{
						Object:  expectations.ns,
						Cluster: clusterID,
					})
				expectations.ns = nil
				expectations.clusterSvc = clusterSvcByType[v1.ServiceTypeClusterIP]
				expectations.fedEps = fedEpsBySvcType[v1.ServiceTypeClusterIP]
				testAddServiceNoNS()
			})
		})

		Context("Namespace is added after generating federated Service", func() {
			It("Federated Resources are propagated", func() {
				expectations.clusterSvc = clusterSvcByType[v1.ServiceTypeClusterIP]
				expectations.fedEps = fedEpsBySvcType[v1.ServiceTypeClusterIP]
				_ = fedManager.clusterResources.Delete(
					clusterclient.Resource{
						Object:  expectations.ns,
						Cluster: clusterID,
					})
				testAddNS()
			})

		})

		Context("Cluster is removed when a federated Service contains its local Service", func() {
			It("Federated Resources are removed", func() {
				expectations.clusterSvc = clusterSvcByType[v1.ServiceTypeClusterIP]
				expectations.fedEps = fedEpsBySvcType[v1.ServiceTypeClusterIP]
				testRemoveClient()
			})
		})

		It("Service initialization", func() {
			exp := expectations
			exp.clusterSvc = clusterSvcByType[v1.ServiceTypeClusterIP]
			exp.fedEps = fedEpsBySvcType[v1.ServiceTypeClusterIP]
			mockClusterClient.EXPECT().List(mock.Any(), mock.AssignableToTypeOf(&v1.ServiceList{}), mock.Any()).Return(nil)
			getSvcMockClient().EXPECT().List(mock.Any(), mock.AssignableToTypeOf(&v1.ServiceList{}), mock.Any()).
				Do(func(_ context.Context, svcList *v1.ServiceList, _ client.ListOption) {
					svcList.Items = []v1.Service{*exp.clusterSvc}
				})
			/*
				getSvcMockClient().EXPECT().Get(mock.Any(),
					client.ObjectKey{Name: exp.clusterEps.Name, Namespace: exp.clusterEps.Namespace},
					mock.AssignableToTypeOf(&v1.Endpoints{})).Do(
					func(_ context.Context, _ client.ObjectKey, eps *v1.Endpoints) {
						exp.clusterEps.DeepCopyInto(eps)
					})
				mockClusterClient.EXPECT().Get(mock.Any(),
					client.ObjectKey{Name: exp.fedSvc.Name, Namespace: exp.fedSvc.Namespace}, mock.AssignableToTypeOf(&v1.Service{})).
					Return(errors.NewNotFound(schema.GroupResource{}, exp.fedSvc.Name))
				mockClusterClient.EXPECT().Create(mock.Any(), mock.AssignableToTypeOf(exp.fedSvc), mock.Any()).Do(
					func(_ context.Context, svc *v1.Service, _ client.CreateOption) {
						svc.Kind = exp.fedSvc.Kind
						Expect(svc).To(Equal(exp.fedSvc))
					})
				mockClusterClient.EXPECT().Get(mock.Any(),
					client.ObjectKey{Name: exp.fedSvc.Name, Namespace: exp.fedSvc.Namespace}, mock.AssignableToTypeOf(&v1.Endpoints{})).
					Return(errors.NewNotFound(schema.GroupResource{}, exp.fedSvc.Name))
				mockClusterClient.EXPECT().Create(mock.Any(), mock.AssignableToTypeOf(exp.fedEps), mock.Any()).Do(
					func(_ context.Context, eps *v1.Endpoints, _ client.CreateOption) {
						Expect(eps.Subsets).To(Equal(exp.fedEps.Subsets))
					})
			*/
			verifyAddService(exp, true, true)
			err := fedManager.initializeFedServices()
			Expect(err).ToNot(HaveOccurred())
			verifyCache(expectations)
		})
	})
})
