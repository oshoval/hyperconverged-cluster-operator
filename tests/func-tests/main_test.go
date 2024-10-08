package tests_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

func TestTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "HyperConverged cluster E2E Test suite")
}

var _ = BeforeSuite(func(ctx context.Context) {
	cli := tests.GetK8sClientSet()

	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: tests.TestNamespace,
		},
	}

	opt := metav1.CreateOptions{}
	_, err := cli.CoreV1().Namespaces().Create(ctx, ns, opt)
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			panic(err)
		}
	}

	tests.BeforeEach(ctx)
})

var _ = AfterSuite(func(ctx context.Context) {
	cli := tests.GetK8sClientSet()
	err := cli.CoreV1().Namespaces().Delete(ctx, tests.TestNamespace, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		panic(err)
	}
})
