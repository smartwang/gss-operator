/*
Copyright 2023.

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

package controller

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	_ "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	devxingzheaicnv1alpha1 "github/smartwang/gss-operator/api/v1alpha1"

	gamekruiseiov1alpha1 "github.com/openkruise/kruise-game/apis/v1alpha1"

	agres "github.com/CloudNativeGame/aigc-gateway/pkg/resources"
)

var log = logf.Log.WithName("scale_handler")

// GssScalerReconciler reconciles a GssScaler object
type GssScalerReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	ScaleLoopContexts *sync.Map
}

//+kubebuilder:rbac:groups=dev.xingzheai.cn.dev.xingzheai.cn,resources=gssscalers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dev.xingzheai.cn.dev.xingzheai.cn,resources=gssscalers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dev.xingzheai.cn.dev.xingzheai.cn,resources=gssscalers/finalizers,verbs=update

// +kubebuilder:rbac:groups=game.kruise.io,resources=gameservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=game.kruise.io,resources=gameservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=game.kruise.io,resources=gameserversets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=game.kruise.io,resources=gameserversets/status,verbs=get;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the GssScaler object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *GssScalerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	gssScaler := &devxingzheaicnv1alpha1.GssScaler{}
	err := r.Client.Get(ctx, req.NamespacedName, gssScaler)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get gssScaler")
		return ctrl.Result{}, err
	}

	log.Info("Reconciling gssScaler")
	ctx, cancel := context.WithCancel(ctx)

	// cancel the outdated ScaleLoop for the same gssScaler (if exists)
	value, loaded := r.ScaleLoopContexts.LoadOrStore(gssScaler.UID, cancel)
	if loaded {
		// cancel the old loop
		cancelValue, ok := value.(context.CancelFunc)
		if ok {
			cancelValue()
		}
		r.ScaleLoopContexts.Store(gssScaler.UID, cancel)
	}
	if gssScaler.IsBeingDeleted() {
		log.Info(fmt.Sprintf("HandleFinalizer for %v", req.NamespacedName))
		if err := r.handleFinalizer(ctx, gssScaler); err != nil {
			return ctrl.Result{}, fmt.Errorf("error when handling finalizer: %v", err)
		}
		log.Info("Handling delete event")
		cancel()
		return ctrl.Result{}, nil
	}

	if !gssScaler.HasFinalizer(devxingzheaicnv1alpha1.FinalizerName) {
		log.Info(fmt.Sprintf("AddFinalizer for %v", req.NamespacedName))
		if err := r.addFinalizer(gssScaler); err != nil {
			return ctrl.Result{}, fmt.Errorf("error when adding finalizer: %v", err)
		}
		return ctrl.Result{}, nil
	}

	go r.startScaleLoop(ctx, gssScaler, &sync.Mutex{})

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GssScalerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&devxingzheaicnv1alpha1.GssScaler{}).
		Complete(r)
}

// startScaleLoop blocks forever and checks the scalableObject based on its pollingInterval
func (r *GssScalerReconciler) startScaleLoop(ctx context.Context, gssScaler *devxingzheaicnv1alpha1.GssScaler, scalingMutex sync.Locker) {
	logger := log.WithValues("namespace", gssScaler.Namespace, "name", gssScaler.Name)

	pollingInterval := time.Second * time.Duration(*gssScaler.Spec.PollingInterval)

	logger.V(1).Info("Watching with pollingInterval", "PollingInterval", pollingInterval)

	for {
		tmr := time.NewTimer(pollingInterval)
		r.runScale(ctx, gssScaler, scalingMutex)
		select {
		case <-tmr.C:
			tmr.Stop()
		case <-ctx.Done():
			logger.V(1).Info("Context canceled")
			tmr.Stop()
			return
		}
	}
}

func (r *GssScalerReconciler) runScale(ctx context.Context, gssScaler *devxingzheaicnv1alpha1.GssScaler, scalingMutex sync.Locker) (err error) {
	// Find all gs belong to gss with opsStats=='WaitToBeDeleted' and delete them
	log.Info("running scale logic for gss: " + gssScaler.Name)
	gsList := &gamekruiseiov1alpha1.GameServerList{}
	err = r.List(context.Background(), gsList, &client.ListOptions{
		Namespace:     gssScaler.Spec.Namespace,
		LabelSelector: labels.SelectorFromSet(map[string]string{"game.kruise.io/owner-gss": gssScaler.Spec.Name}),
	})
	if err != nil {
		log.Error(err, err.Error())
		return err
	}
	rm := agres.NewResourceManager()
	for _, gs := range gsList.Items {
		log.Info(fmt.Sprintf("Found gs: %s belong to gss", gs.Name))
		if gs.Spec.OpsState == gamekruiseiov1alpha1.WaitToDelete && time.Since(gs.Status.LastTransitionTime.Time) > time.Duration(*gssScaler.Spec.StabilizationWindowSeconds) {
			parts := strings.Split(gs.Name, "-")
			id := parts[len(parts)-1]
			log.Info(fmt.Sprintf("Found gs: ID=%s belong to gss waitToDelete, pause it now", id))
			err = rm.PauseResource(&agres.ResourceMeta{Name: gssScaler.Spec.Name, Namespace: gssScaler.Spec.Namespace, ID: id})
			if err != nil {
				log.Error(err, err.Error())
			}
		}
	}
	return
}

func (r *GssScalerReconciler) addFinalizer(gssScaler *devxingzheaicnv1alpha1.GssScaler) error {
	gssScaler.AddFinalizer(devxingzheaicnv1alpha1.FinalizerName)
	return r.Update(context.Background(), gssScaler)
}

func (r *GssScalerReconciler) handleFinalizer(ctx context.Context, gssScaler *devxingzheaicnv1alpha1.GssScaler) error {
	if !gssScaler.HasFinalizer(devxingzheaicnv1alpha1.FinalizerName) {
		return nil
	}

	gssScaler.RemoveFinalizer(devxingzheaicnv1alpha1.FinalizerName)
	return r.Update(context.Background(), gssScaler)
}
