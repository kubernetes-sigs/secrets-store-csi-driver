/*
Copyright 2024 The Kubernetes Authors.

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
package controllers

import (
	"context"

	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
	secretsyncv1alpha1 "sigs.k8s.io/secrets-store-csi-driver/secret-sync-controller/api/v1alpha1"
)

const (
	ConditionReasonUnknown  = "Unknown"
	ConditionMessageUnknown = "Unknown"
	ConditionTypeUnknown    = "Unknown"
	ConditionTypeCreate     = "Create"
	ConditionTypeUpdate     = "Update"

	ConditionReasonCreateSucceeded  = "CreateSucceeded"
	ConditionMessageCreateSucceeded = "Secret created successfully."

	ConditionReasonFailedProviderError  = "ProviderError"
	ConditionMessageFailedProviderError = "Secret creation failed due to provider error, check the logs or the events for more information."

	ConditionReasonFailedInvalidLabelError  = "InvalidClusterSecretLabelError"
	ConditionMessageFailedInvalidLabelError = "The secret operation failed because a label reserved for the controller is applied on the secret."

	ConditionReasonFailedInvalidAnnotationError  = "InvalidClusterSecretAnnotationError"
	ConditionMessageFailedInvalidAnnotationError = "The secret create failed because an annotation reserved for the controller is applied on the secret."

	ConditionReasonUpdateNoValueChangeSucceeded  = "UpdateNoValueChangeSucceeded"
	ConditionMessageUpdateNoValueChangeSucceeded = "The secret was updated successfully at the end of the poll interval and no value change was detected."

	ConditionReasonUpdateValueChangeOrForceUpdateSucceeded  = "UpdateValueChangeOrForceUpdateSucceeded"
	ConditionMessageUpdateValueChangeOrForceUpdateSucceeded = "The secret was updated successfully: a value change or a force update was detected."

	ConditionReasonSecretPatchFailedUnknownError  = "UnknownError"
	ConditionMessageSecretPatchFailedUnknownError = "Secret patch failed due to unknown error, check the logs or the events for more information."

	ConditionReasonValidatingAdmissionPolicyCheckFailed  = "ValidatingAdmissionPolicyCheckFailed"
	ConditionMessageValidatingAdmissionPolicyCheckFailed = "Secret update failed due to validating admission policy check failure, check the logs or the events for more information."

	ConditionReasonControllerInternalError  = "ControllerInternalError"
	ConditionMessageControllerInternalError = "Secret update failed due to controller internal error, check the logs or the events for more information."

	ConditionReasonControllerSpcError  = "ControllerSPCError"
	ConditionMessageControllerSpcError = "Secret update failed because the controller could not retrieve the Secret Provider Class or the SPC is misconfigured. Check the logs or the events for more information."

	ConditionReasonUserInputValidationFailed  = "UserInputValidationFailed"
	ConditionMessageUserInputValidationFailed = "Secret create or update failed due to SecretProviderClass or SecretSync error, check the logs or the events for more information."
)

var FailedConditionsTriggeringRetry = []string{
	ConditionReasonControllerSpcError,
	ConditionReasonFailedInvalidAnnotationError,
	ConditionReasonFailedInvalidLabelError,
	ConditionReasonFailedProviderError,
	ConditionReasonFailedInvalidAnnotationError,
	ConditionReasonFailedProviderError,
	ConditionReasonSecretPatchFailedUnknownError,
	ConditionReasonValidatingAdmissionPolicyCheckFailed,
	ConditionReasonUserInputValidationFailed,
	ConditionTypeUnknown}

var SucceededConditionsTriggeringRetry = []string{
	ConditionReasonCreateSucceeded,
	ConditionReasonUpdateNoValueChangeSucceeded,
	ConditionReasonUpdateValueChangeOrForceUpdateSucceeded}

var AllowedStringsToDisplayConditionErrorMessage = []string{
	"validatingadmissionpolicy",
}

func (r *SecretSyncReconciler) updateStatusConditions(ctx context.Context, ss *secretsyncv1alpha1.SecretSync, oldConditionType string, newConditionType string, conditionReason string, shouldUpdateStatus bool) {
	logger := log.FromContext(ctx)

	if ss.Status.Conditions == nil {
		ss.Status.Conditions = []metav1.Condition{}
	}

	if len(oldConditionType) > 0 {
		logger.V(10).Info("Removing old condition", "oldConditionType", oldConditionType)
		meta.RemoveStatusCondition(&ss.Status.Conditions, oldConditionType)
	}

	var condition metav1.Condition
	switch conditionReason {
	case ConditionReasonCreateSucceeded:
		condition.Status = metav1.ConditionTrue
		condition.Type = newConditionType
		condition.Reason = ConditionReasonCreateSucceeded
		condition.Message = ConditionMessageCreateSucceeded
	case ConditionReasonUpdateNoValueChangeSucceeded:
		condition.Status = metav1.ConditionTrue
		condition.Type = newConditionType
		condition.Reason = ConditionReasonUpdateNoValueChangeSucceeded
		condition.Message = ConditionMessageUpdateNoValueChangeSucceeded
	case ConditionReasonUpdateValueChangeOrForceUpdateSucceeded:
		condition.Status = metav1.ConditionTrue
		condition.Type = newConditionType
		condition.Reason = ConditionReasonUpdateValueChangeOrForceUpdateSucceeded
		condition.Message = ConditionMessageUpdateValueChangeOrForceUpdateSucceeded
	case ConditionReasonValidatingAdmissionPolicyCheckFailed:
		condition.Status = metav1.ConditionFalse
		condition.Type = newConditionType
		condition.Reason = ConditionReasonValidatingAdmissionPolicyCheckFailed
		condition.Message = ConditionMessageValidatingAdmissionPolicyCheckFailed
	case ConditionReasonFailedInvalidAnnotationError:
		condition.Status = metav1.ConditionFalse
		condition.Type = newConditionType
		condition.Reason = ConditionReasonFailedInvalidAnnotationError
		condition.Message = ConditionMessageFailedInvalidAnnotationError
	case ConditionReasonFailedProviderError:
		condition.Status = metav1.ConditionFalse
		condition.Type = newConditionType
		condition.Reason = ConditionReasonFailedProviderError
		condition.Message = ConditionMessageFailedProviderError
	case ConditionReasonFailedInvalidLabelError:
		condition.Status = metav1.ConditionFalse
		condition.Type = newConditionType
		condition.Reason = ConditionReasonFailedInvalidLabelError
		condition.Message = ConditionMessageFailedInvalidLabelError
	case ConditionReasonSecretPatchFailedUnknownError:
		condition.Status = metav1.ConditionUnknown
		condition.Type = newConditionType
		condition.Reason = ConditionReasonSecretPatchFailedUnknownError
		condition.Message = ConditionMessageSecretPatchFailedUnknownError
	case ConditionReasonUserInputValidationFailed:
		condition.Status = metav1.ConditionFalse
		condition.Type = newConditionType
		condition.Reason = ConditionReasonUserInputValidationFailed
		condition.Message = ConditionMessageUserInputValidationFailed
	case ConditionReasonControllerSpcError:
		condition.Status = metav1.ConditionFalse
		condition.Type = newConditionType
		condition.Reason = ConditionReasonControllerSpcError
		condition.Message = ConditionMessageControllerSpcError
	case ConditionReasonControllerInternalError:
		condition.Status = metav1.ConditionUnknown
		condition.Type = newConditionType
		condition.Reason = ConditionReasonControllerInternalError
		condition.Message = ConditionMessageControllerInternalError
	default:
		condition.Status = metav1.ConditionUnknown
		condition.Type = ConditionTypeUnknown
		condition.Reason = ConditionReasonUnknown
		condition.Message = ConditionMessageUnknown
	}

	logger.V(10).Info("Adding new condition", "newConditionType", newConditionType, "conditionReason", conditionReason)
	meta.SetStatusCondition(&ss.Status.Conditions, condition)

	if !shouldUpdateStatus {
		return
	}

	if err := r.Client.Status().Update(ctx, ss); err != nil {
		logger.Error(err, "Failed to update status", "condition", condition)
	}

	logger.V(10).Info("Updated status", "condition", condition)
}
