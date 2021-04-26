package webhook_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	antreatypes "github.com/vmware-tanzu/antrea/pkg/apis/crd/v1alpha2"

	"github.com/vmware-tanzu/antrea/federation/webhook"
)

var _ = Describe("Cache", func() {

	var (
		externalEntity antreatypes.ExternalEntity
		cache          webhook.Cache
	)

	BeforeEach(func() {
		logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
		labels := map[string]string{
			"test-key":  "test-val",
			"test-key2": "test-val2",
			"test-key3": "test-val3",
		}
		externalEntity.Labels = labels
		externalEntity.Spec.Endpoints = []antreatypes.Endpoint{{}}
		externalEntity.Spec.Ports = []antreatypes.NamedPort{{Name: "test-port", Port: 222, Protocol: v1.ProtocolTCP}}
		cache = webhook.NewCache(logf.Log.WithName("web-cache-test"))
	})

	It("Validate equal", func() {
		cache.Update(&externalEntity)
		testObj := externalEntity.DeepCopy()
		err := cache.Validate(testObj)
		Expect(err).ToNot(HaveOccurred())
	})

	It("Validate equal with more labels", func() {
		cache.Update(&externalEntity)
		testObj := externalEntity.DeepCopy()
		testObj.Labels["add-key"] = "add-val"
		err := cache.Validate(testObj)
		Expect(err).ToNot(HaveOccurred())
		externalEntity.Labels = nil
		err = cache.Validate(testObj)
		Expect(err).ToNot(HaveOccurred())
	})

	It("Validate unequal with less labels", func() {
		cache.Update(&externalEntity)
		testObj := externalEntity.DeepCopy()
		delete(testObj.Labels, "test-key")
		err := cache.Validate(testObj)
		Expect(err).To(HaveOccurred())
		testObj.Labels = nil
		err = cache.Validate(testObj)
		Expect(err).To(HaveOccurred())
	})

	It("Validate unequal with spec mismatch", func() {
		cache.Update(&externalEntity)
		testObj := externalEntity.DeepCopy()
		testObj.Spec.Endpoints = append(testObj.Spec.Endpoints, antreatypes.Endpoint{IP: "1.1.1.1"})
		err := cache.Validate(testObj)
		Expect(err).To(HaveOccurred())
		testObj.Spec = antreatypes.ExternalEntitySpec{}
		err = cache.Validate(testObj)
		Expect(err).To(HaveOccurred())
	})
})
