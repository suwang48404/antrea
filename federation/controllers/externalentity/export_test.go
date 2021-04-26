/*
 *  Copyright  (c) 2020 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 */

package externalentity

var (
	GetObjectKeyFromOwner         = getObjectKeyFromOwner
	ExternalEntityOpRetryInterval = &externalEntityOpRetryInterval
)

func ProcessOwnerEvent(reconciler *ExternalEntityReconciler, owner ExternalEntityOwner,
	failedUpdates map[string]ExternalEntityOwner, retryChan chan<- ExternalEntityOwner, isRetry bool) {
	reconciler.processOwnerEvent(owner, failedUpdates, retryChan, isRetry)
}
