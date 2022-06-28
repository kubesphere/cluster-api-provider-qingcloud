package installer

import (
	"context"
	"fmt"
	"strings"

	"github.com/lithammer/dedent"
	"github.com/pkg/errors"
	kubeErr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/klog/v2"
)

type Installer interface {
	Install(resource string) error
	Uninstall(resource string) error
}

type InstallerConfig struct {
	CTX           context.Context
	KubeConfig    *rest.Config
	ClientSet     kubernetes.Interface
	DynamicClient dynamic.Interface
}

func (installer InstallerConfig) Install(resource string) error {
	// create  resources
	err := ResourceCreater(installer, resource)
	if err == nil {
		return err
	}
	return nil
}

func (installer InstallerConfig) Uninstall(resource string) error {
	// delete  resources
	err := ResourceRemover(installer, resource)
	if err != nil {
		return err
	}
	return nil
}

func ResourceCreater(installer InstallerConfig, resource string) (err error) {
	ctx := installer.CTX

	dynamicClient := installer.DynamicClient
	clientset := installer.ClientSet

	// Parse Resources,get the unstructured resource
	mapping, unstructuredResource, err := ParseResources(clientset, resource)
	if err != nil {
		return err
	}

	// create resource
	if err := CreateResource(ctx, dynamicClient, mapping, unstructuredResource); err != nil {
		if kubeErr.IsAlreadyExists(err) {
			return nil
		} else if kubeErr.IsInvalid(err) {
			return errors.Wrap(err, "Create resource failed, resource is invalid")
		} else {
			return err
		}
	}

	return nil
}

func CreateResource(ctx context.Context, dynamicClient dynamic.Interface, mapping *meta.RESTMapping, unstructuredResource *unstructured.Unstructured) error {
	namespace := unstructuredResource.GetNamespace()
	resp, err := dynamicClient.Resource(mapping.Resource).Namespace(namespace).Create(ctx, unstructuredResource, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	if resp == nil {
		klog.Errorf("%s\t%s\t create failed", unstructuredResource.GetKind(), unstructuredResource.GetName(), err)
		return errors.Wrap(err, fmt.Sprintf("create resource %s %s failed", unstructuredResource.GetKind(), unstructuredResource.GetName()))
	}
	klog.Infof("%s\t%s\t created\n", resp.GetKind(), resp.GetName())

	return nil
}

func ResourceRemover(installer InstallerConfig, resource string) (err error) {
	ctx := installer.CTX

	dynamicClient := installer.DynamicClient
	clientset := installer.ClientSet

	mapping, unstructuredResource, err := ParseResources(clientset, resource)
	if err != nil {
		return err
	}

	if err := RemoveResource(ctx, dynamicClient, mapping, unstructuredResource); err != nil {
		return err
	}

	return nil
}

func RemoveResource(ctx context.Context, dynamicClient dynamic.Interface, mapping *meta.RESTMapping, unstructuredResource *unstructured.Unstructured) error {
	name := unstructuredResource.GetName()
	namespace := unstructuredResource.GetNamespace()
	kind := unstructuredResource.GetKind()

	// delete resource by dynamic client
	if err := dynamicClient.Resource(mapping.Resource).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		if kubeErr.IsNotFound(err) {
			return nil
		} else {
			klog.Errorf("%s\t%s\t delete failed", kind, name, err)
			return errors.Wrap(err, "failed to remove resource")
		}
	}
	klog.Infof("%s\t%s\t%s\t deleted\n", kind, namespace, name)
	return nil
}

// ParseResources by parsing the resource, put the result into unstructuredResource and return it.
func ParseResources(clientset kubernetes.Interface, resource string) (*meta.RESTMapping, *unstructured.Unstructured, error) {
	var unstructuredResource unstructured.Unstructured
	r := dedent.Dedent(resource)
	// decode resource for convert the resource to unstructur.
	newreader := strings.NewReader(r)
	decode := yaml.NewYAMLOrJSONDecoder(newreader, 4096)
	// get resource kind and group
	disc := clientset.Discovery()
	restMapperRes, _ := restmapper.GetAPIGroupResources(disc)
	restMapper := restmapper.NewDiscoveryRESTMapper(restMapperRes)
	ext := runtime.RawExtension{}
	if err := decode.Decode(&ext); err != nil {
		return nil, &unstructuredResource, errors.Wrap(err, "decode error")
	}
	// get resource.Object
	obj, gvk, err := unstructured.UnstructuredJSONScheme.Decode(ext.Raw, nil, nil)
	if err != nil {
		return nil, &unstructuredResource, errors.Wrap(err, "failed to get resource object")
	}
	// identifies a preferred resource mapping
	mapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, &unstructuredResource, errors.Wrap(err, "failed to get resource mapping")
	}

	// convert the resource.Object into unstructured
	unstructuredResource.Object, err = runtime.DefaultUnstructuredConverter.ToUnstructured(obj)

	if err != nil {
		return nil, &unstructuredResource, errors.Wrap(err, "failed to converts an object into unstructured representation")
	}
	return mapping, &unstructuredResource, nil
}
