package main

import (
	"context"
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
	v1alpha1.SchemeGroupVersion.WithKind("ApprovalTask"): &v1alpha1.ApprovalTask{},
}

func newValidationAdmissionController(name string) func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		return validation.NewAdmissionController(ctx,
			name,
			"/defaulting",
			func(ctx context.Context) context.Context {
				return ctx
			},
			// types,
			true,
		)
	}
}

func main() {
	serviceName := os.Getenv("WEBHOOK_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "manual-approval-webhook"
	}

	secretName := os.Getenv("WEBHOOK_SECRET_NAME")
	if secretName == "" {
		secretName = "manual-approval-gate-webhook-certs" // #nosec
	}

	webhookName := os.Getenv("WEBHOOK_ADMISSION_CONTROLLER_NAME")
	if webhookName == "" {
		webhookName = "webhook.manual.approval.dev"
	}

	systemNamespace := os.Getenv("SYSTEM_NAMESPACE")
	// Scope informers to the webhook's namespace instead of cluster-wide
	ctx := injection.WithNamespaceScope(signals.NewContext(), systemNamespace)

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

	sharedmain.WebhookMainWithConfig(ctx, serviceName,
		injection.ParseAndGetRESTConfigOrDie(),
		certificates.NewController,
		newValidationAdmissionController(webhookName),
	)
}
