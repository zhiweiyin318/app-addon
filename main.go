package main

import (
	"context"
	"math/rand"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/addonmanager"
)

const (
	AppAddonName = "application-manager"
)

func main() {
	rand.Seed(time.Now().UTC().UnixNano())

}

func runController(ctx context.Context, kubeConfig *rest.Config) error {
	var mgr, err = addonmanager.New(kubeConfig)
	if err != nil {
		klog.Errorf("failed to new addon manager %v", err)
		return err
	}

	appRegistrationOption := newRegistrationOption(kubeConfig, AppAddonName)

	// should add addonfactory.GetValuesFromAddonAnnotation for the WithGetValuesFuncs
	// klustelet-adon-controller will override images, proxy env by annotations .
	// you can define several GetValuesFunc, the values got from the big index Func will override the one from small index Func.
	// getUserValues overrides getValues,  addonfactory.GetValuesFromAddonAnnotation overrides getValues and getUserValues for example.
	certAgentAddon, err := addonfactory.NewAgentAddonFactory(AppAddonName, AppChartFS, AppChartDir).
		WithGetValuesFuncs(getValues, addonfactory.GetValuesFromAddonAnnotation).
		WithAgentRegistrationOption(appRegistrationOption).
		BuildHelmAgentAddon()
	if err != nil {
		klog.Errorf("failed to build agent %v", err)
		return err
	}

	err = mgr.AddAgent(certAgentAddon)
	if err != nil {
		klog.Fatal(err)
	}

	err = mgr.Start(ctx)
	if err != nil {
		klog.Fatal(err)
	}
	<-ctx.Done()

	return nil
}
