package main

import (
	"context"
	"fmt"
	"os"

	"github.com/openshift-pipelines/manual-approval-gate/pkg/apis/approvaltask/v1alpha1"
	validation "github.com/openshift-pipelines/manual-approval-gate/pkg/reconciler/webhook"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/webhook"
	"knative.dev/pkg/webhook/certificates"
	"knative.dev/pkg/webhook/resourcesemantics"
)

var types = map[schema.GroupVersionKind]resourcesemantics.GenericCRD{
	// v1alpha1
	v1alpha1.SchemeGroupVersion.WithKind("VerificationPolicy"): &v1alpha1.ApprovalTask{},
}

func newValidationAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	return validation.NewAdmissionController(ctx,
		"webhook.manual.approval.dev",
		"/resource-validation",
		func(ctx context.Context) context.Context {
			return ctx
		},
		types,
		true,
	)

}

func main() {
	fmt.Println("======================================")
	fmt.Println("Server started yoo...")
	fmt.Println("======================================")
	serviceName := os.Getenv("WEBHOOK_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "manual-approval-webhook"
	}

	secretName := os.Getenv("WEBHOOK_SECRET_NAME")
	if secretName == "" {
		secretName = "manual-approval-webhook-certs" // #nosec
	}

	webhookName := os.Getenv("WEBHOOK_ADMISSION_CONTROLLER_NAME")
	if webhookName == "" {
		webhookName = "webhook.manual.approval.dev"
	}

	systemNamespace := os.Getenv("SYSTEM_NAMESPACE")
	// Scope informers to the webhook's namespace instead of cluster-wide
	ctx := injection.WithNamespaceScope(signals.NewContext(), systemNamespace)

	fmt.Println("Name --->", webhookName)
	fmt.Println("secretName --->", secretName)
	fmt.Println("serviceName --->", serviceName)

	// Set up a signal context with our webhook options
	ctx = webhook.WithOptions(ctx, webhook.Options{
		ServiceName: serviceName,
		Port:        webhook.PortFromEnv(8443),
		SecretName:  secretName,
	})

	port := os.Getenv("PROBES_PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Println("works fine till here...")

	// sharedmain.MainWithContext(ctx, serviceName,
	// 	certificates.NewController,
	// 	newValidationAdmissionController,
	// )

	sharedmain.WebhookMainWithConfig(ctx, serviceName,
		injection.ParseAndGetRESTConfigOrDie(),
		certificates.NewController,
		newValidationAdmissionController,
		// proxy.NewProxyDefaultingAdmissionController,
	)
}
