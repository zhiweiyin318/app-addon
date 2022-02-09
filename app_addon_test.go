package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/agent"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/yaml"
)

func newAgentAddon(t *testing.T) (agent.AgentAddon, error) {
	appRegistrationOption := newRegistrationOption(nil, AppAddonName)
	agentAddon, err := addonfactory.NewAgentAddonFactory(AppAddonName, AppChartFS, AppChartDir).
		WithGetValuesFuncs(getValues, addonfactory.GetValuesFromAddonAnnotation).
		WithAgentRegistrationOption(appRegistrationOption).
		BuildHelmAgentAddon()
	if err != nil {
		t.Errorf("failed to build agentAddon")
		return agentAddon, err
	}
	return agentAddon, nil
}
func newManagedCluster(clusterName string) *clusterv1.ManagedCluster {
	return &clusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterName,
		},
		Spec:   clusterv1.ManagedClusterSpec{},
		Status: clusterv1.ManagedClusterStatus{Version: clusterv1.ManagedClusterVersion{Kubernetes: "1.10.1"}},
	}
}
func newManagedClusterAddon(clusterName, values string) *addonapiv1alpha1.ManagedClusterAddOn {
	return &addonapiv1alpha1.ManagedClusterAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AppAddonName,
			Namespace: clusterName,
			Annotations: map[string]string{
				"addon.open-cluster-management.io/values": values,
			},
		},
		Spec: addonapiv1alpha1.ManagedClusterAddOnSpec{
			InstallNamespace: "open-cluster-management-agent-addon",
		},
	}
}

func TestAddonAgentManifests(t *testing.T) {
	cases := []struct {
		name             string
		annotationValues string
		clusterName      string
	}{
		{
			name:             "case1_local-cluster",
			annotationValues: `{"global":{"nodeSelector":{"node-role.kubernetes.io/infra":""},"imageOverrides":{"multicluster_operators_subscription":"quay.io/test:test"}}}`,
			clusterName:      "local-cluster",
		},
		{
			name:             "case2_has_proxy",
			annotationValues: `{"global":{"proxyConfig":{"HTTPS_PROXY":"2.2.2.2","HTTP_PROXY":"1.1.1.1","NO_PROXY":"3.3.3.3"}}}`,
			clusterName:      "local-cluster",
		},
		{
			name:        "case3_no_values",
			clusterName: "cluster2",
		},
	}

	agentAddon, err := newAgentAddon(t)
	if err != nil {
		t.Fatalf("failed to new agentAddon %v", err)
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cluster := newManagedCluster(c.clusterName)
			addon := newManagedClusterAddon(c.clusterName, c.annotationValues)
			objects, err := agentAddon.Manifests(cluster, addon)
			if err != nil {
				t.Fatalf("failed to get manifests %v", err)
			}
			output(t, c.name, objects...)
		})
	}

}

func output(t *testing.T, name string, objects ...runtime.Object) {
	tmpDir, err := os.MkdirTemp("./", name)
	if err != nil {
		t.Fatalf("failed to create temp %v", err)
	}

	for i, o := range objects {
		data, err := yaml.Marshal(o)
		if err != nil {
			t.Fatalf("failed yaml marshal %v", err)
		}

		err = ioutil.WriteFile(fmt.Sprintf("%v/%v-%v.yaml", tmpDir, i, o.GetObjectKind().GroupVersionKind().Kind), data, 0644)
		if err != nil {
			t.Fatalf("failed to Marshal object.%v", err)
		}

	}
}
