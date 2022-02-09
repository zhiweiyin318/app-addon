package main

import (
	"context"
	"embed"
	"os"

	"github.com/openshift/library-go/pkg/assets"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/agent"
	"open-cluster-management.io/addon-framework/pkg/utils"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
)

//go:embed manifests
//go:embed manifests/charts/application-manager
//go:embed manifests/charts/application-manager/templates/_helpers.tpl
var AppChartFS embed.FS

var permissionFiles = []string{
	"manifests/permission/clusterRole.yaml",
	"manifests/permission/rolebinding.yaml",
}

//go:embed manifests
//go:embed manifests/permission
var permissionFS embed.FS

const (
	AppChartDir = "manifests/charts/application-manager"
)

func newRegistrationOption(kubeConfig *rest.Config, addonName string) *agent.RegistrationOption {
	return &agent.RegistrationOption{
		CSRConfigurations: agent.KubeClientSignerConfigurations(addonName, addonName),
		CSRApproveCheck:   utils.DefaultCSRApprover(addonName),
		PermissionConfig: func(cluster *clusterv1.ManagedCluster, addon *addonapiv1alpha1.ManagedClusterAddOn) error {
			// update the permission of hub for each addon agent here if the addon needs to access the hub.
			// the permission will be given into a kubeConfig secret named <addon-Name>-hub-hubeconfig which
			// the deployment mount on the managed cluster.

			// suggest create clusterrole with addon controller together, only create rolebiding here.

			groups := agent.DefaultGroups(cluster.GetName(), addonName)
			config := struct {
				ManagedClusterName string
				Group              string
			}{
				ManagedClusterName: cluster.GetName(),
				Group:              groups[0], // "system:open-cluster-management:cluster:{{ .ManagedClusterName }}:addon:application-manager"
			}

			kubeclient, err := kubernetes.NewForConfig(kubeConfig)
			if err != nil {
				return err
			}

			for _, file := range permissionFiles {
				results := resourceapply.ApplyDirectly(context.Background(),
					resourceapply.NewKubeClientHolder(kubeclient),
					nil,
					resourceapply.NewResourceCache(),
					func(name string) ([]byte, error) {
						template, err := permissionFS.ReadFile(file)
						if err != nil {
							return nil, err
						}
						return assets.MustCreateAssetFromTemplate(name, template, config).Data, nil
					},
					file,
				)

				for _, result := range results {
					if result.Error != nil {
						return result.Error
					}
				}

			}

			return nil
		},
	}
}

// define all of fields in values.yaml here
type GlobalValues struct {
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,"`
	ImagePullSecret string            `json:"imagePullSecret"`
	// do not change these field names,
	// klusterlet-addon-controller will override these values according annotation since we have case that need to change these values.
	ImageOverrides map[string]string `json:"imageOverrides,"`
	NodeSelector   map[string]string `json:"nodeSelector,"`
	ProxyConfig    map[string]string `json:"proxyConfig,"`
}

type Values struct {
	// we have 3 builtin values that do not need to set.
	// clusterName, addonInstallNamespace , hubKubeConfigSecret
	// Note: hubKubeConfigSecret is not hubKubeconfigSecret, need to correct them in charts.
	// the helm chart builtin values only supports `Capabilities.KubeVersion`, `Release.Name`,`Release.Namespace`
	// need to use local-cluster name to check if the cluster is hub cluster.
	FullNameOverride string       `json:"fullnameOverride"`
	Global           GlobalValues `json:"global,omitempty"`
}

func getValues(cluster *clusterv1.ManagedCluster,
	addon *addonapiv1alpha1.ManagedClusterAddOn) (addonfactory.Values, error) {
	var imageName string
	// one option to get the image name from env
	//  all the images info already are in the application-chart-sub subscriptions spec
	if imageName = os.Getenv("ADDON_AGENT_IMAGE"); len(imageName) == 0 {
		imageName = "quay.io/open-cluster-management/multicluster_operators_subscription:latest"
	}
	jsonValues := Values{
		Global: GlobalValues{
			ImagePullPolicy: "IfNotPresent",
			// the default image pull secert is "open-cluster-management-image-pull-credentials",
			ImagePullSecret: "open-cluster-management-image-pull-credentials",
			ImageOverrides: map[string]string{
				// images can get from the cmd options or env
				"multicluster_operators_subscription": imageName,
			},
			NodeSelector: map[string]string{},
			ProxyConfig: map[string]string{
				"HTTP_PROXY":  "",
				"HTTPS_PROXY": "",
				"NO_PROXY":    "",
			},
		},
	}
	values, err := addonfactory.JsonStructToValues(jsonValues)
	if err != nil {
		return nil, err
	}
	return values, nil
}
