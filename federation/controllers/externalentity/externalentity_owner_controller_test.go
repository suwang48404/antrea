/*
 *  Copyright  (c) 2020 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package externalentity_test

import (
	"context"
	"time"

	mock "github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	networking "github.com/vmware-tanzu/antrea/federation/controllers/externalentity"
	"github.com/vmware-tanzu/antrea/federation/controllers/externalentity/externalentity_owners"
)

var _ = Describe("ExternalentityOwnerController", func() {

	BeforeEach(func() {
		commonInitTest()
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	table.DescribeTable("Reconciler",
		func(retError error) {
			owner := externalEntityOwners["Endpoints"].(*externalentityowners.EndpointsOwnerOf)
			ch := make(chan networking.ExternalEntityOwner)
			reconciler := &networking.ExternalEntityOwnerReconciler{
				Log:         logf.NullLogger{},
				EntityOwner: owner,
				Client:      mockClient,
				Ch:          ch,
			}
			request := ctrl.Request{}
			request.Namespace = owner.Namespace
			request.Name = owner.Name
			mockClient.EXPECT().Get(mock.Any(), request.NamespacedName, mock.Any()).Return(retError).
				Do(func(_ context.Context, _ types.NamespacedName, eps *v1.Endpoints) {
					if owner != nil {
						owner.DeepCopyInto(eps)
					}
				})
			go func() {
				defer GinkgoRecover()
				rlt, err := reconciler.Reconcile(request)
				if retError == nil || errors.IsNotFound(retError) {
					Expect(err).ToNot(HaveOccurred())
				} else {
					Expect(err).To(HaveOccurred())
				}
				Expect(rlt).To(Equal(ctrl.Result{}))

			}()
			select {
			case out := <-ch:
				Expect(out).To(Equal(owner))
			case <-time.After(time.Second):
				Expect(retError).ToNot(BeNil())
			}
		},
		table.Entry("Owner retrieve OK, generate event", nil),
		table.Entry("Owner retrieve not found, generate empty event", errors.NewNotFound(ctrl.GroupResource{}, "")),
		table.Entry("Owner retrieve error, not generate event", errors.NewBadRequest("")),
	)
})
