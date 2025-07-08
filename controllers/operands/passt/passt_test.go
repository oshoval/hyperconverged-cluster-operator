package passt_test

import (
	"context"

	netattdefv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	securityv1 "github.com/openshift/api/security/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands/passt"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("Passt tests", func() {
	var (
		hco *hcov1beta1.HyperConverged
		req *common.HcoRequest
		cl  client.Client
	)

	BeforeEach(func() {
		hco = commontestutils.NewHco()
		hco.Annotations = make(map[string]string)
		req = commontestutils.NewReq(hco)
	})

	Context("test NewPasstBindingCNISA", func() {
		It("should have all default fields", func() {
			sa := passt.NewPasstBindingCNISA(hco)

			Expect(sa.Name).To(Equal("passt-binding-cni"))
			Expect(sa.Namespace).To(Equal(hco.Namespace))
			Expect(sa.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(sa.Labels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentNetwork)))
		})
	})

	Context("test NewPasstBindingCNIDaemonSet", func() {
		It("should have all default fields", func() {
			ds := passt.NewPasstBindingCNIDaemonSet(hco, false)

			Expect(ds.Name).To(Equal("passt-binding-cni"))
			Expect(ds.Namespace).To(Equal(hco.Namespace))

			Expect(ds.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(ds.Labels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentNetwork)))
			Expect(ds.Labels).To(HaveKeyWithValue("tier", "node"))
			Expect(ds.Labels).To(HaveKeyWithValue("app", "kubevirt-hyperconverged"))

			Expect(ds.Spec.Selector.MatchLabels).To(HaveKeyWithValue("name", "passt-binding-cni"))

			Expect(ds.Spec.Template.Labels).To(HaveKeyWithValue("name", "passt-binding-cni"))
			Expect(ds.Spec.Template.Labels).To(HaveKeyWithValue("tier", "node"))
			Expect(ds.Spec.Template.Labels).To(HaveKeyWithValue("app", "passt-binding-cni"))

			Expect(ds.Spec.Template.Annotations).To(HaveKeyWithValue("description", "passt-binding-cni installs 'passt binding' CNI on cluster nodes"))

			Expect(ds.Spec.Template.Spec.PriorityClassName).To(Equal("system-cluster-critical"))
			Expect(ds.Spec.Template.Spec.ServiceAccountName).To(Equal("passt-binding-cni"))

			Expect(ds.Spec.Template.Spec.Containers).To(HaveLen(1))
			container := ds.Spec.Template.Spec.Containers[0]
			Expect(container.Name).To(Equal("installer"))
			Expect(container.SecurityContext.Privileged).ToNot(BeNil())
			Expect(*container.SecurityContext.Privileged).To(BeTrue())

			Expect(ds.Spec.Template.Spec.Volumes).To(HaveLen(1))
			volume := ds.Spec.Template.Spec.Volumes[0]
			Expect(volume.Name).To(Equal("cnibin"))
			Expect(volume.HostPath).ToNot(BeNil())
			Expect(volume.HostPath.Path).To(Equal("/opt/cni/bin"))
		})
	})

	Context("test NewPasstBindingCNINetworkAttachmentDefinition", func() {
		It("should have all default fields", func() {
			nad := passt.NewPasstBindingCNINetworkAttachmentDefinition(hco)

			Expect(nad.Name).To(Equal("primary-udn-kubevirt-binding"))
			Expect(nad.Namespace).To(Equal("default"))

			Expect(nad.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(nad.Labels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentNetwork)))

			Expect(nad.Spec.Config).To(ContainSubstring(`"cniVersion": "1.0.0"`))
			Expect(nad.Spec.Config).To(ContainSubstring(`"name": "primary-udn-kubevirt-binding"`))
			Expect(nad.Spec.Config).To(ContainSubstring(`"type": "kubevirt-passt-binding"`))
		})
	})

	Context("ServiceAccount deployment", func() {
		It("should not create ServiceAccount if the annotation is not set", func() {
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := operands.NewServiceAccountHandler(cl, commontestutils.GetScheme(), passt.NewPasstBindingCNISA)

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundSAs := &corev1.ServiceAccountList{}
			Expect(cl.List(context.Background(), foundSAs)).To(Succeed())
			Expect(foundSAs.Items).To(BeEmpty())
		})

		It("should delete ServiceAccount if the deployPasstNetworkBinding annotation is not set", func() {
			sa := passt.NewPasstBindingCNISA(hco)
			cl = commontestutils.InitClient([]client.Object{hco, sa})

			handler := operands.NewServiceAccountHandler(cl, commontestutils.GetScheme(), passt.NewPasstBindingCNISA)

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal(sa.Name))
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeTrue())

			foundSAs := &corev1.ServiceAccountList{}
			Expect(cl.List(context.Background(), foundSAs)).To(Succeed())
			Expect(foundSAs.Items).To(BeEmpty())
		})

		It("should create ServiceAccount if the deployPasstNetworkBinding annotation is true", func() {
			hco.Annotations[passt.DeployPasstNetworkBindingAnnotation] = "true"
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := operands.NewServiceAccountHandler(cl, commontestutils.GetScheme(), passt.NewPasstBindingCNISA)

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal("passt-binding-cni"))
			Expect(res.Created).To(BeTrue())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundSA := &corev1.ServiceAccount{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: res.Name, Namespace: hco.Namespace}, foundSA)).To(Succeed())

			Expect(foundSA.Name).To(Equal("passt-binding-cni"))
			Expect(foundSA.Namespace).To(Equal(hco.Namespace))
		})
	})

	Context("DaemonSet deployment", func() {
		It("should not create DaemonSet if the annotation is not set", func() {
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := operands.NewConditionalDaemonSetHandler(cl, commontestutils.GetScheme(), false, passt.NewPasstBindingCNIDaemonSet)

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundDSs := &appsv1.DaemonSetList{}
			Expect(cl.List(context.Background(), foundDSs)).To(Succeed())
			Expect(foundDSs.Items).To(BeEmpty())
		})

		It("should delete DaemonSet if the deployPasstNetworkBinding annotation is false", func() {
			hco.Annotations[passt.DeployPasstNetworkBindingAnnotation] = "false"
			ds := passt.NewPasstBindingCNIDaemonSet(hco, false)
			cl = commontestutils.InitClient([]client.Object{hco, ds})

			handler := operands.NewConditionalDaemonSetHandler(cl, commontestutils.GetScheme(), false, passt.NewPasstBindingCNIDaemonSet)

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal(ds.Name))
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeTrue())

			foundDSs := &appsv1.DaemonSetList{}
			Expect(cl.List(context.Background(), foundDSs)).To(Succeed())
			Expect(foundDSs.Items).To(BeEmpty())
		})

		It("should create DaemonSet if the deployPasstNetworkBinding annotation is true", func() {
			hco.Annotations[passt.DeployPasstNetworkBindingAnnotation] = "true"
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := operands.NewConditionalDaemonSetHandler(cl, commontestutils.GetScheme(), true, passt.NewPasstBindingCNIDaemonSet)

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal("passt-binding-cni"))
			Expect(res.Created).To(BeTrue())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundDS := &appsv1.DaemonSet{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: res.Name, Namespace: hco.Namespace}, foundDS)).To(Succeed())

			Expect(foundDS.Name).To(Equal("passt-binding-cni"))
			Expect(foundDS.Namespace).To(Equal(hco.Namespace))

			// example of field set by the handler
			Expect(foundDS.Spec.Template.Spec.PriorityClassName).To(Equal("system-cluster-critical"))
		})
	})

	Context("NetworkAttachmentDefinition deployment", func() {
		It("should not create NetworkAttachmentDefinition if the annotation is not set", func() {
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := operands.NewConditionalNetworkAttachmentDefinitionHandler(cl, commontestutils.GetScheme(), passt.NewPasstBindingCNINetworkAttachmentDefinition)

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundNADs := &netattdefv1.NetworkAttachmentDefinitionList{}
			Expect(cl.List(context.Background(), foundNADs)).To(Succeed())
			Expect(foundNADs.Items).To(BeEmpty())
		})

		It("should delete NetworkAttachmentDefinition if the deployPasstNetworkBinding annotation is false", func() {
			hco.Annotations[passt.DeployPasstNetworkBindingAnnotation] = "false"
			nad := passt.NewPasstBindingCNINetworkAttachmentDefinition(hco)
			cl = commontestutils.InitClient([]client.Object{hco, nad})

			handler := operands.NewConditionalNetworkAttachmentDefinitionHandler(cl, commontestutils.GetScheme(), passt.NewPasstBindingCNINetworkAttachmentDefinition)

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal(nad.Name))
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeTrue())

			foundNADs := &netattdefv1.NetworkAttachmentDefinitionList{}
			Expect(cl.List(context.Background(), foundNADs)).To(Succeed())
			Expect(foundNADs.Items).To(BeEmpty())
		})

		It("should create NetworkAttachmentDefinition if the deployPasstNetworkBinding annotation is true", func() {
			hco.Annotations[passt.DeployPasstNetworkBindingAnnotation] = "true"
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := operands.NewConditionalNetworkAttachmentDefinitionHandler(cl, commontestutils.GetScheme(), passt.NewPasstBindingCNINetworkAttachmentDefinition)

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal("primary-udn-kubevirt-binding"))
			Expect(res.Created).To(BeTrue())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundNAD := &netattdefv1.NetworkAttachmentDefinition{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: res.Name, Namespace: "default"}, foundNAD)).To(Succeed())

			Expect(foundNAD.Name).To(Equal("primary-udn-kubevirt-binding"))
			Expect(foundNAD.Namespace).To(Equal("default"))

			Expect(foundNAD.Spec.Config).To(ContainSubstring(`"type": "kubevirt-passt-binding"`))
		})
	})

	Context("test NewPasstBindingCNISecurityContextConstraints", func() {
		It("should have all default fields", func() {
			scc := passt.NewPasstBindingCNISecurityContextConstraints(hco)

			Expect(scc.Name).To(Equal("passt-binding-cni"))
			Expect(scc.Labels).To(HaveKeyWithValue(hcoutil.AppLabel, hcoutil.HyperConvergedName))
			Expect(scc.Labels).To(HaveKeyWithValue(hcoutil.AppLabelComponent, string(hcoutil.AppComponentNetwork)))

			Expect(scc.AllowPrivilegedContainer).To(BeTrue())
			Expect(scc.AllowHostDirVolumePlugin).To(BeTrue())
			Expect(scc.AllowHostIPC).To(BeFalse())
			Expect(scc.AllowHostNetwork).To(BeFalse())
			Expect(scc.AllowHostPID).To(BeFalse())
			Expect(scc.AllowHostPorts).To(BeFalse())
			Expect(scc.ReadOnlyRootFilesystem).To(BeFalse())

			Expect(scc.RunAsUser.Type).To(Equal(securityv1.RunAsUserStrategyRunAsAny))
			Expect(scc.SELinuxContext.Type).To(Equal(securityv1.SELinuxStrategyRunAsAny))

			expectedUser := "system:serviceaccount:" + hco.Namespace + ":passt-binding-cni"
			Expect(scc.Users).To(ContainElement(expectedUser))

			Expect(scc.Volumes).To(ContainElement(securityv1.FSTypeAll))
		})
	})

	Context("SecurityContextConstraints deployment", func() {
		It("should not create SecurityContextConstraints if the annotation is not set", func() {
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := operands.NewConditionalSecurityContextConstraintsHandler(cl, commontestutils.GetScheme(), passt.NewPasstBindingCNISecurityContextConstraints)

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundSCCs := &securityv1.SecurityContextConstraintsList{}
			Expect(cl.List(context.Background(), foundSCCs)).To(Succeed())
			Expect(foundSCCs.Items).To(BeEmpty())
		})

		It("should delete SecurityContextConstraints if the deployPasstNetworkBinding annotation is false", func() {
			hco.Annotations[passt.DeployPasstNetworkBindingAnnotation] = "false"
			scc := passt.NewPasstBindingCNISecurityContextConstraints(hco)
			cl = commontestutils.InitClient([]client.Object{hco, scc})

			handler := operands.NewConditionalSecurityContextConstraintsHandler(cl, commontestutils.GetScheme(), passt.NewPasstBindingCNISecurityContextConstraints)

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal(scc.Name))
			Expect(res.Created).To(BeFalse())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeTrue())

			foundSCCs := &securityv1.SecurityContextConstraintsList{}
			Expect(cl.List(context.Background(), foundSCCs)).To(Succeed())
			Expect(foundSCCs.Items).To(BeEmpty())
		})

		It("should create SecurityContextConstraints if the deployPasstNetworkBinding annotation is true", func() {
			hco.Annotations[passt.DeployPasstNetworkBindingAnnotation] = "true"
			cl = commontestutils.InitClient([]client.Object{hco})

			handler := operands.NewConditionalSecurityContextConstraintsHandler(cl, commontestutils.GetScheme(), passt.NewPasstBindingCNISecurityContextConstraints)

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal("passt-binding-cni"))
			Expect(res.Created).To(BeTrue())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundSCC := &securityv1.SecurityContextConstraints{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: res.Name}, foundSCC)).To(Succeed())

			Expect(foundSCC.Name).To(Equal("passt-binding-cni"))
			Expect(foundSCC.AllowPrivilegedContainer).To(BeTrue())
		})
	})
})
