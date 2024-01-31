package validation

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	admissionlisters "k8s.io/client-go/listers/admissionregistration/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmp"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/system"
	"knative.dev/pkg/webhook"
	certresources "knative.dev/pkg/webhook/certificates/resources"
	"knative.dev/pkg/webhook/resourcesemantics"
)

// reconciler implements the AdmissionController for resources
type reconciler struct {
	webhook.StatelessAdmissionImpl
	pkgreconciler.LeaderAwareFuncs

	key         types.NamespacedName
	path        string
	handlers    map[schema.GroupVersionKind]resourcesemantics.GenericCRD
	withContext func(context.Context) context.Context

	client       kubernetes.Interface
	mwhlister    admissionlisters.MutatingWebhookConfigurationLister
	secretlister corelisters.SecretLister

	disallowUnknownFields bool
	secretName            string
}

var _ controller.Reconciler = (*reconciler)(nil)
var _ pkgreconciler.LeaderAware = (*reconciler)(nil)
var _ webhook.AdmissionController = (*reconciler)(nil)
var _ webhook.StatelessAdmissionController = (*reconciler)(nil)

// Reconcile implements controller.Reconciler
func (ac *reconciler) Reconcile(ctx context.Context, key string) error {
	logger := logging.FromContext(ctx)

	fmt.Println("It did reconcile bro ...")
	if !ac.IsLeaderFor(ac.key) {
		return controller.NewSkipKey(key)
	}

	// Look up the webhook secret, and fetch the CA cert bundle.
	secret, err := ac.secretlister.Secrets(system.Namespace()).Get(ac.secretName)
	if err != nil {
		fmt.Println("Secret mein error aya hai")
		logger.Errorw("Error fetching secret", zap.Error(err))
		return err
	}

	caCert, ok := secret.Data[certresources.CACert]
	if !ok {
		fmt.Println("Cert assign karte time error aya hai....")
		// return fmt.Errorf("secret %q is missing %q key", ac.secretName, certresources.CACert)
	}

	// Reconcile the webhook configuration.
	return ac.reconcileMutatingWebhook(ctx, caCert)
}

func (ac *reconciler) Admit(ctx context.Context, request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	if ac.withContext != nil {
		ctx = ac.withContext(ctx)
	}

	fmt.Println("+++++++++++++++++++++++++++++++++++++")
	fmt.Println("Bhai aa raha hai kya tu ...", request.UserInfo.Username)
	fmt.Println("+++++++++++++++++++++++++++++++++++++")
	return nil
}

func (ac *reconciler) reconcileMutatingWebhook(ctx context.Context, caCert []byte) error {
	logger := logging.FromContext(ctx)
	rules := []admissionregistrationv1.RuleWithOperations{
		{
			Operations: []admissionregistrationv1.OperationType{
				admissionregistrationv1.Create,
			},
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{"openshift-pipelines.org"},
				APIVersions: []string{"v1alpha1"},
				Resources:   []string{"approvaltask", "approvaltasks"},
			},
		},
	}

	configuredWebhook, err := ac.mwhlister.Get(ac.key.Name)
	if err != nil {
		return err
	}

	webhook := configuredWebhook.DeepCopy()

	webhook.OwnerReferences = nil

	for i, wh := range webhook.Webhooks {
		if wh.Name != webhook.Name {
			continue
		}
		webhook.Webhooks[i].Rules = rules
		webhook.Webhooks[i].NamespaceSelector = &metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{{
				Key:      "operator.tekton.dev/disable-proxy",
				Operator: metav1.LabelSelectorOpDoesNotExist,
			}, {
				// "control-plane" is added to support Azure's AKS, otherwise the controllers fight.
				// See knative/pkg#1590 for details.
				Key:      "control-plane",
				Operator: metav1.LabelSelectorOpDoesNotExist,
			}},
		}
		webhook.Webhooks[i].ObjectSelector = &metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      "app.kubernetes.io/managed-by",
					Values:   []string{"tekton-pipelines", "pipelinesascode.tekton.dev"},
					Operator: metav1.LabelSelectorOpIn,
				},
			},
		}
		webhook.Webhooks[i].ClientConfig.CABundle = caCert
		if webhook.Webhooks[i].ClientConfig.Service == nil {
			return fmt.Errorf("missing service reference for webhook: %s", wh.Name)
		}
		webhook.Webhooks[i].ClientConfig.Service.Path = ptr.String(ac.Path())

		fmt.Println("Rules toh bata de bhai")
		fmt.Println(webhook.Webhooks[i].Rules)
	}

	if ok, err := kmp.SafeEqual(configuredWebhook, webhook); err != nil {
		return fmt.Errorf("error diffing webhooks: %w", err)
	} else if !ok {
		logger.Info("Updating webhook")
		mwhclient := ac.client.AdmissionregistrationV1().MutatingWebhookConfigurations()
		if _, err := mwhclient.Update(ctx, webhook, metav1.UpdateOptions{}); err != nil {
			fmt.Println("Error aya hai update mein", err)
			return fmt.Errorf("failed to update webhook: %w", err)
		}
	} else {
		logger.Info("Webhook is valid")
	}
	return nil
}

// Path implements AdmissionController
func (ac *reconciler) Path() string {
	return ac.path
}
