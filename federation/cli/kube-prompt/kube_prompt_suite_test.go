package kube_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestKubePrompt(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "KubePrompt Suite")
}
