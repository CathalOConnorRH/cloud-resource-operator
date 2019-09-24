package postgres

import (
	"context"
	"fmt"
	"time"

	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/integr8ly/cloud-resource-operator/pkg/providers/openshift"

	"github.com/integr8ly/cloud-resource-operator/pkg/providers"
	"github.com/sirupsen/logrus"

	integreatlyv1alpha1 "github.com/integr8ly/cloud-resource-operator/pkg/apis/integreatly/v1alpha1"
	errorUtil "github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_postgres")

// Add creates a new Postgres Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcilePostgres{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("postgres-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Postgres
	err = c.Watch(&source.Kind{Type: &integreatlyv1alpha1.Postgres{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Pods and requeue the owner Postgres
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &integreatlyv1alpha1.Postgres{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcilePostgres implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcilePostgres{}

// ReconcilePostgres reconciles a Postgres object
type ReconcilePostgres struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Postgres object and makes changes based on the state read
// and what is in the Postgres.Spec
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcilePostgres) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.TODO()
	logger := logrus.WithFields(logrus.Fields{"controller": "controller_postgres"})
	providerList := []providers.PostgresProvider{openshift.NewOpenShiftPostgresProvider(r.client, logger)}
	cfgMgr := providers.NewConfigManager(providers.DefaultProviderConfigMapName, providers.DefaultConfigNamespace, r.client)

	logger.Info("Reconciling Postgres")

	// Fetch the Postgres instance
	instance := &integreatlyv1alpha1.Postgres{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	stratMap, err := cfgMgr.GetStrategyMappingForDeploymentType(ctx, instance.Spec.Type)
	if err != nil {
		return reconcile.Result{}, errorUtil.Wrapf(err, "failed to read deployment type config for deployment %s", instance.Spec.Type)
	}
	for _, p := range providerList {
		if !p.SupportsStrategy(stratMap.Postgres) {
			continue
		}

		// delete the postgres if the deletion timestamp exists
		if instance.DeletionTimestamp != nil {
			err := p.DeletePostgres(ctx, instance)
			if err != nil {
				return reconcile.Result{}, errorUtil.Wrapf(err, "failed to perform provider-specific storage deletion")
			}
			return reconcile.Result{}, nil
		}

		ps, err := p.CreatePostgres(ctx, instance)
		if err != nil {
			return reconcile.Result{}, err
		}
		if ps == nil {
			return reconcile.Result{}, errorUtil.New("secret data is still reconciling")
		}

		// return the connection secret
		sec := &corev1.Secret{
			ObjectMeta: controllerruntime.ObjectMeta{
				Name:      instance.Spec.SecretRef.Name,
				Namespace: instance.Namespace,
			},
		}
		_, err = controllerruntime.CreateOrUpdate(ctx, r.client, sec, func(existing runtime.Object) error {
			e := existing.(*corev1.Secret)
			if err = controllerutil.SetControllerReference(instance, e, r.scheme); err != nil {
				return errorUtil.Wrapf(err, "failed to set owner on secret %s", sec.Name)
			}
			e.Data = ps.DeploymentDetails.Data()
			e.Type = corev1.SecretTypeOpaque
			return nil
		})
		if err != nil {
			return reconcile.Result{}, errorUtil.Wrapf(err, "failed to reconcile postgres instance secret %s", sec.Name)
		}

		instance.Status.SecretRef = instance.Spec.SecretRef
		instance.Status.Strategy = stratMap.Postgres
		instance.Status.Provider = p.GetName()
		if err = r.client.Status().Update(ctx, instance); err != nil {
			return reconcile.Result{}, errorUtil.Wrapf(err, "failed to update instance %s in namespace %s", instance.Name, instance.Namespace)
		}
		return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 30}, nil
	}

	return reconcile.Result{}, errorUtil.New(fmt.Sprintf("unsupported deployment strategy %s", stratMap.Postgres))
}
