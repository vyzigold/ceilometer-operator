/*
Copyright 2022.

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
	"fmt"
	"time"

	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	common "github.com/openstack-k8s-operators/lib-common/modules/common"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	helper "github.com/openstack-k8s-operators/lib-common/modules/common/helper"

	heatv1 "github.com/openstack-k8s-operators/heat-operator/api/v1beta1"
	telemetryv1 "github.com/openstack-k8s-operators/telemetry-operator/api/v1beta1"
	autoscaling "github.com/openstack-k8s-operators/telemetry-operator/pkg/autoscaling"
)

func (r *AutoscalingReconciler) reconcileDisabledHeat(
	ctx context.Context,
	instance *telemetryv1.Autoscaling,
	helper *helper.Helper,
) (ctrl.Result, error) {
	r.Log.Info("Reconciling Service Heat disabled")

	serviceLabels := map[string]string{
		common.AppSelector: autoscaling.ServiceName,
	}
	heat := autoscaling.Heat(instance, serviceLabels)
	err := r.Client.Delete(ctx, heat)
	if err != nil {
		if !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
	}

	// Set the condition to true, since the service is disabled
	for _, c := range instance.Status.Conditions {
		instance.Status.Conditions.MarkTrue(c.Type, telemetryv1.AutoscalingReadyDisabledMessage)
	}
	r.Log.Info(fmt.Sprintf("Reconciled Service '%s' disable successfully", autoscaling.ServiceName))
	return ctrl.Result{}, nil
}

func (r *AutoscalingReconciler) reconcileDeleteHeat(
	ctx context.Context,
	instance *telemetryv1.Autoscaling,
	helper *helper.Helper,
) (ctrl.Result, error) {
	r.Log.Info("Reconciling Service Heat delete")
	r.Log.Info(fmt.Sprintf("Reconciled Service '%s' delete successfully", autoscaling.ServiceName))

	return ctrl.Result{}, nil
}

func (r *AutoscalingReconciler) reconcileInitHeat(
	ctx context.Context,
	instance *telemetryv1.Autoscaling,
	helper *helper.Helper,
	serviceLabels map[string]string,
) (ctrl.Result, error) {
	// TODO: init?
	return ctrl.Result{}, nil
}

func (r *AutoscalingReconciler) reconcileNormalHeat(
	ctx context.Context,
	instance *telemetryv1.Autoscaling,
	helper *helper.Helper,
) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Reconciling Service Heat"))
	serviceLabels := map[string]string{
		common.AppSelector: autoscaling.ServiceName,
	}

	if instance.Spec.Heat.Template != nil {
		heatCR := autoscaling.Heat(instance, serviceLabels)
		op, err := controllerutil.CreateOrUpdate(ctx, r.Client, heatCR, func() error {
			err := controllerutil.SetControllerReference(instance, heatCR, r.Scheme)
			return err
		})
		if err != nil {
			return ctrl.Result{}, err
		}
		if op != controllerutil.OperationResultNone {
			r.Log.Info(fmt.Sprintf("Heat %s successfully reconciled - operation: %s", heatCR.ObjectMeta.Name, string(op)))
		}
	}
	heat, err := r.getAutoscalingHeat(ctx, helper, instance)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			instance.Status.Conditions.Set(condition.FalseCondition(
				telemetryv1.HeatReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				telemetryv1.HeatReadyNotFoundMessage))
			return ctrl.Result{RequeueAfter: time.Duration(10) * time.Second}, fmt.Errorf("heat %s not found", instance.Spec.Heat.InstanceName)
		}
		instance.Status.Conditions.Set(condition.FalseCondition(
			telemetryv1.HeatReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			telemetryv1.HeatReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}

	if !heat.IsReady() {
		instance.Status.Conditions.Set(condition.FalseCondition(
			telemetryv1.HeatReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			telemetryv1.HeatReadyUnreadyMessage))
		return ctrl.Result{RequeueAfter: time.Duration(10) * time.Second}, fmt.Errorf("heat %s is not ready", heat.Name)
	}
	// Mark the Heat Service as Ready if we get to this point with no errors
	instance.Status.Conditions.MarkTrue(
		telemetryv1.HeatReadyCondition, condition.ReadyMessage)

	r.Log.Info(fmt.Sprintf("Reconciled Service Heat '%s' successfully", heat.Name))
	return ctrl.Result{}, nil
}

func (r *AutoscalingReconciler) getAutoscalingHeat(
	ctx context.Context,
	h *helper.Helper,
	instance *telemetryv1.Autoscaling,
) (*heatv1.Heat, error) {
	heat := &heatv1.Heat{}
	err := h.GetClient().Get(
		ctx,
		types.NamespacedName{
			Name:      instance.Spec.Heat.InstanceName,
			Namespace: instance.Namespace,
		},
		heat)
	if err != nil {
		return nil, err
	}
	return heat, err
}
