/*
 *  Copyright  (c) 2020 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package externalentity_test

import (
	"testing"
	"time"

	mock "github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	networking "github.com/vmware-tanzu/antrea/federation/controllers/externalentity"
	"github.com/vmware-tanzu/antrea/federation/controllers/externalentity/externalentity_owners"
	testing2 "github.com/vmware-tanzu/antrea/federation/testing"
	"github.com/vmware-tanzu/antrea/federation/testing/controllerruntimeclient"
	antreatypes "github.com/vmware-tanzu/antrea/pkg/apis/crd/v1alpha2"
)

var (
	mockCtrl   *mock.Controller
	mockClient *controllerruntimeclient.MockClient
	scheme     = runtime.NewScheme()
	namedports = []antreatypes.NamedPort{
		{Name: "http", Protocol: v1.ProtocolTCP, Port: 80},
		{Name: "https", Protocol: v1.ProtocolTCP, Port: 443},
	}
	networkInterfaceIPAddresses = []string{"1.1.1.1", "2.2.2.2"}
	testNamespace               = "test-namespace"
	emptyExternalEntityOwners   = map[string]networking.ExternalEntityOwner{
		"Endpoints": &externalentityowners.EndpointsOwnerOf{},
	}

	externalEntityOwners map[string]networking.ExternalEntityOwner
)

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	_ = clientgoscheme.AddToScheme(scheme)
	_ = antreatypes.AddToScheme(scheme)

	*networking.ExternalEntityOpRetryInterval = time.Millisecond * 500
})

func commonInitTest() {
	// common setup valid for all tests.
	mockCtrl = mock.NewController(GinkgoT())
	mockClient = controllerruntimeclient.NewMockClient(mockCtrl)
	externalEntityOwners = testing2.SetupExternalEntityOwners(networkInterfaceIPAddresses, namedports, testNamespace)
}

func TestNetworking(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Networking Suite")
}
