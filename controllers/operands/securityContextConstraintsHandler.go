package operands

import (
	"errors"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	securityv1 "github.com/openshift/api/security/v1"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/operands/passt"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

func NewConditionalSecurityContextConstraintsHandler(Client client.Client, Scheme *runtime.Scheme, newCrFunc newSecurityContextConstraintsFunc) Operand {
	return &conditionalHandler{
		operand: &genericOperand{
			Client: Client,
			Scheme: Scheme,
			crType: "SecurityContextConstraints",
			hooks:  &securityContextConstraintsHooks{newCrFunc: newCrFunc},
		},
		shouldDeploy: func(hc *hcov1beta1.HyperConverged) bool {
			value, ok := hc.Annotations[passt.DeployPasstNetworkBindingAnnotation]
			return ok && value == "true"
		},
		getCRWithName: func(hc *hcov1beta1.HyperConverged) client.Object {
			return passt.NewPasstBindingCNISecurityContextConstraints(hc)
		},
	}
}

type newSecurityContextConstraintsFunc func(hc *hcov1beta1.HyperConverged) *securityv1.SecurityContextConstraints

type securityContextConstraintsHooks struct {
	newCrFunc newSecurityContextConstraintsFunc
}

func (h securityContextConstraintsHooks) getFullCr(hc *hcov1beta1.HyperConverged) (client.Object, error) {
	return h.newCrFunc(hc), nil
}

func (securityContextConstraintsHooks) getEmptyCr() client.Object {
	return &securityv1.SecurityContextConstraints{}
}

func (securityContextConstraintsHooks) justBeforeComplete(_ *common.HcoRequest) { /* no implementation */
}

func (securityContextConstraintsHooks) updateCr(req *common.HcoRequest, Client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	return updateSecurityContextConstraints(req, Client, exists, required)
}

func updateSecurityContextConstraints(req *common.HcoRequest, Client client.Client, exists runtime.Object, required runtime.Object) (bool, bool, error) {
	securityContextConstraints, ok1 := required.(*securityv1.SecurityContextConstraints)
	found, ok2 := exists.(*securityv1.SecurityContextConstraints)
	if !ok1 || !ok2 {
		return false, false, errors.New("can't convert to SecurityContextConstraints")
	}
	if !hasSecurityContextConstraintsRightFields(found, securityContextConstraints) {
		if req.HCOTriggered {
			req.Logger.Info("Updating existing SecurityContextConstraints Spec to new opinionated values")
		} else {
			req.Logger.Info("Reconciling an externally updated SecurityContextConstraints's Spec to its opinionated values")
		}
		util.MergeLabels(&securityContextConstraints.ObjectMeta, &found.ObjectMeta)
		securityContextConstraints.DeepCopyInto(found)
		err := Client.Update(req.Ctx, found)
		if err != nil {
			return false, false, err
		}
		return true, !req.HCOTriggered, nil
	}
	return false, false, nil
}

func hasSecurityContextConstraintsRightFields(found *securityv1.SecurityContextConstraints, required *securityv1.SecurityContextConstraints) bool {
	return util.CompareLabels(required, found)
}
