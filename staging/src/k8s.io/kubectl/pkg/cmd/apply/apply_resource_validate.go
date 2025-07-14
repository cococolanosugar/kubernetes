package apply

import (
	"k8s.io/kubectl/pkg/cmd/util"
)

// validateRBACResources validate RBAC Resources
func (o *ApplyOptions) validateRBACResources() error {
	if !o.StrictResourceCheck {
		return nil
	}
	infoList, err := o.GetObjects()
	if err != nil {
		return err
	}
	return util.ValidateRBACResources(infoList, o.DiscoveryClient, o.Mapper)
}
