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
	k8s_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	helper "github.com/openstack-k8s-operators/lib-common/modules/common/helper"

	telemetryv1 "github.com/openstack-k8s-operators/telemetry-operator/api/v1beta1"
	obov1 "github.com/rhobs/observability-operator/pkg/apis/monitoring/v1alpha1"
)

func reconcileNormalPrometheus(
	ctx context.Context,
	instance k8s_v1.Object,
	prom *obov1.MonitoringStack,
	instanceConditions condition.Conditions,
	helper *helper.Helper,
	deploy bool,
) (ctrl.Result, error) {
	Log := helper.GetLogger()
	Log.Info(fmt.Sprintf("Reconciling prometheus '%s'", prom.Name))

	if deploy {
		op, err := controllerutil.CreateOrUpdate(ctx, helper.GetClient(), prom, func() error {
			err := controllerutil.SetControllerReference(instance, prom, helper.GetScheme())
			return err
		})
		if err != nil {
			return ctrl.Result{}, err
		}
		if op != controllerutil.OperationResultNone {
			Log.Info(fmt.Sprintf("Prometheus %s successfully reconciled - operation: %s", prom.Name, string(op)))
		}
		promReady := true
		for _, c := range prom.Status.Conditions {
			if c.Status != "True" {
				instanceConditions.MarkFalse(telemetryv1.PrometheusReadyCondition,
					condition.Reason(c.Reason),
					condition.SeverityError,
					c.Message)
				promReady = false
				break
			}
		}
		if len(prom.Status.Conditions) == 0 {
			promReady = false
		}
		if promReady {
			instanceConditions.MarkTrue(telemetryv1.PrometheusReadyCondition, condition.ReadyMessage)
		} else {
			return ctrl.Result{RequeueAfter: time.Duration(10) * time.Second}, fmt.Errorf("Prometheus %s isn't ready", prom.Name)
		}
	} else {
		err := helper.GetClient().Delete(ctx, prom)
		if err != nil {
			if !k8s_errors.IsNotFound(err) {
				return ctrl.Result{}, nil
			}
		}
	}

	Log.Info(fmt.Sprintf("Reconciled Service Aodh '%s' successfully", prom.Name))
	return ctrl.Result{}, nil
}
