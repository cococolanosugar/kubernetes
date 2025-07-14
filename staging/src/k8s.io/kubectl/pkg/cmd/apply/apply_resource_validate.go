package apply

import (
	"fmt"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
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
	for _, info := range infoList {
		err = o.validateRBACResource(info)
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *ApplyOptions) validateRBACResource(info *resource.Info) error {
	obj := info.Object
	if obj == nil {
		return nil
	}

	gvk := obj.GetObjectKind().GroupVersionKind()
	if gvk.Group != "rbac.authorization.k8s.io" {
		return nil
	}
	if gvk.Kind != "Role" && gvk.Kind != "ClusterRole" {
		return nil
	}

	apiResourceLists, err := o.DiscoveryClient.ServerPreferredResources()
	if err != nil {
		return fmt.Errorf("failed to list api-resources: %v", err)
	}

	resourceMap := make(map[string]map[string]map[string]bool)
	for _, apiResourceList := range apiResourceLists {
		gv, err := schema.ParseGroupVersion(apiResourceList.GroupVersion)
		if err != nil {
			continue
		}
		group := gv.Group
		version := gv.Version
		if _, ok := resourceMap[group]; !ok {
			resourceMap[group] = make(map[string]map[string]bool)
		}
		if _, ok := resourceMap[group][version]; !ok {
			resourceMap[group][version] = make(map[string]bool)
		}
		for _, apiRes := range apiResourceList.APIResources {
			resourceMap[group][version][apiRes.Name] = true
			//resourceMap[group][version][apiRes.Kind] = true
		}
	}

	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil
	}

	rules, found, err := unstructured.NestedSlice(u.Object, "rules")
	if err != nil || !found {
		return nil
	}

	for _, rule := range rules {
		ruleMap, ok := rule.(map[string]interface{})
		if !ok {
			continue
		}
		resources, found, err := unstructured.NestedStringSlice(ruleMap, "resources")
		if err != nil || !found {
			continue
		}
		for _, resourceName := range resources {
			if resourceName == "*" {
				continue
			}

			resource := schema.GroupVersionResource{Resource: resourceName, Group: ""}
			groupVersionResource, err := o.Mapper.ResourceFor(schema.GroupVersionResource{Resource: resourceName, Group: ""})
			if err == nil {
				resource = groupVersionResource
			}

			found := false
			if _, exists := resourceMap[resource.Group]; exists {
				if _, exists := resourceMap[resource.Group][resource.Version]; exists {
					if _, exists := resourceMap[resource.Group][resource.Version][resourceName]; exists {
						found = true
					}
				}
			}
			if !found {
				return fmt.Errorf("resource group:%v, version:%v, name:%v not found in the cluster, please check 'kubectl api-resources'", resource.Group, resource.Version, resourceName)
			}
		}
	}

	return nil
}
