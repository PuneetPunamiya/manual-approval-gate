package v1alpha1

import (
	"context"

	"knative.dev/pkg/apis"
)

var _ apis.Defaultable = (*ApprovalTask)(nil)

func (at *ApprovalTask) SetDefaults(ctx context.Context) {
	//at.Spec.SetDefaults
}
