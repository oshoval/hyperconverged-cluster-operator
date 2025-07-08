package passt

import (
	"fmt"
	"maps"
	"os"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	netattdefv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	securityv1 "github.com/openshift/api/security/v1"

	kubevirtcorev1 "kubevirt.io/api/core/v1"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/nodeinfo"
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

// NewPasstBindingCNIDaemonSet creates a DaemonSet for the passt binding CNI
func NewPasstBindingCNIDaemonSet(hc *hcov1beta1.HyperConverged, isOpenShift bool) *appsv1.DaemonSet {
	maxUnavailable := intstr.FromString("10%")

	labels := hcoutil.GetLabels(hcoutil.HyperConvergedName, hcoutil.AppComponentNetwork)
	labels["tier"] = "node"

	infrastructureHighlyAvailable := nodeinfo.IsInfrastructureHighlyAvailable()
	affinity := getPodAntiAffinity(labels[hcoutil.AppLabelComponent], infrastructureHighlyAvailable)

	hostpath := "/opt/cni/bin"
	if isOpenShift {
		hostpath = "/var/lib/cni/bin"
	}

	daemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "passt-binding-cni",
			Namespace: hc.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"name": "passt-binding-cni",
				},
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: &maxUnavailable,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"name": "passt-binding-cni",
						"tier": "node",
						"app":  "passt-binding-cni",
					},
					Annotations: map[string]string{
						"description": "passt-binding-cni installs 'passt binding' CNI on cluster nodes",
					},
				},
				Spec: corev1.PodSpec{
					PriorityClassName:  "system-cluster-critical",
					ServiceAccountName: "passt-binding-cni",
					Containers: []corev1.Container{
						{
							Name:  "installer",
							Image: os.Getenv(hcoutil.PasstCNIImageEnvV),
							Command: []string{
								"/bin/sh",
								"-ce",
							},
							Args: []string{
								`ls -la "/cni/kubevirt-passt-binding"
cp -f "/cni/kubevirt-passt-binding" "/opt/cni/bin"
echo "passt binding CNI plugin installation complete..sleep infinity"
sleep 2147483647`,
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("15Mi"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: func() *bool { p := true; return &p }(),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "cnibin",
									MountPath: "/opt/cni/bin",
								},
							},
							ImagePullPolicy: corev1.PullIfNotPresent,
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "cnibin",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: hostpath,
								},
							},
						},
					},
				},
			},
		},
	}

	if hc.Spec.Infra.NodePlacement != nil {
		if hc.Spec.Infra.NodePlacement.NodeSelector != nil {
			daemonSet.Spec.Template.Spec.NodeSelector = maps.Clone(hc.Spec.Infra.NodePlacement.NodeSelector)
		} else {
			daemonSet.Spec.Template.Spec.NodeSelector = nil
		}

		if hc.Spec.Infra.NodePlacement.Affinity != nil {
			daemonSet.Spec.Template.Spec.Affinity = hc.Spec.Infra.NodePlacement.Affinity.DeepCopy()
		} else {
			daemonSet.Spec.Template.Spec.Affinity = affinity
		}

		if hc.Spec.Infra.NodePlacement.Tolerations != nil {
			daemonSet.Spec.Template.Spec.Tolerations = make([]corev1.Toleration, len(hc.Spec.Infra.NodePlacement.Tolerations))
			copy(daemonSet.Spec.Template.Spec.Tolerations, hc.Spec.Infra.NodePlacement.Tolerations)
		} else {
			daemonSet.Spec.Template.Spec.Tolerations = nil
		}
	} else {
		daemonSet.Spec.Template.Spec.NodeSelector = nil
		daemonSet.Spec.Template.Spec.Affinity = affinity
		daemonSet.Spec.Template.Spec.Tolerations = nil
	}

	return daemonSet
}

// NewPasstBindingCNINetworkAttachmentDefinition creates a NetworkAttachmentDefinition for the passt binding CNI
func NewPasstBindingCNINetworkAttachmentDefinition(hc *hcov1beta1.HyperConverged) *netattdefv1.NetworkAttachmentDefinition {
	return &netattdefv1.NetworkAttachmentDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:      passtNetworkBindingNADName,
			Namespace: "default",
			Labels:    hcoutil.GetLabels(hcoutil.HyperConvergedName, hcoutil.AppComponentNetwork),
		},
		Spec: netattdefv1.NetworkAttachmentDefinitionSpec{
			Config: `{
  "cniVersion": "1.0.0",
  "name": "primary-udn-kubevirt-binding",
  "plugins": [
    {
      "type": "kubevirt-passt-binding"
    }
  ]
}`,
		},
	}
}

// NewPasstBindingCNISecurityContextConstraints creates a SecurityContextConstraints for the passt binding CNI
func NewPasstBindingCNISecurityContextConstraints(hc *hcov1beta1.HyperConverged) *securityv1.SecurityContextConstraints {
	return &securityv1.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "passt-binding-cni",
			Labels: hcoutil.GetLabels(hcoutil.HyperConvergedName, hcoutil.AppComponentNetwork),
		},
		AllowPrivilegedContainer: true,
		AllowHostDirVolumePlugin: true,
		AllowHostIPC:             false,
		AllowHostNetwork:         false,
		AllowHostPID:             false,
		AllowHostPorts:           false,
		ReadOnlyRootFilesystem:   false,
		RunAsUser: securityv1.RunAsUserStrategyOptions{
			Type: securityv1.RunAsUserStrategyRunAsAny,
		},
		SELinuxContext: securityv1.SELinuxContextStrategyOptions{
			Type: securityv1.SELinuxStrategyRunAsAny,
		},
		Users: []string{
			fmt.Sprintf("system:serviceaccount:%s:passt-binding-cni", hc.Namespace),
		},
		Volumes: []securityv1.FSType{
			securityv1.FSTypeAll,
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

func getPodAntiAffinity(componentLabel string, infrastructureHighlyAvailable bool) *corev1.Affinity {
	if infrastructureHighlyAvailable {
		return &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
					{
						Weight: 90,
						PodAffinityTerm: corev1.PodAffinityTerm{
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      hcoutil.AppLabelComponent,
										Operator: metav1.LabelSelectorOpIn,
										Values:   []string{componentLabel},
									},
								},
							},
							TopologyKey: corev1.LabelHostname,
						},
					},
				},
			},
		}
	}

	return nil
}
