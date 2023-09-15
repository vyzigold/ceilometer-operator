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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	logr "github.com/go-logr/logr"
	common "github.com/openstack-k8s-operators/lib-common/modules/common"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	configmap "github.com/openstack-k8s-operators/lib-common/modules/common/configmap"
	deployment "github.com/openstack-k8s-operators/lib-common/modules/common/deployment"
	endpoint "github.com/openstack-k8s-operators/lib-common/modules/common/endpoint"
	env "github.com/openstack-k8s-operators/lib-common/modules/common/env"
	helper "github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	job "github.com/openstack-k8s-operators/lib-common/modules/common/job"
	labels "github.com/openstack-k8s-operators/lib-common/modules/common/labels"
	common_rbac "github.com/openstack-k8s-operators/lib-common/modules/common/rbac"
	secret "github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	service "github.com/openstack-k8s-operators/lib-common/modules/common/service"
	util "github.com/openstack-k8s-operators/lib-common/modules/common/util"
	"github.com/openstack-k8s-operators/lib-common/modules/database"
	mariadbv1 "github.com/openstack-k8s-operators/mariadb-operator/api/v1beta1"

	rabbitmqv1 "github.com/openstack-k8s-operators/infra-operator/apis/rabbitmq/v1beta1"
	keystonev1 "github.com/openstack-k8s-operators/keystone-operator/api/v1beta1"
	telemetryv1 "github.com/openstack-k8s-operators/telemetry-operator/api/v1beta1"
	autoscaling "github.com/openstack-k8s-operators/telemetry-operator/pkg/autoscaling"
	obov1 "github.com/rhobs/observability-operator/pkg/apis/monitoring/v1alpha1"
)

// AutoscalingReconciler reconciles a Autoscaling object
type AutoscalingReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Log     logr.Logger
	Scheme  *runtime.Scheme
}

// +kubebuilder:rbac:groups=telemetry.openstack.org,resources=autoscalings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=telemetry.openstack.org,resources=autoscalings/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=telemetry.openstack.org,resources=autoscalings/finalizers,verbs=update
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list
// +kubebuilder:rbac:groups=monitoring.rhobs,resources=monitoringstacks,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups=keystone.openstack.org,resources=keystoneservices,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups=rabbitmq.openstack.org,resources=transporturls,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=keystone.openstack.org,resources=keystoneapis,verbs=get;list;watch;
// service account, role, rolebinding
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=roles,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=rolebindings,verbs=get;list;watch;create;update
// service account permissions that are needed to grant permission to the above
// +kubebuilder:rbac:groups="security.openshift.io",resourceNames=anyuid,resources=securitycontextconstraints,verbs=use

// Reconcile reconciles an Autoscaling
func (r *AutoscalingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues(autoscaling.ServiceName, req.NamespacedName)

	// Fetch the Autoscaling instance
	instance := &telemetryv1.Autoscaling{}
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
		r.Log,
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
			// service account, role, rolebinding conditions
			condition.UnknownCondition(condition.ServiceAccountReadyCondition,
				condition.InitReason,
				condition.ServiceAccountReadyInitMessage),
			condition.UnknownCondition(condition.RoleReadyCondition, condition.InitReason, condition.RoleReadyInitMessage),
			condition.UnknownCondition(condition.RoleBindingReadyCondition,
				condition.InitReason,
				condition.RoleBindingReadyInitMessage),
			condition.UnknownCondition("PrometheusReady", condition.InitReason, "PrometheusNotStarted"),
			condition.UnknownCondition(condition.DBReadyCondition, condition.InitReason, condition.DBReadyInitMessage),
			condition.UnknownCondition(condition.DBSyncReadyCondition, condition.InitReason, condition.DBSyncReadyInitMessage),
			condition.UnknownCondition(condition.RabbitMqTransportURLReadyCondition, condition.InitReason, condition.RabbitMqTransportURLReadyInitMessage),
			// right now we have no dedicated KeystoneServiceReadyInitMessage
			//condition.UnknownCondition(condition.KeystoneServiceReadyCondition, condition.InitReason, ""),
		)

		instance.Status.Conditions.Init(&cl)

		// Register overall status immediately to have an early feedback e.g. in the cli
		return ctrl.Result{}, nil
	}

	if instance.Status.Hash == nil {
		instance.Status.Hash = map[string]string{}
	}

	// Handle service delete
	if !instance.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, instance, helper)
	}

	// Handle service disabled
	if !instance.Spec.Enabled {
		return r.reconcileDisabled(ctx, instance, helper)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, instance, helper)
}

func (r *AutoscalingReconciler) reconcileDisabled(
	ctx context.Context,
	instance *telemetryv1.Autoscaling,
	helper *helper.Helper,
) (ctrl.Result, error) {
	r.Log.Info("Reconciling Service disabled")
	serviceLabels := map[string]string{
		common.AppSelector: autoscaling.ServiceName,
	}
	// remove db finalizer first
	db, err := database.GetDatabaseByName(ctx, helper, instance.Name)
	if err != nil && !k8s_errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if !k8s_errors.IsNotFound(err) {
		//TODO: This doesn't work
		if err := db.DeleteFinalizer(ctx, helper); err != nil {
			return ctrl.Result{}, err
		}
	}

	prom := autoscaling.Prometheus(instance, serviceLabels)
	err = r.Client.Delete(ctx, prom)
	if err != nil {
		if !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
	}

	// Set the condition to true, since the service is disabled
	for _, c := range instance.Status.Conditions {
		instance.Status.Conditions.MarkTrue(c.Type, "Autoscaling disabled")
	}
	r.Log.Info(fmt.Sprintf("Reconciled Service '%s' disable successfully", autoscaling.ServiceName))
	return ctrl.Result{}, nil
}

func (r *AutoscalingReconciler) reconcileDelete(
	ctx context.Context,
	instance *telemetryv1.Autoscaling,
	helper *helper.Helper,
) (ctrl.Result, error) {
	r.Log.Info("Reconciling Service delete")

	// remove db finalizer first
	db, err := database.GetDatabaseByName(ctx, helper, instance.Name)
	if err != nil && !k8s_errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if !k8s_errors.IsNotFound(err) {
		if err := db.DeleteFinalizer(ctx, helper); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Remove the finalizer from our KeystoneService CR
	keystoneService, err := keystonev1.GetKeystoneServiceWithName(ctx, helper, autoscaling.ServiceName, instance.Namespace)
	if err != nil && !k8s_errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if err == nil {
		if controllerutil.RemoveFinalizer(keystoneService, helper.GetFinalizer()) {
			err = r.Update(ctx, keystoneService)
			if err != nil && !k8s_errors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
			util.LogForObject(helper, "Removed finalizer from our KeystoneService", instance)
		}
	}

	// Service is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(instance, helper.GetFinalizer())
	r.Log.Info(fmt.Sprintf("Reconciled Service '%s' delete successfully", autoscaling.ServiceName))

	return ctrl.Result{}, nil
}

func (r *AutoscalingReconciler) reconcileInit(
	ctx context.Context,
	instance *telemetryv1.Autoscaling,
	helper *helper.Helper,
	serviceLabels map[string]string,
) (ctrl.Result, error) {
	r.Log.Info("Reconciling Service init")
	_, _, err := secret.GetSecret(ctx, helper, instance.Spec.Aodh.Secret, instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: time.Duration(10) * time.Second}, fmt.Errorf("OpenStack secret %s not found", instance.Spec.Aodh.Secret)
		}
		return ctrl.Result{}, err
	}

	ksSvcSpec := keystonev1.KeystoneServiceSpec{
		ServiceType:        autoscaling.ServiceType,
		ServiceName:        autoscaling.ServiceName,
		ServiceDescription: "Aodh for autoscaling Service",
		Enabled:            true,
		ServiceUser:        instance.Spec.Aodh.ServiceUser,
		Secret:             instance.Spec.Aodh.Secret,
		PasswordSelector:   instance.Spec.Aodh.PasswordSelectors.Service,
	}

	ksSvc := keystonev1.NewKeystoneService(ksSvcSpec, instance.Namespace, serviceLabels, 10)
	ctrlResult, err := ksSvc.CreateOrPatch(ctx, helper)
	if err != nil {
		return ctrlResult, err
	}

	// mirror the Status, Reason, Severity and Message of the latest keystoneservice condition
	// into a local condition with the type condition.KeystoneServiceReadyCondition
	c := ksSvc.GetConditions().Mirror(condition.KeystoneServiceReadyCondition)
	if c != nil {
		instance.Status.Conditions.Set(c)
	}

	if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	if instance.Status.Hash == nil {
		instance.Status.Hash = map[string]string{}
	}

	//
	// create service DB instance
	//
	db := database.NewDatabase(
		// instance.Name
		// TODO: We might want to change the db name.
		// The mariadb-operator is currently implemented
		// in a way, that the db name needs to be the
		// same as the user
		instance.Spec.Aodh.DatabaseUser,
		instance.Spec.Aodh.DatabaseUser,
		instance.Spec.Aodh.Secret,
		map[string]string{
			"dbName": instance.Spec.Aodh.DatabaseInstance,
		},
	)
	fmt.Println(instance.Name)
	fmt.Println(instance.Spec.Aodh.DatabaseUser)
	fmt.Println(instance.Spec.Aodh.Secret)
	fmt.Println(instance.Spec.Aodh.DatabaseInstance)
	// create or patch the DB
	ctrlResult, err = db.CreateOrPatchDBByName(
		ctx,
		helper,
		instance.Spec.Aodh.DatabaseInstance,
	)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DBReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.DBReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if (ctrlResult != ctrl.Result{}) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DBReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.DBReadyRunningMessage))
		return ctrlResult, nil
	}
	// wait for the DB to be setup
	ctrlResult, err = db.WaitForDBCreated(ctx, helper)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DBReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.DBReadyErrorMessage,
			err.Error()))
		return ctrlResult, err
	}
	if (ctrlResult != ctrl.Result{}) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DBReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.DBReadyRunningMessage))
		return ctrlResult, nil
	}
	// update Status.DatabaseHostname, used to config the service
	instance.Status.DatabaseHostname = db.GetDatabaseHostname()
	instance.Status.Conditions.MarkTrue(condition.DBReadyCondition, condition.DBReadyMessage)
	// create service DB - end

	//
	// run Aodh db sync
	dbSyncHash := instance.Status.Hash[telemetryv1.DbSyncHash]
	jobDef := autoscaling.DbSyncJob(instance, serviceLabels)

	dbSyncjob := job.NewJob(
		jobDef,
		telemetryv1.DbSyncHash,
		instance.Spec.Aodh.PreserveJobs,
		time.Duration(5)*time.Second,
		dbSyncHash,
	)
	ctrlResult, err = dbSyncjob.DoJob(
		ctx,
		helper,
	)
	if (ctrlResult != ctrl.Result{}) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DBSyncReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.DBSyncReadyRunningMessage))
		return ctrlResult, nil
	}
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DBSyncReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.DBSyncReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if dbSyncjob.HasChanged() {
		instance.Status.Hash[telemetryv1.DbSyncHash] = dbSyncjob.GetHash()
		r.Log.Info(fmt.Sprintf("Service '%s' - Job %s hash added - %s", instance.Name, jobDef.Name, instance.Status.Hash[telemetryv1.DbSyncHash]))
	}
	instance.Status.Conditions.MarkTrue(condition.DBSyncReadyCondition, condition.DBSyncReadyMessage)

	// when job passed, mark NetworkAttachmentsReadyCondition ready
	instance.Status.Conditions.MarkTrue(condition.NetworkAttachmentsReadyCondition, condition.NetworkAttachmentsReadyMessage)

	// run Cinder db sync - end

	r.Log.Info("Reconciled Service init successfully")
	return ctrl.Result{}, nil

}

func (r *AutoscalingReconciler) reconcileNormal(
	ctx context.Context,
	instance *telemetryv1.Autoscaling,
	helper *helper.Helper,
) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Reconciling Service '%s'", autoscaling.ServiceName))

	// Service account, role, binding
	rbacRules := []rbacv1.PolicyRule{
		{
			APIGroups:     []string{"security.openshift.io"},
			ResourceNames: []string{"anyuid"},
			Resources:     []string{"securitycontextconstraints"},
			Verbs:         []string{"use"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods"},
			Verbs:     []string{"create", "get", "list", "watch", "update", "patch", "delete"},
		},
	}
	rbacResult, err := common_rbac.ReconcileRbac(ctx, helper, instance, rbacRules)
	if err != nil {
		return rbacResult, err
	} else if (rbacResult != ctrl.Result{}) {
		return rbacResult, nil
	}

	instance.Status.Conditions.MarkTrue(condition.ServiceConfigReadyCondition, condition.ServiceConfigReadyMessage)

	serviceLabels := map[string]string{
		common.AppSelector: autoscaling.ServiceName,
	}

	//
	// create RabbitMQ transportURL CR and get the actual URL from the associated secret that is created
	//
	transportURL, op, err := r.transportURLCreateOrUpdate(instance)

	if err != nil {
		r.Log.Info("Error getting transportURL. Setting error condition on status and returning")
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.RabbitMqTransportURLReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.RabbitMqTransportURLReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}

	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("TransportURL %s successfully reconciled - operation: %s", transportURL.Name, string(op)))
	}

	instance.Status.TransportURLSecret = transportURL.Status.SecretName

	if instance.Status.TransportURLSecret == "" {
		r.Log.Info(fmt.Sprintf("Waiting for TransportURL %s secret to be created", transportURL.Name))
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.RabbitMqTransportURLReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.RabbitMqTransportURLReadyRunningMessage))
		return ctrl.Result{RequeueAfter: time.Duration(10) * time.Second}, nil
	}

	instance.Status.Conditions.MarkTrue(condition.RabbitMqTransportURLReadyCondition, condition.RabbitMqTransportURLReadyMessage)
	// end transportURL

	configMapVars := make(map[string]env.Setter)

	//
	// check for required OpenStack secret holding passwords for service/admin user and add hash to the vars map
	//
	ctrlResult, err := r.getSecret(ctx, helper, instance, instance.Spec.Aodh.Secret, &configMapVars)
	if err != nil {
		return ctrlResult, err
	}
	// run check OpenStack secret - end

	//
	// check for required TransportURL secret holding transport URL string
	//
	ctrlResult, err = r.getSecret(ctx, helper, instance, instance.Status.TransportURLSecret, &configMapVars)
	if err != nil {
		return ctrlResult, err
	}
	// run check TransportURL secret - end

	//
	// create Configmap required for autoscaling input
	// - %-scripts configmap holding scripts to e.g. bootstrap the service
	// - %-config configmap holding minimal autoscaling config required to get the service up, user can add additional files to be added to the service
	// - parameters which has passwords gets added from the OpenStack secret via the init container
	//
	err = r.generateServiceConfigMaps(ctx, helper, instance, &configMapVars)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.ServiceConfigReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.ServiceConfigReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}

	inputHash, err := r.createHashOfInputHashes(ctx, instance, configMapVars)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Handle service init
	ctrlResult, err = r.reconcileInit(ctx, instance, helper, serviceLabels)
	if err != nil {
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	promResult, err := r.reconcilePrometheus(ctx, instance, helper, serviceLabels)
	if err != nil {
		return promResult, err
	}

	deplDef, err := autoscaling.AodhDeployment(instance, inputHash, serviceLabels)
	if err != nil {
		return ctrl.Result{}, err
	}
	depl := deployment.NewDeployment(
		deplDef,
		time.Duration(5)*time.Second,
	)

	ctrlResult, err = depl.CreateOrPatch(ctx, helper)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DeploymentReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.DeploymentReadyErrorMessage,
			err.Error()))
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DeploymentReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.DeploymentReadyRunningMessage))
		return ctrlResult, nil
	}

	err = controllerutil.SetControllerReference(instance, deplDef, r.Scheme)
	if err != nil {
		return ctrl.Result{}, err
	}

	instance.Status.ReadyCount = depl.GetDeployment().Status.ReadyReplicas
	if instance.Status.ReadyCount > 0 {
		instance.Status.Conditions.MarkTrue(condition.DeploymentReadyCondition, condition.DeploymentReadyMessage)
	}
	instance.Status.Networks = instance.Spec.Aodh.NetworkAttachmentDefinitions

	//	service, _, err = autoscaling.Service(instance, helper, autoscaling.AodhApiPort, serviceLabels)
	//	if err != nil {
	//		return ctrl.Result{}, err
	//	}

	ports := map[endpoint.Endpoint]endpoint.Data{}
	ports[endpoint.EndpointInternal] = endpoint.Data{
		Port: autoscaling.AodhApiPort,
	}

	apiEndpoints, ctrlResult, err := endpoint.ExposeEndpoints(
		ctx,
		helper,
		autoscaling.ServiceName,
		serviceLabels,
		ports,
		time.Duration(5)*time.Second,
	)
	if err != nil {
		return ctrlResult, err
	}

	//
	// create keystone endpoints
	//

	ksEndpointSpec := keystonev1.KeystoneEndpointSpec{
		ServiceName: autoscaling.ServiceName,
		Endpoints:   apiEndpoints,
	}

	ksSvc := keystonev1.NewKeystoneEndpoint(instance.Name, instance.Namespace, ksEndpointSpec, serviceLabels, time.Duration(10)*time.Second)
	ctrlResult, err = ksSvc.CreateOrPatch(ctx, helper)
	if err != nil {
		return ctrlResult, err
	}

	if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	r.Log.Info("Reconciled Service successfully")
	return ctrl.Result{}, nil
}

func (r *AutoscalingReconciler) reconcilePrometheus(ctx context.Context,
	instance *telemetryv1.Autoscaling,
	helper *helper.Helper,
	serviceLabels map[string]string,
) (ctrl.Result, error) {
	prom := autoscaling.Prometheus(instance, serviceLabels)

	var promHost string
	var promPort int32

	if instance.Spec.Prometheus.DeployPrometheus {
		op, err := controllerutil.CreateOrUpdate(ctx, r.Client, prom, func() error {
			err := controllerutil.SetControllerReference(instance, prom, r.Scheme)
			return err
		})
		if err != nil {
			return ctrl.Result{}, err
		}
		if op != controllerutil.OperationResultNone {
			r.Log.Info(fmt.Sprintf("Prometheus %s successfully reconciled - operation: %s", prom.Name, string(op)))
		}
		promReady := true
		for _, c := range prom.Status.Conditions {
			if c.Status != "True" {
				instance.Status.Conditions.MarkFalse("PrometheusReady",
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
			instance.Status.Conditions.MarkTrue("PrometheusReady", "Prometheus is ready")
			serviceName := prom.Name + "-prometheus"
			promSvc, err := service.GetServiceWithName(ctx, helper, serviceName, instance.Namespace)
			if err != nil {
				return ctrl.Result{}, err
			}
			// TODO: Remove the nolint after adding aodh and using the variables
			promHost = fmt.Sprintf("%s.%s.svc", serviceName, instance.Namespace) //nolint:all
			promPort = service.GetServicesPortDetails(promSvc, "web").Port       //nolint:all
		}
	} else {
		err := r.Client.Delete(ctx, prom)
		if err != nil {
			if !k8s_errors.IsNotFound(err) {
				return ctrl.Result{}, nil
			}
		}
		promHost = instance.Spec.Prometheus.Host
		promPort = instance.Spec.Prometheus.Port
		if promHost == "" || promPort == 0 {
			instance.Status.Conditions.MarkFalse("PrometheusReady",
				condition.ErrorReason,
				condition.SeverityError,
				"deployPrometheus is false and either port or host isn't set")
		} else {
			instance.Status.Conditions.MarkTrue("PrometheusReady", "Prometheus is ready")
		}
	}

	// TODO: Pass the promHost and promPort variables to aodh

	return ctrl.Result{}, nil
}

// createHashOfInputHashes - creates a hash of hashes which gets added to the resources which requires a restart
// if any of the input resources change, like configs, passwords, ...
//
// returns the hash and any error
func (r *AutoscalingReconciler) createHashOfInputHashes(
	ctx context.Context,
	instance *telemetryv1.Autoscaling,
	envVars map[string]env.Setter,
) (string, error) {
	var hashMap map[string]string
	changed := false
	mergedMapVars := env.MergeEnvs([]corev1.EnvVar{}, envVars)
	hash, err := util.ObjectHash(mergedMapVars)
	if err != nil {
		return hash, err
	}
	if hashMap, changed = util.SetHash(instance.Status.Hash, common.InputHashName, hash); changed {
		instance.Status.Hash = hashMap
		r.Log.Info(fmt.Sprintf("Input maps hash %s - %s", common.InputHashName, hash))
	}
	return hash, nil
}

func (r *AutoscalingReconciler) generateServiceConfigMaps(
	ctx context.Context,
	h *helper.Helper,
	instance *telemetryv1.Autoscaling,
	envVars *map[string]env.Setter,
) error {

	cmLabels := labels.GetLabels(instance, labels.GetGroupLabel(autoscaling.ServiceName), map[string]string{})
	customData := map[string]string{common.CustomServiceConfigFileName: instance.Spec.Aodh.CustomServiceConfig}
	for key, data := range instance.Spec.Aodh.DefaultConfigOverwrite {
		customData[key] = data
	}

	keystoneAPI, err := keystonev1.GetKeystoneAPI(ctx, h, instance.Namespace, map[string]string{})
	if err != nil {
		return err
	}

	keystoneInternalURL, err := keystoneAPI.GetEndpoint(endpoint.EndpointInternal)
	if err != nil {
		return err
	}

	ospSecret, _, err := secret.GetSecret(ctx, h, instance.Spec.Aodh.Secret, instance.Namespace)
	if err != nil {
		return err
	}

	transportURLSecret, _, err := secret.GetSecret(ctx, h, instance.Status.TransportURLSecret, instance.Namespace)
	if err != nil {
		return err
	}

	templateParameters := map[string]interface{}{
		"KeystoneInternalURL": keystoneInternalURL,
		"TransportURL":        string(transportURLSecret.Data["transport_url"]),
		"DatabaseConnection": fmt.Sprintf("mysql+pymysql://%s:%s@%s/%s",
			instance.Spec.Aodh.DatabaseUser,
			string(ospSecret.Data[instance.Spec.Aodh.PasswordSelectors.Database]),
			instance.Status.DatabaseHostname,
			autoscaling.DatabaseName),
	}

	cms := []util.Template{
		// ScriptsConfigMap
		{
			Name:               fmt.Sprintf("%s-scripts", autoscaling.ServiceName),
			Namespace:          instance.Namespace,
			Type:               util.TemplateTypeScripts,
			InstanceType:       instance.Kind,
			AdditionalTemplate: map[string]string{"common.sh": "/common/common.sh"},
			Labels:             cmLabels,
		},
		// ConfigMap
		{
			Name:          fmt.Sprintf("%s-config-data", autoscaling.ServiceName),
			Namespace:     instance.Namespace,
			Type:          util.TemplateTypeConfig,
			InstanceType:  instance.Kind,
			CustomData:    customData,
			ConfigOptions: templateParameters,
			Labels:        cmLabels,
		},
	}
	return configmap.EnsureConfigMaps(ctx, h, instance, cms, envVars)
}

// getSecret - get the specified secret, and add its hash to envVars
func (r *AutoscalingReconciler) getSecret(ctx context.Context, h *helper.Helper, instance *telemetryv1.Autoscaling, secretName string, envVars *map[string]env.Setter) (ctrl.Result, error) {
	secret, hash, err := secret.GetSecret(ctx, h, secretName, instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			instance.Status.Conditions.Set(condition.FalseCondition(
				condition.InputReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				condition.InputReadyWaitingMessage))
			return ctrl.Result{RequeueAfter: time.Duration(10) * time.Second}, fmt.Errorf("secret %s not found", secretName)
		}
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.InputReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.InputReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}

	// Add a prefix to the var name to avoid accidental collision with other non-secret
	// vars. The secret names themselves will be unique.
	(*envVars)["secret-"+secret.Name] = env.SetValue(hash)

	return ctrl.Result{}, nil
}

func (r *AutoscalingReconciler) transportURLCreateOrUpdate(instance *telemetryv1.Autoscaling) (*rabbitmqv1.TransportURL, controllerutil.OperationResult, error) {
	transportURL := &rabbitmqv1.TransportURL{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-transport", autoscaling.ServiceName),
			Namespace: instance.Namespace,
		},
	}
	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, transportURL, func() error {
		transportURL.Spec.RabbitmqClusterName = instance.Spec.Aodh.RabbitMqClusterName
		err := controllerutil.SetControllerReference(instance, transportURL, r.Scheme)
		return err
	})
	return transportURL, op, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *AutoscalingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// transportURLSecretFn - Watch for changes made to the secret associated with the RabbitMQ
	// TransportURL created and used by Autoscaling CRs.  Watch functions return a list of namespace-scoped
	// CRs that then get fed  to the reconciler.  Hence, in this case, we need to know the name of the
	// Autoscaling CR associated with the secret we are examining in the function.  We could parse the name
	// out of the "%s-transport" secret label, which would be faster than getting the list of
	// the Autoscaling CRs and trying to match on each one.  The downside there, however, is that technically
	// someone could randomly label a secret "something-transport" where "something" actually
	// matches the name of an existing Autoscaling CR.  In that case changes to that secret would trigger
	// reconciliation for a Autoscaling CR that does not need it.
	//
	// TODO: We also need a watch func to monitor for changes to the secret referenced by Autoscaling.Spec.Secret
	transportURLSecretFn := func(o client.Object) []reconcile.Request {
		result := []reconcile.Request{}

		// get all Autoscaling CRs
		autoscalings := &telemetryv1.AutoscalingList{}
		listOpts := []client.ListOption{
			client.InNamespace(o.GetNamespace()),
		}
		if err := r.Client.List(context.Background(), autoscalings, listOpts...); err != nil {
			r.Log.Error(err, "Unable to retrieve Autoscaling CRs %v")
			return nil
		}

		for _, ownerRef := range o.GetOwnerReferences() {
			if ownerRef.Kind == "TransportURL" {
				for _, cr := range autoscalings.Items {
					if ownerRef.Name == fmt.Sprintf("%s-transport", cr.Name) {
						// return namespace and Name of CR
						name := client.ObjectKey{
							Namespace: o.GetNamespace(),
							Name:      cr.Name,
						}
						r.Log.Info(fmt.Sprintf("TransportURL Secret %s belongs to TransportURL belonging to Autoscaling CR %s", o.GetName(), cr.Name))
						result = append(result, reconcile.Request{NamespacedName: name})
					}
				}
			}
		}
		if len(result) > 0 {
			return result
		}
		return nil
	}
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apiextensions.k8s.io",
		Kind:    "CustomResourceDefinition",
		Version: "v1",
	})
	err := r.Client.Get(context.Background(), client.ObjectKey{
		Name: "monitoringstacks.monitoring.rhobs",
	}, u)
	if err != nil {
		return ctrl.NewControllerManagedBy(mgr).
			For(&telemetryv1.Autoscaling{}).
			Owns(&keystonev1.KeystoneService{}).
			Owns(&appsv1.Deployment{}).
			Owns(&corev1.ConfigMap{}).
			Owns(&corev1.Service{}).
			Owns(&corev1.Secret{}).
			Owns(&mariadbv1.MariaDBDatabase{}).
			Owns(&rabbitmqv1.TransportURL{}).
			Owns(&corev1.ServiceAccount{}).
			Owns(&rbacv1.Role{}).
			Owns(&rbacv1.RoleBinding{}).
			// Watch for TransportURL Secrets which belong to any TransportURLs created by Autoscaling CRs
			Watches(&source.Kind{Type: &corev1.Secret{}},
				handler.EnqueueRequestsFromMapFunc(transportURLSecretFn)).
			Complete(r)
	}
	// There is OBO installed and we can own MonitoringStack
	return ctrl.NewControllerManagedBy(mgr).
		For(&telemetryv1.Autoscaling{}).
		Owns(&obov1.MonitoringStack{}).
		Owns(&keystonev1.KeystoneService{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Owns(&mariadbv1.MariaDBDatabase{}).
		Owns(&rabbitmqv1.TransportURL{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		// Watch for TransportURL Secrets which belong to any TransportURLs created by Autoscaling CRs
		Watches(&source.Kind{Type: &corev1.Secret{}},
			handler.EnqueueRequestsFromMapFunc(transportURLSecretFn)).
		Complete(r)
}
