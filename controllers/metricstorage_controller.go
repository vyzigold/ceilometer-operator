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
	"reflect"
	"time"

	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	logr "github.com/go-logr/logr"
	common "github.com/openstack-k8s-operators/lib-common/modules/common"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	helper "github.com/openstack-k8s-operators/lib-common/modules/common/helper"

	telemetryv1 "github.com/openstack-k8s-operators/telemetry-operator/api/v1beta1"
	ceilometer "github.com/openstack-k8s-operators/telemetry-operator/pkg/ceilometer"
	metricstorage "github.com/openstack-k8s-operators/telemetry-operator/pkg/metricstorage"
	monv1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1"
	obov1 "github.com/rhobs/observability-operator/pkg/apis/monitoring/v1alpha1"
)

// MetricStorageReconciler reconciles a MetricStorage object
type MetricStorageReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Scheme  *runtime.Scheme
}

// GetLogger returns a logger object with a prefix of "conroller.name" and aditional controller context fields
func (r *MetricStorageReconciler) GetLogger(ctx context.Context) logr.Logger {
	return log.FromContext(ctx).WithName("Controllers").WithName("MetricStorage")
}

//+kubebuilder:rbac:groups=telemetry.openstack.org,resources=metricstorages,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=telemetry.openstack.org,resources=metricstorages/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=telemetry.openstack.org,resources=metricstorages/finalizers,verbs=update
//+kubebuilder:rbac:groups=monitoring.rhobs,resources=monitoringstacks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=monitoring.rhobs,resources=servicemonitors,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=monitoring.rhobs,resources=scrapeconfigs,verbs=get;list;watch;create;update;patch;delete

// Reconcile reconciles MetricStorage
func (r *MetricStorageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	Log := r.GetLogger(ctx)

	// Fetch the MetricStorage instance
	instance := &telemetryv1.MetricStorage{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected.
			// For additional cleanup logic use finalizers. Return and don't requeue.
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	helper, err := helper.NewHelper(
		instance,
		r.Client,
		r.Kclient,
		r.Scheme,
		Log,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always patch the instance status when exiting this function so we can persist any changes.
	defer func() {
		// update the Ready condition based on the sub conditions
		if instance.Status.Conditions.AllSubConditionIsTrue() {
			instance.Status.Conditions.MarkTrue(
				condition.ReadyCondition, condition.ReadyMessage)
		} else {
			// something is not ready so reset the Ready condition
			instance.Status.Conditions.MarkUnknown(
				condition.ReadyCondition, condition.InitReason, condition.ReadyInitMessage)
			// and recalculate it based on the state of the rest of the conditions
			instance.Status.Conditions.Set(
				instance.Status.Conditions.Mirror(condition.ReadyCondition))
		}
		err := helper.PatchInstance(ctx, instance)
		if err != nil {
			return
		}
	}()

	// If we're not deleting this and the service object doesn't have our finalizer, add it.
	if instance.DeletionTimestamp.IsZero() && controllerutil.AddFinalizer(instance, helper.GetFinalizer()) {
		return ctrl.Result{}, nil
	}

	//
	// initialize status
	//
	if instance.Status.Conditions == nil {
		instance.Status.Conditions = condition.Conditions{}
		// initialize conditions used later as Status=Unknown
		cl := condition.CreateList(
			// Prometheus conditions
			condition.UnknownCondition(telemetryv1.PrometheusReadyCondition, condition.InitReason, telemetryv1.PrometheusReadyInitMessage),
		)

		instance.Status.Conditions.Init(&cl)

		// Register overall status immediately to have an early feedback e.g. in the cli
		return ctrl.Result{}, nil
	}

	// Handle service delete
	if !instance.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, instance, helper)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, instance, helper)
}

func (r *MetricStorageReconciler) reconcileDelete(
	ctx context.Context,
	instance *telemetryv1.MetricStorage,
	helper *helper.Helper,
) (ctrl.Result, error) {
	Log := r.GetLogger(ctx)
	Log.Info("Reconciling Service delete")
	// Service is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(instance, helper.GetFinalizer())
	Log.Info(fmt.Sprintf("Reconciled Service '%s' delete successfully", instance.Name))

	return ctrl.Result{}, nil
}

func (r *MetricStorageReconciler) reconcileInit(
	ctx context.Context,
	instance *telemetryv1.MetricStorage,
	helper *helper.Helper,
	serviceLabels map[string]string,
) (ctrl.Result, error) {
	Log := r.GetLogger(ctx)
	Log.Info("Reconciling Service init")
	Log.Info("Reconciled Service init successfully")
	return ctrl.Result{}, nil
}

func (r *MetricStorageReconciler) reconcileNormal(
	ctx context.Context,
	instance *telemetryv1.MetricStorage,
	helper *helper.Helper,
) (ctrl.Result, error) {
	Log := r.GetLogger(ctx)
	Log.Info(fmt.Sprintf("Reconciling Service '%s'", instance.Name))

	serviceLabels := map[string]string{
		common.AppSelector: "metricStorage",
	}

	// Deploy monitoring stack
	monitoringStack := &obov1.MonitoringStack{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
	}
	op, err := controllerutil.CreateOrPatch(ctx, r.Client, monitoringStack, func() error {
		if reflect.DeepEqual(instance.Spec.CustomMonitoringStack, obov1.MonitoringStackSpec{}) {
			Log.Info(fmt.Sprintf("Using MetricStorage exposed options for MonitoringStack %s definition", monitoringStack.Name))
			desiredMonitoringStack, err := metricstorage.MonitoringStack(instance, serviceLabels)
			if err != nil {
				return err
			}
			desiredMonitoringStack.Spec.DeepCopyInto(&monitoringStack.Spec)
		} else {
			Log.Info(fmt.Sprintf("Using CustomMonitoringStack for MonitoringStack %s definition", monitoringStack.Name))
			instance.Spec.ScrapeInterval = ""
			instance.Spec.Storage = telemetryv1.Storage{}
			instance.Spec.CustomMonitoringStack.DeepCopyInto(&monitoringStack.Spec)
		}
		monitoringStack.ObjectMeta.Labels = serviceLabels
		err := controllerutil.SetControllerReference(instance, monitoringStack, r.Scheme)
		return err
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		Log.Info(fmt.Sprintf("MonitoringStack %s successfully changed - operation: %s", monitoringStack.Name, string(op)))
	}
	monitoringStackReady := true
	for _, c := range monitoringStack.Status.Conditions {
		if c.Status != "True" {
			instance.Status.Conditions.MarkFalse(telemetryv1.PrometheusReadyCondition,
				condition.Reason(c.Reason),
				condition.SeverityError,
				c.Message)
			monitoringStackReady = false
			break
		}
	}
	if len(monitoringStack.Status.Conditions) == 0 {
		monitoringStackReady = false
	}
	if monitoringStackReady {
		instance.Status.Conditions.MarkTrue(telemetryv1.PrometheusReadyCondition, condition.ReadyMessage)
	} else {
		// TODO: reschedule?
		return ctrl.Result{RequeueAfter: time.Duration(10) * time.Second}, nil
	}

	// Deploy ServiceMonitor for ceilometer monitoring
	ceilometerMonitor := &monv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
	}
	op, err = controllerutil.CreateOrPatch(ctx, r.Client, ceilometerMonitor, func() error {
		ceilometerLabels := map[string]string{
			common.AppSelector: ceilometer.ServiceName,
		}
		desiredCeilometerMonitor := metricstorage.ServiceMonitor(instance, serviceLabels, ceilometerLabels)
		desiredCeilometerMonitor.Spec.DeepCopyInto(&ceilometerMonitor.Spec)
		ceilometerMonitor.ObjectMeta.Labels = desiredCeilometerMonitor.ObjectMeta.Labels
		err = controllerutil.SetControllerReference(instance, ceilometerMonitor, r.Scheme)
		return err
	})

	// Handle service init
	ctrlResult, err := r.reconcileInit(ctx, instance, helper, serviceLabels)
	if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}
	if err != nil {
		return ctrlResult, err
	}
	Log.Info("Reconciled Service successfully")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MetricStorageReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&telemetryv1.MetricStorage{}).
		// TODO make dynamic
		Owns(&obov1.MonitoringStack{}).
		Owns(&monv1.ServiceMonitor{}).
		// TODO Own ScrapeConfig
		Complete(r)
}
