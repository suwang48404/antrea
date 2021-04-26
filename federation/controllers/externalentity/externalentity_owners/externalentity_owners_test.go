/*
 *  Copyright  (c) 2020 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package externalentityowners_test

import (
	"reflect"

	mock "github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"

	networking "github.com/vmware-tanzu/antrea/federation/controllers/externalentity"
	"github.com/vmware-tanzu/antrea/federation/testing"
	"github.com/vmware-tanzu/antrea/federation/testing/controllerruntimeclient"
	antreatypes "github.com/vmware-tanzu/antrea/pkg/apis/crd/v1alpha2"
)

var _ = Describe("ExternalentityOwners", func() {

	var (
		// Test framework
		mockCtrl   *mock.Controller
		mockclient *controllerruntimeclient.MockClient

		// Test parameters
		namespace = "test-externalentity-owners-namespace"

		// Test Tunables
		networkInterfaceIPAddresses = []string{"1.1.1.1", "2.2.2.2"}
		namedports                  = []antreatypes.NamedPort{
			{Name: "http", Protocol: v1.ProtocolTCP, Port: 80},
			{Name: "https", Protocol: v1.ProtocolTCP, Port: 443},
		}

		externalEntityOwners map[string]networking.ExternalEntityOwner
	)

	BeforeEach(func() {
		mockCtrl = mock.NewController(GinkgoT())
		mockclient = controllerruntimeclient.NewMockClient(mockCtrl)
		externalEntityOwners = testing.SetupExternalEntityOwners(networkInterfaceIPAddresses, namedports, namespace)
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	It("Initializes with correct parameters", func() {
		for name, ownerReconciler := range networking.RegisteredExternalEntityOwners {
			By("Owner: " + name)
			owner := ownerReconciler.EntityOwner
			Expect(reflect.TypeOf(owner).Elem().Name()).Should(Equal(name))
		}
	})

	getEndPointAddressesTester := func(name string, hasNic bool) {
		owner := externalEntityOwners[name]
		ips, err := owner.GetEndPointAddresses(mockclient)
		Expect(err).ToNot(HaveOccurred())
		// As Equal and []string{} == nil
		Expect(ips).To(ConsistOf(networkInterfaceIPAddresses))
		Expect(networkInterfaceIPAddresses).To(ConsistOf(ips))
	}

	getEndPointPortTester := func(name string, hasPort bool) {
		owner := externalEntityOwners[name]
		ports := owner.GetEndPointPort(mockclient)
		if hasPort {
			Expect(ports).To(Equal(namedports))
		} else {
			Expect(ports).To(BeNil())
		}
	}

	copyTester := func(name string) {
		owner := externalEntityOwners[name]
		dup := owner.Copy()
		Expect(dup).To(Equal(owner))
	}

	embedTypeTester := func(name string, expType interface{}) {
		owner := externalEntityOwners[name]
		embed := owner.EmbedType()
		Expect(reflect.TypeOf(embed).Elem()).To(Equal(reflect.TypeOf(expType).Elem()))
	}

	getTagsTester := func(name string, hasTag bool) {
		owner := externalEntityOwners[name]
		tags := owner.GetTags()
		if hasTag {
			Expect(tags).ToNot(BeNil())
		} else {
			Expect(tags).To(BeNil())
		}
	}

	getLabelsTester := func(name string, hasLabels bool) {
		// Empty.
	}

	Context("Owner has required information", func() {
		table.DescribeTable("GetEndPointAddresses",
			func(name string, hasNic bool) {
				getEndPointAddressesTester(name, hasNic)
			},
			table.Entry("Endpoints", "Endpoints", false))

		table.DescribeTable("GetEndPointPort",
			func(name string, hasPort bool) {
				getEndPointPortTester(name, hasPort)
			},
			table.Entry("Endpoints", "Endpoints", true))

		table.DescribeTable("GetTags",
			func(name string, hasTags bool) {
				getTagsTester(name, hasTags)
			},
			table.Entry("Endpoints", "Endpoints", false))

		table.DescribeTable("GetLabels",
			func(name string, hasLabels bool) {
				getLabelsTester(name, hasLabels)
			},
			table.Entry("Endpoints", "Endpoints", true))

		table.DescribeTable("Copy",
			func(name string) {
				copyTester(name)
			},
			table.Entry("Endpoints", "Endpoints"))

		table.DescribeTable("EmbedType",
			func(name string, expType interface{}) {
				embedTypeTester(name, expType)
			},
			table.Entry("Endpoints", "Endpoints", &v1.Endpoints{}))
	})
})
