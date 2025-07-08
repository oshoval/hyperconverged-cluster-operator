package operands

import (
	"errors"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	netattdefv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands/passt"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func NewConditionalNetworkAttachmentDefinitionHandler(Client client.Client, Scheme *runtime.Scheme, newCrFunc newNetworkAttachmentDefinitionFunc) Operand {
	return &conditionalHandler{
		operand: &genericOperand{
			Client: Client,
			Scheme: Scheme,
			crType: "NetworkAttachmentDefinition",
			hooks:  &networkAttachmentDefinitionHooks{newCrFunc: newCrFunc},
		},
		shouldDeploy: func(hc *hcov1beta1.HyperConverged) bool {
			value, ok := hc.Annotations[passt.DeployPasstNetworkBindingAnnotation]
			return ok && value == "true"
		},
		getCRWithName: func(hc *hcov1beta1.HyperConverged) client.Object {
			return passt.NewPasstBindingCNINetworkAttachmentDefinition(hc)
		},
	}
}

type newNetworkAttachmentDefinitionFunc func(hc *hcov1beta1.HyperConverged) *netattdefv1.NetworkAttachmentDefinition

type networkAttachmentDefinitionHooks struct {
	newCrFunc newNetworkAttachmentDefinitionFunc
}

func (h networkAttachmentDefinitionHooks) getFullCr(hc *hcov1beta1.HyperConverged) (client.Object, error) {
	return h.newCrFunc(hc), nil
}

func (networkAttachmentDefinitionHooks) getEmptyCr() client.Object {
	return &netattdefv1.NetworkAttachmentDefinition{}
}

func (networkAttachmentDefinitionHooks) justBeforeComplete(_ *common.HcoRequest) { /* no implementation */
}

func (networkAttachmentDefinitionHooks) updateCr(req *common.HcoRequest, Client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	return updateNetworkAttachmentDefinition(req, Client, exists, required)
}

func updateNetworkAttachmentDefinition(req *common.HcoRequest, Client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	networkAttachmentDefinition, ok1 := required.(*netattdefv1.NetworkAttachmentDefinition)
	found, ok2 := exists.(*netattdefv1.NetworkAttachmentDefinition)
	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to NetworkAttachmentDefinition")
	}
	if !hasNetworkAttachmentDefinitionRightFields(found, networkAttachmentDefinition) {
		if req.HCOTriggered {
			req.Logger.Info("Updating existing NetworkAttachmentDefinition Spec to new opinionated values")
		} else {
			req.Logger.Info("Reconciling an externally updated NetworkAttachmentDefinition's Spec to its opinionated values")
		}
		util.MergeLabels(&networkAttachmentDefinition.ObjectMeta, &found.ObjectMeta)
		networkAttachmentDefinition.Spec.DeepCopyInto(&found.Spec)
		err := Client.Update(req.Ctx, found)
		if err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}
	return false, false, nil
}

func hasNetworkAttachmentDefinitionRightFields(found *netattdefv1.NetworkAttachmentDefinition, required *netattdefv1.NetworkAttachmentDefinition) bool {
	return util.CompareLabels(required, found)
}
