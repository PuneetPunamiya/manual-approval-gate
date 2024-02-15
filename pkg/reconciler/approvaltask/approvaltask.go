/*
Copyright 2022 The OpenShift Pipelines Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package approvaltask

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/go-multierror"
	approvaltaskv1alpha1 "github.com/openshift-pipelines/manual-approval-gate/pkg/apis/approvaltask/v1alpha1"
	v1alpha1 "github.com/openshift-pipelines/manual-approval-gate/pkg/apis/approvaltask/v1alpha1"
	approvaltaskclientset "github.com/openshift-pipelines/manual-approval-gate/pkg/client/clientset/versioned"
	listersapprovaltask "github.com/openshift-pipelines/manual-approval-gate/pkg/client/listers/approvaltask/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	customrunreconciler "github.com/tektoncd/pipeline/pkg/client/injection/reconciler/pipeline/v1beta1/customrun"
	listersalpha "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1alpha1"
	listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/reconciler/events"
	"go.uber.org/zap"
	"gomodules.xyz/jsonpatch/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

const (
	// approvaltaskLabelKey is the label identifier for a ApprovalTask.  This label is added to the Run and its TaskRuns.
	approvaltaskLabelKey = "/approvaltask"

	// approvaltaskRunLabelKey is the label identifier for a Run.  This label is added to the Run's TaskRuns.
	approvaltaskRunLabelKey = "/run"
)

// Reconciler implements controller.Reconciler for Configuration resources.
type Reconciler struct {
	pipelineClientSet     clientset.Interface
	kubeClientSet         kubernetes.Interface
	approvaltaskClientSet approvaltaskclientset.Interface
	runLister             listersalpha.RunLister
	customRunLister       listers.CustomRunLister
	approvaltaskLister    listersapprovaltask.ApprovalTaskLister
	taskRunLister         listers.TaskRunLister
}

var (
	// Check that our Reconciler implements runreconciler.Interface
	_                customrunreconciler.Interface = (*Reconciler)(nil)
	cancelPatchBytes []byte
)

func init() {
	var err error
	patches := []jsonpatch.JsonPatchOperation{{
		Operation: "add",
		Path:      "/spec/status",
		Value:     v1beta1.TaskRunSpecStatusCancelled,
	}}
	cancelPatchBytes, err = json.Marshal(patches)
	if err != nil {
		log.Fatalf("failed to marshal patch bytes in order to cancel: %v", err)
	}
}

// ReconcileKind compares the actual state with the desired, and attempts to converge the two.
// It then updates the Status block of the Run resource with the current status of the resource.
func (c *Reconciler) ReconcileKind(ctx context.Context, run *v1beta1.CustomRun) pkgreconciler.Event {
	var merr error
	logger := logging.FromContext(ctx)
	logger.Infof("Reconciling Run %s/%s at %v", run.Namespace, run.Name, time.Now())

	// Check that the Run references a ApprovalTask CRD.  The logic is controller.go should ensure that only this type of Run
	// is reconciled this controller but it never hurts to do some bullet-proofing.
	if err := checkCustomRunReferencesApprovalTask(run); err != nil {
		return err
	}

	// If the Run has not started, initialize the Condition and set the start time.
	initializeCustomRun(ctx, run)

	if run.IsDone() {
		logger.Infof("Run %s/%s is done", run.Namespace, run.Name)
		return nil
	}

	// Store the condition before reconcile
	beforeCondition := run.Status.GetCondition(apis.ConditionSucceeded)

	status := &approvaltaskv1alpha1.ApprovalTaskRunStatus{}
	if err := run.Status.DecodeExtraFields(status); err != nil {
		run.Status.MarkCustomRunFailed(approvaltaskv1alpha1.ApprovalTaskRunReasonInternalError.String(),
			"Internal error calling DecodeExtraFields: %v", err)
		logger.Errorf("DecodeExtraFields error: %v", err.Error())
	}

	// Reconcile the Run
	if err := c.reconcile(ctx, run, status); err != nil {
		logger.Errorf("Reconcile error: %v", err.Error())
		merr = multierror.Append(merr, err)
		return merr
	}

	if err := c.updateLabelsAndAnnotations(ctx, run); err != nil {
		logger.Warn("Failed to update Run labels/annotations", zap.Error(err))
		merr = multierror.Append(merr, err)
	}

	if err := run.Status.EncodeExtraFields(status); err != nil {
		run.Status.MarkCustomRunFailed(approvaltaskv1alpha1.ApprovalTaskRunReasonInternalError.String(),
			"Internal error calling EncodeExtraFields: %v", err)
		logger.Errorf("EncodeExtraFields error: %v", err.Error())
	}

	afterCondition := run.Status.GetCondition(apis.ConditionSucceeded)
	events.Emit(ctx, beforeCondition, afterCondition, run)

	// Only transient errors that should retry the reconcile are returned.
	return merr
}

func (r *Reconciler) reconcile(ctx context.Context, run *v1beta1.CustomRun, status *approvaltaskv1alpha1.ApprovalTaskRunStatus) error {
	logger := logging.FromContext(ctx)

	// Get the ApprovalTask referenced by the Run

	approvalTask, err := r.getOrCreateApprovalTask(ctx, run)
	if err != nil {
		return err
	}

	approvalTaskMeta := &approvalTask.ObjectMeta
	approvalTaskSpec := approvalTask.Spec

	// Store the fetched ApprovalTaskSpec on the Run for auditing
	// storeApprovalTaskSpec(status, approvaltaskSpec)

	// Propagate labels and annotations from ApprovalTask to Run.
	propagateApprovalTaskLabelsAndAnnotations(run, approvalTaskMeta)

	// Validate ApprovalTask spec
	if err := approvalTaskSpec.Validate(ctx); err != nil {
		run.Status.MarkCustomRunFailed(approvaltaskv1alpha1.ApprovalTaskRunReasonFailedValidation.String(),
			"ApprovalTask %s/%s can't be Run; it has an invalid spec: %s",
			approvalTaskMeta.Namespace, approvalTaskMeta.Name, err)
		return nil
	}

	// ---------------------Updating the approvedBy field in the status ------------------
	// Temp map to hold current approvals with true input
	currentApprovals := make(map[string]string)
	approvalTask.Status.ApprovedBy = []v1alpha1.Users{}
	// Populate the map with approvals having input true
	for _, approval := range approvalTask.Spec.Approvals {
		if approval.InputValue == "true" {
			currentApprovals[approval.Name] = "true"
		} else if approval.InputValue == "false" {
			currentApprovals[approval.Name] = "false"
		}
	}

	// Filter the ApprovedBy to only include those that are still true
	filteredApprovedBy := []v1alpha1.Users{}
	for name, value := range currentApprovals {
		filteredApprovedBy = append(filteredApprovedBy, v1alpha1.Users{Name: name, Approved: value})
	}

	// Update the ApprovedBy list
	approvalTask.Status.ApprovedBy = filteredApprovedBy

	// -------------------------- Update the approvalState ------------------
	// False scenario: Check if there is one false and if found mark the approvalstate to false
	if ApprovalTaskHasFalseInput(*approvalTask) {
		approvalTask.Status.ApprovalState = "false"
	} else if ApprovalTaskHasTrueInput(*approvalTask) {
		fmt.Println("Yaha bhi aa raha hai.....")
		approvalTask.Status.ApprovalState = "true"
	}

	// True scenario: Check if it is approved and the approvalRequired limit is reached mark the approval state as true
	// else mark the approvalstate as wait

	_, err = r.approvaltaskClientSet.OpenshiftpipelinesV1alpha1().ApprovalTasks(run.Namespace).UpdateStatus(ctx, approvalTask, metav1.UpdateOptions{})
	if err != nil {
		fmt.Println("Something wrong", err)
	}

	fmt.Println("==================================")
	fmt.Println("Print the approval task state....", approvalTask.Status.ApprovalState)
	fmt.Println("===================================")
	switch approvalTask.Status.ApprovalState {
	case "wait":
		logger.Info("Approval task is in wait state")
		return nil
	case "false":
		logger.Infof("Approval task %s is denied", approvalTaskMeta.Name)
		run.Status.MarkCustomRunFailed(approvaltaskv1alpha1.ApprovalTaskRunReasonFailed.String(), "Approval Task denied")
		return nil
	case "true":
		fmt.Println("How come it is true")
		run.Status.MarkCustomRunSucceeded(approvaltaskv1alpha1.ApprovalTaskRunReasonSucceeded.String(),
			"TaskRun succeeded")
		return nil
	}

	return nil
}
