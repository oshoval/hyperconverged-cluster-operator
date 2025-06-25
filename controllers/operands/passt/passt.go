package passt

import (
	"fmt"
	"os"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubevirtcorev1 "kubevirt.io/api/core/v1"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

const (
	DeployPasstNetworkBindingAnnotation = "deployPasstNetworkBinding"

	PasstBindingName = "passt"

	passtNetworkBindingNADName      = "primary-udn-kubevirt-binding"
	passtNetworkBindingNADNamespace = "default"
	NetworkAttachmentDefinition     = passtNetworkBindingNADNamespace + "/" + passtNetworkBindingNADName

	bindingComputeMemoryOverhead = "500Mi"
)

var (
	passtResourceMemory = resource.MustParse(bindingComputeMemoryOverhead)
	passtImage          string
	passtImageOnce      sync.Once
)

// CheckPasstImagesEnvExists checks if the passt image environment variable exists
func CheckPasstImagesEnvExists() error {
	if _, passtImageVarExists := os.LookupEnv(hcoutil.PasstImageEnvV); !passtImageVarExists {
		return fmt.Errorf("%s env var not found", hcoutil.PasstImageEnvV)
	}
	if _, passtCNIImageVarExists := os.LookupEnv(hcoutil.PasstCNIImageEnvV); !passtCNIImageVarExists {
		return fmt.Errorf("%s env var not found", hcoutil.PasstCNIImageEnvV)
	}
	return nil
}

// NetworkBinding creates an InterfaceBindingPlugin for passt network binding
func NetworkBinding(sidecarImage string) kubevirtcorev1.InterfaceBindingPlugin {
	return kubevirtcorev1.InterfaceBindingPlugin{
		NetworkAttachmentDefinition: NetworkAttachmentDefinition,
		SidecarImage:                sidecarImage,
		Migration:                   &kubevirtcorev1.InterfaceBindingMigration{},
		ComputeResourceOverhead: &kubevirtcorev1.ResourceRequirementsWithoutClaims{
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: passtResourceMemory,
			},
		},
	}
}

// NewPasstBindingCNISA creates a ServiceAccount for the passt binding CNI
func NewPasstBindingCNISA(hc *hcov1beta1.HyperConverged) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "passt-binding-cni",
			Namespace: hc.Namespace,
			Labels:    hcoutil.GetLabels(hcoutil.HyperConvergedName, hcoutil.AppComponentNetwork),
		},
	}
}

// ImageOnceFn gets the passt image from environment variable
func ImageOnceFn() string {
	passtImageOnce.Do(func() {
		passtImage = os.Getenv(hcoutil.PasstImageEnvV)
	})
	return passtImage
}
