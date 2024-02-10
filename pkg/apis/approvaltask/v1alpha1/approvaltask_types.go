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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ApprovalTask is a "wait for manual approval" Task.
// +k8s:openapi-gen=true
type ApprovalTask struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata"`

	// Spec holds the desired state of the TaskGroup from the client
	// +optional
	Spec   ApprovalTaskSpec   `json:"spec"`
	Params []Param            `json:"params,omitempty"`
	Status ApprovalTaskStatus `json:"status"`
}

type ApprovalTaskSpec struct {
	Approvals         []Input `json:"approvals"`
	ApprovalsRequired int     `json:"approvalsRequired"`
}

type Input struct {
	Name       string `json:"name"`
	InputValue string `json:"input"`
}

type Param struct {
	Name  string   `json:"name,omitempty"`
	Value []string `json:"value,omitempty"`
}

type ApprovalTaskStatus struct {
	ApprovalState string   `json:"approvalState"`
	Approvals     []string `json:"approvals"`
	ApprovedBy    []Users  `json:"approvedBy"`
}

type Users struct {
	Name     string `json:"name"`
	Approved string `json:"approved"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ApprovalTaskList contains a list of ApprovalTasks
type ApprovalTaskList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ApprovalTask `json:"items"`
}

// ApprovalTaskRunStatus contains the status stored in the ExtraFields of a Run that references a ApprovalTask.
type ApprovalTaskRunStatus struct {
	// ApprovalTaskSpec contains the exact spec used to instantiate the Run
	// FIXME(openshift-pipelines) can probably remove
	ApprovalTaskSpec *ApprovalTaskSpec `json:"taskLoopSpec,omitempty"`
	// +optional
	// TaskRun *v1beta1.TaskRunStatus `json:"status,omitempty"`
}

// ApprovalTaskRunReason represents a reason for the Run "Succeeded" condition
type ApprovalTaskRunReason string

const (
	// ApprovalTaskRunReasonStarted is the reason set when the Run has just started
	ApprovalTaskRunReasonStarted ApprovalTaskRunReason = "Started"

	// ApprovalTaskRunReasonRunning indicates that the Run is in progress
	ApprovalTaskRunReasonRunning ApprovalTaskRunReason = "Running"

	// ApprovalTaskRunReasonFailed indicates that one of the TaskRuns created from the Run failed
	ApprovalTaskRunReasonFailed ApprovalTaskRunReason = "Failed"

	// ApprovalTaskRunReasonSucceeded indicates that all of the TaskRuns created from the Run completed successfully
	ApprovalTaskRunReasonSucceeded ApprovalTaskRunReason = "Succeeded"

	// ApprovalTaskRunReasonCouldntCancel indicates that a Run was cancelled but attempting to update
	// the running TaskRun as cancelled failed.
	ApprovalTaskRunReasonCouldntCancel ApprovalTaskRunReason = "ApprovalTaskRunCouldntCancel"

	// ApprovalTaskRunReasonCouldntGetApprovalTask indicates that the associated ApprovalTask couldn't be retrieved
	ApprovalTaskRunReasonCouldntGetApprovalTask ApprovalTaskRunReason = "CouldntGetApprovalTask"

	// ApprovalTaskRunReasonFailedValidation indicates that the ApprovalTask failed runtime validation
	ApprovalTaskRunReasonFailedValidation ApprovalTaskRunReason = "ApprovalTaskValidationFailed"

	// ApprovalTaskRunReasonInternalError indicates that the ApprovalTask failed due to an internal error in the reconciler
	ApprovalTaskRunReasonInternalError ApprovalTaskRunReason = "ApprovalTaskInternalError"
)

func (t ApprovalTaskRunReason) String() string {
	return string(t)
}
