/*
 *  Copyright  (c) 2020 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package externalentity_test

import (
	"context"
	"reflect"
	"strings"
	"time"

	mock "github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	antreatypes "github.com/vmware-tanzu/antrea/pkg/apis/crd/v1alpha2"

	"github.com/vmware-tanzu/antrea/federation/controllers/config"

	networking "github.com/vmware-tanzu/antrea/federation/controllers/externalentity"

	_ "github.com/vmware-tanzu/antrea/federation/controllers/externalentity/externalentity_owners"
)

var _ = Describe("ExternalentityController", func() {

	var (
		// Test framework
		reconciler *networking.ExternalEntityReconciler

		// Test parameters
		expectedExternalEntities = make(map[string]*antreatypes.ExternalEntity)
		// expectedTags             = make(map[string]string)

		// Test tunables
		useInternalMethod      bool
		isOwnerEmptyEvent      bool
		generateOwnerEvent     bool
		expectExternalEntityOp bool
		externalEntityOpError  error
		expectWaiTime          time.Duration
	)

	BeforeEach(func() {
		commonInitTest()
		reconciler = &networking.ExternalEntityReconciler{
			Client: mockClient,
			Log:    logf.NullLogger{}, // suppress product logs, otherwise
			// Log: logf.Log,
			Scheme: scheme,
		}
		useInternalMethod = true
		isOwnerEmptyEvent = false
		generateOwnerEvent = true
		expectExternalEntityOp = true
		externalEntityOpError = nil
		expectWaiTime = time.Millisecond * 100

		err := reconciler.SetupWithManager(nil)
		Expect(err).ToNot(HaveOccurred())
		for name, owner := range networking.RegisteredExternalEntityOwners {
			Expect(owner.Ch).ToNot(BeNil())
			Expect(owner.Client).To(Equal(mockClient))
			Expect(owner.EntityOwner).ToNot(BeNil())
			Expect(owner.Log).ToNot(BeNil())
			Expect(reflect.TypeOf(owner.EntityOwner).Elem().Name()).To(Equal(name))
		}

		// Setup expected ExternalEntity from owner resources.
		for name, owner := range externalEntityOwners {
			ee := &antreatypes.ExternalEntity{}
			fetchKey := networking.GetObjectKeyFromOwner(owner)
			ee.Name = fetchKey.Name
			ee.Namespace = fetchKey.Namespace
			eps := make([]antreatypes.Endpoint, 0)
			for _, ip := range networkInterfaceIPAddresses {
				eps = append(eps, antreatypes.Endpoint{IP: ip})
			}
			ee.Spec.Endpoints = eps
			ee.Spec.Ports = owner.GetEndPointPort(nil)
			labels := make(map[string]string)
			accessor, _ := meta.Accessor(owner)
			labels[config.ExternalEntityLabelKeyKind] = networking.GetExternalEntityLabelKind(owner.EmbedType())
			labels[config.ExternalEntityLabelKeyName] = strings.ToLower(accessor.GetName())
			labels[config.ExternalEntityLabelKeyNamespace] = strings.ToLower(accessor.GetNamespace())
			for k, v := range owner.GetLabels(nil) {
				labels[k] = v
			}
			accessor, _ = meta.Accessor(owner.EmbedType())
			_ = controllerruntime.SetControllerReference(accessor, ee, scheme)
			for k, v := range owner.GetTags() {
				labels[k+config.ExternalEntityLabelKeyTagPostfix] = v
			}
			ee.Labels = labels
			expectedExternalEntities[name] = ee
		}
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Describe("processOwnerEvent", func() {
		var (
			externalEntityGetErr error
			failedUpdates        map[string]networking.ExternalEntityOwner
			retryChan            chan networking.ExternalEntityOwner
			isRetry              bool
		)

		tester := func(name string, op string) {
			finished := make(chan struct{}, 1)
			outstandingExpects := 0
			owner := externalEntityOwners[name]
			if isOwnerEmptyEvent {
				owner = emptyExternalEntityOwners[name]
			}

			fetchKey := networking.GetObjectKeyFromOwner(owner)
			orderedCalls := make([]*mock.Call, 0)

			// Determine owner ExternalEntity exits
			orderedCalls = append(orderedCalls,
				mockClient.EXPECT().Get(mock.Any(), fetchKey, mock.Any()).
					Return(externalEntityGetErr).
					Do(func(_ context.Context, _ client.ObjectKey, ee *antreatypes.ExternalEntity) {
						expectedExternalEntities[name].DeepCopyInto(ee)
						outstandingExpects--
						if outstandingExpects == 0 {
							finished <- struct{}{}
						}
					}))

			if expectExternalEntityOp {
				if op == "create" {
					orderedCalls = append(orderedCalls,
						mockClient.EXPECT().Create(mock.Any(), mock.Any()).
							Return(externalEntityOpError).
							Do(func(_ context.Context, obj runtime.Object) {
								Expect(obj).To(Equal(expectedExternalEntities[name]))
								outstandingExpects--
								if outstandingExpects == 0 {
									finished <- struct{}{}
								}
							}))
				} else if op == "patch" {
					orderedCalls = append(orderedCalls,
						mockClient.EXPECT().Patch(mock.Any(), mock.Any(), mock.Any()).
							Return(externalEntityOpError).
							Do(func(_ context.Context, patch runtime.Object, _ client.Patch) {
								Expect(patch).To(Equal(expectedExternalEntities[name]))
								outstandingExpects--
								if outstandingExpects == 0 {
									finished <- struct{}{}
								}
							}))

				} else if op == "delete" {
					orderedCalls = append(orderedCalls,
						mockClient.EXPECT().Delete(mock.Any(), mock.Any()).
							Return(externalEntityOpError).
							Do(func(_ context.Context, obj runtime.Object) {
								Expect(obj).To(Equal(expectedExternalEntities[name]))
								outstandingExpects--
								if outstandingExpects == 0 {
									finished <- struct{}{}
								}
							}))

				}
			}
			mock.InOrder(orderedCalls...)
			outstandingExpects = len(orderedCalls)

			// To handle NativeService dependents.
			if name == "NativeService" {
				mockClient.EXPECT().List(mock.Any(), mock.Any(), mock.Any()).Return(nil).MaxTimes(1)
			}

			if generateOwnerEvent {
				if useInternalMethod {
					networking.ProcessOwnerEvent(reconciler, owner, failedUpdates, retryChan, isRetry)
				} else {
					networking.RegisteredExternalEntityOwners[name+"OwnerOf"].Ch <- owner
				}
			}

			select {
			case <-finished:
				logf.Log.Info("All expectations are met")
			case <-time.After(expectWaiTime):
				logf.Log.Info("Test timed out")
			case <-retryChan:
				Fail("Received retry event")
			}
		}

		BeforeEach(func() {
			externalEntityGetErr = nil
			failedUpdates = make(map[string]networking.ExternalEntityOwner)
			retryChan = make(chan networking.ExternalEntityOwner)
			isRetry = false
		})

		Context("Should create when ExternalEntity is not found", func() {
			JustBeforeEach(func() {
				externalEntityGetErr = errors.NewNotFound(schema.GroupResource{}, "")
			})
			table.DescribeTable("When owner is",
				func(name string) {
					tester(name, "create")
				},
				table.Entry("EndpointsOwnerOf", "Endpoints"),
			)
		})

		Context("Should patch when ExternalEntity is found", func() {
			table.DescribeTable("When owner is",
				func(name string, nicIdx int) {
					tester(name, "patch")
				},
				table.Entry("EndpointsOwnerOf", "Endpoints"),
			)
		})

		Context("Should delete ExternalEntity when owner is empty", func() {
			JustBeforeEach(func() {
				isOwnerEmptyEvent = true
			})
			table.DescribeTable("When owner is",
				func(name string, nicIdx int) {
					tester(name, "delete")
				},
				table.Entry("EndpointsOwnerOf", "Endpoints"),
			)
		})

		Context("Should do nothing if ExternalEntity is not found and  owner is empty", func() {
			JustBeforeEach(func() {
				externalEntityGetErr = errors.NewNotFound(schema.GroupResource{}, "")
				isOwnerEmptyEvent = true
			})
			table.DescribeTable("When owner is",
				func(name string, nicIdx int) {
					tester(name, "")
				},
				table.Entry("EndpointsOwnerOf", "Endpoints"),
			)
		})

		Context("Handle error with retry", func() {
			JustBeforeEach(func() {
				externalEntityOpError = errors.NewBadRequest("dummy")
				useInternalMethod = false
			})

			Context("Should create when ExternalEntity is not found", func() {
				JustBeforeEach(func() {
					externalEntityGetErr = errors.NewNotFound(schema.GroupResource{}, "")
				})
				table.DescribeTable("When owner is",
					func(name string) {
						tester(name, "create")
						tester(name, "create")
						tester(name, "create")

						// a single successful retry cancels all previous failures
						externalEntityOpError = nil
						generateOwnerEvent = false
						expectWaiTime = (*networking.ExternalEntityOpRetryInterval) + time.Second*10
						tester(name, "create")
					},
					table.Entry("VirtualMachineOwnerOf", "VirtualMachine", 2),
				)
			})

			Context("Should patch when ExternalEntity is found", func() {
				table.DescribeTable("When owner is",
					func(name string) {
						tester(name, "patch")

						externalEntityOpError = nil
						generateOwnerEvent = false
						expectWaiTime = (*networking.ExternalEntityOpRetryInterval) + time.Second*10
						tester(name, "patch")
					},
					table.Entry("EndpointsOwnerOf", "Endpoints"),
				)
			})

			Context("Should delete ExternalEntity when owner is empty", func() {
				JustBeforeEach(func() {
					isOwnerEmptyEvent = true
				})
				table.DescribeTable("When owner is",
					func(name string) {
						tester(name, "delete")

						externalEntityOpError = nil
						generateOwnerEvent = false
						expectWaiTime = (*networking.ExternalEntityOpRetryInterval) + time.Second*10
						tester(name, "delete")
					},
					table.Entry("EndpointsOwnerOf", "Endpoints"),
				)
			})

		})
	})
})
