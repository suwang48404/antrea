package fedmanager

import (
	"sync"
	"sync/atomic"
	"time"

	mock "github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/vmware-tanzu/antrea/federation/controllers/federation/clusterclient"
	mockclient "github.com/vmware-tanzu/antrea/federation/controllers/federation/clusterclient/testing"
	antreanetworking "github.com/vmware-tanzu/antrea/pkg/apis/crd/v1alpha1"
	antreacore "github.com/vmware-tanzu/antrea/pkg/apis/crd/v1alpha2"
)

var _ = Describe("Federation manager", func() {
	var (
		mockCtrl               *mock.Controller
		mockClusterClient      *mockclient.MockClusterClient
		fedManager             FederationManager
		fedID                  clusterclient.FederationID
		clusterID              clusterclient.ClusterID
		stopMgrChan            chan struct{}
		stopClientLeaderChan   chan struct{}
		stoppedClientLeader    bool
		stopClientResourceChan chan struct{}
		stoppedClientResource  bool
		wg                     *sync.WaitGroup
		canContinue            int32
	)

	AfterEach(func() {
		mockCtrl.Finish()
	})

	BeforeEach(func() {
		mockCtrl = mock.NewController(GinkgoT())
		mockClusterClient = mockclient.NewMockClusterClient(mockCtrl)
		fedID = "test-fed"
		clusterID = "test-cluster"
		fedManager = NewFederationManager(fedID, logf.NullLogger{})
		stopMgrChan = make(chan struct{})
		stopClientLeaderChan = make(chan struct{})
		stopClientResourceChan = make(chan struct{})
		stoppedClientLeader = false
		stoppedClientResource = false
		wg = &sync.WaitGroup{}
		canContinue = 1
	})

	bgStop := func() {
		go func() {
			defer wg.Done()
			for atomic.LoadInt32(&canContinue) == 0 {
				time.Sleep(time.Millisecond)
			}
			stopMgrChan <- struct{}{}
		}()
	}

	bgRecvChan := func(c <-chan struct{}, stopped *bool) {
		go func() {
			defer wg.Done()
			<-c
			*stopped = true
		}()
	}

	It("Start and Stop with no Client", func() {
		wg.Add(1)
		bgStop()
		err := fedManager.Start(stopMgrChan)
		Expect(err).ToNot(HaveOccurred())
	})

	It("start and stop with Client", func() {
		mockClusterClient.EXPECT().GetClusterID().AnyTimes().Return(clusterID)
		// Expectations when member cluster is added.
		mockClusterClient.EXPECT().StartLeaderElection(mock.Any()).Return(stopClientLeaderChan, nil)
		mockClusterClient.EXPECT().StartMonitorResources(mock.Any()).Return(stopClientResourceChan, nil)
		err := fedManager.AddClusterClient(mockClusterClient)
		Expect(err).ToNot(HaveOccurred())
		wg.Add(3)
		// Expectations when member cluster is removed
		bgRecvChan(stopClientResourceChan, &stoppedClientLeader)
		bgRecvChan(stopClientLeaderChan, &stoppedClientResource)
		// Stop fedManager after cluster is added.
		bgStop()
		err = fedManager.Start(stopMgrChan)
		Expect(err).ToNot(HaveOccurred())
		wg.Wait()
		Expect(stoppedClientResource).To(BeTrue())
		Expect(stoppedClientLeader).To(BeTrue())
	})

	It("Add and remove Client", func() {
		mockClusterClient.EXPECT().GetClusterID().AnyTimes().Return(clusterID)
		// Expectations when member cluster is added.
		mockClusterClient.EXPECT().StartLeaderElection(mock.Any()).Return(stopClientLeaderChan, nil)
		mockClusterClient.EXPECT().StartMonitorResources(mock.Any()).Return(stopClientResourceChan, nil)
		err := fedManager.AddClusterClient(mockClusterClient)
		Expect(err).ToNot(HaveOccurred())
		err = fedManager.RemoveClusterClient(mockClusterClient)
		Expect(err).ToNot(HaveOccurred())
		wg.Add(3)
		// Expectations when member cluster is removed
		bgRecvChan(stopClientResourceChan, &stoppedClientLeader)
		bgRecvChan(stopClientLeaderChan, &stoppedClientResource)
		// Stop fedManager after cluster is added amd then removed.
		bgStop()
		err = fedManager.Start(stopMgrChan)
		Expect(err).ToNot(HaveOccurred())
		wg.Wait()
		Expect(stoppedClientResource).To(BeTrue())
		Expect(stoppedClientLeader).To(BeTrue())
	})

	It("Remove elected leader", func() {
		mockClusterClient.EXPECT().GetClusterID().AnyTimes().Return(clusterID)
		// Expectations when member cluster is added.
		mockClusterClient.EXPECT().StartMonitorResources(mock.Any()).Return(stopClientResourceChan, nil)
		canContinue = 0
		mockClusterClient.EXPECT().StartLeaderElection(mock.Any()).Return(stopClientLeaderChan, nil).
			Do(func(queue workqueue.RateLimitingInterface) {
				// Notify fedManager is elected leader.
				queue.Add(clusterclient.LeaderEvent{
					IsLeader: true,
					Cluster:  clusterID,
				})
				// After election event the test allows fedManager to stop.
				atomic.StoreInt32(&canContinue, 1)
			})

		// Expectations when fedManager becomes the leader of cluster.
		// Purging 1) stale resources from other fed during leader elected.
		//         2) remove resources from this fed during client removal.
		mockClusterClient.EXPECT().List(mock.Any(), &antreacore.ExternalEntityList{}, mock.Any()).Return(nil).Times(2)
		mockClusterClient.EXPECT().List(mock.Any(), &v1.ServiceList{}, mock.Any()).Return(nil).Times(2)
		mockClusterClient.EXPECT().List(mock.Any(), &v1.EndpointsList{}, mock.Any()).Return(nil).Times(2)
		// Initial federated resources computation.
		mockClusterClient.EXPECT().List(mock.Any(), &v1.ServiceList{}, mock.Any()).Return(nil)
		mockClusterClient.EXPECT().List(mock.Any(), &antreanetworking.ClusterNetworkPolicyList{}, mock.Any()).Return(nil)
		mockClusterClient.EXPECT().List(mock.Any(), &antreanetworking.NetworkPolicyList{}, mock.Any()).Return(nil)
		// Account calls in 1. initializeFedNetworkPolicies; 2. initializeFedNetworkPolicies.
		mockClusterClient.EXPECT().List(mock.Any(), &antreacore.ExternalEntityList{}, mock.Any()).Return(nil).Times(2)
		mockClusterClient.EXPECT().List(mock.Any(), &v1.NamespaceList{}, mock.Any()).Return(nil)

		wg.Add(3)
		// Run in background, because test neets to wait on canContinue.
		go func() {
			defer wg.Done()
			defer GinkgoRecover()
			// Add cluster that in turn elects fedManager as leader.
			err := fedManager.AddClusterClient(mockClusterClient)
			Expect(err).ToNot(HaveOccurred())
			// Remove cluster only after it has elected fedManager as leader.
			for atomic.LoadInt32(&canContinue) == 0 {
				time.Sleep(time.Millisecond)
			}
			err = fedManager.RemoveClusterClient(mockClusterClient)
			Expect(err).ToNot(HaveOccurred())
			// Stop fedManager after cluster is added and elects fedManager as leader, and then removed.
			stopMgrChan <- struct{}{}

		}()
		// Expectations when member cluster is removed
		bgRecvChan(stopClientResourceChan, &stoppedClientLeader)
		bgRecvChan(stopClientLeaderChan, &stoppedClientResource)
		err := fedManager.Start(stopMgrChan)
		Expect(err).ToNot(HaveOccurred())
		wg.Wait()
		Expect(stoppedClientResource).To(BeTrue())
		Expect(stoppedClientLeader).To(BeTrue())
	})
})
