package util

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
)

// ValidateRBACResources 校验 RBAC 资源
func ValidateRBACResources(
	infos []*resource.Info,
	discoveryClient discovery.DiscoveryInterface,
	mapper meta.RESTMapper,
) error {
	resourceMap := make(map[string]map[string]map[string]bool)
	apiResourceLists, err := discoveryClient.ServerPreferredResources()
	if err != nil {
		return fmt.Errorf("failed to list api-resources: %v", err)
	}
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
		}
	}

	for _, info := range infos {
		obj := info.Object
		if obj == nil {
			continue
		}
		gvk := obj.GetObjectKind().GroupVersionKind()
		if gvk.Group != "rbac.authorization.k8s.io" {
			continue
		}
		if gvk.Kind != "Role" && gvk.Kind != "ClusterRole" {
			continue
		}
		u, ok := obj.(*unstructured.Unstructured)
		if !ok {
			continue
		}
		rules, found, err := unstructured.NestedSlice(u.Object, "rules")
		if err != nil || !found {
			continue
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
				groupVersionResource, err := mapper.ResourceFor(schema.GroupVersionResource{Resource: resourceName, Group: ""})
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
	}
	return nil
}
