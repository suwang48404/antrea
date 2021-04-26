/*******************************************************************************
 * Copyright 2020 VMWare, Inc.  All rights reserved. -- VMWare Confidential
 *******************************************************************************/

package federation

import (
	. "github.com/onsi/ginkgo"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
})
