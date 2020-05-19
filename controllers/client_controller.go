/*
Copyright 2020 Betsson Group.

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

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	oauth2v1 "github.com/BetssonGroup/dex-operator/api/v1"
	dexapi "github.com/BetssonGroup/dex-operator/pkg/dex"
)

var (
	clientsCreated = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "client_created_total",
			Help: "Number of clients created",
		},
	)
	clientFailures = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "client_failures_total",
			Help: "Number of failed clients",
		},
	)
)

func init() {
	metrics.Registry.MustRegister(clientsCreated, clientFailures)
}

// ClientReconciler reconciles a Client object
type ClientReconciler struct {
	client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	DexClient *dexapi.APIClient
	Recorder  record.EventRecorder
}

// Reconcile reconciles oauth2 clients in dex
// +kubebuilder:rbac:groups=dex.betssongroup.com,resources=clients,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dex.betssongroup.com,resources=clients/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
func (r *ClientReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("client", req.NamespacedName)

	// Get an oauth2 Client
	oauth2Client := &oauth2v1.Client{}
	if err := r.Get(ctx, req.NamespacedName, oauth2Client); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// deletion finalizer
	dexFinalizer := "client.dex.finalizers.betssongroup.com"

	// Delete
	// examine DeletionTimestamp to determine if object is under deletion
	if oauth2Client.ObjectMeta.DeletionTimestamp.IsZero() {
		if !containsString(oauth2Client.ObjectMeta.Finalizers, dexFinalizer) {
			// append our finalizer
			log.Info("Adding finalizer")
			oauth2Client.ObjectMeta.Finalizers = append(oauth2Client.ObjectMeta.Finalizers, dexFinalizer)
			if err := r.Update(ctx, oauth2Client); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if containsString(oauth2Client.ObjectMeta.Finalizers, dexFinalizer) {
			r.Recorder.Eventf(oauth2Client, "Normal", "ClientDeletion", "client %s", oauth2Client.Name)
			// our finalizer is present, so lets handle any external dependency
			if oauth2Client.Status.State != oauth2v1.PhaseFailed { // It's a failed client, just delete it.
				oauth2Client.Status.State = oauth2v1.PhaseDeleting
				if err := r.DexClient.DeleteClient(ctx, oauth2Client.Name); err != nil {
					oauth2Client.Status.Message = err.Error()
					if err := r.Update(ctx, oauth2Client); err != nil {
						r.Recorder.Eventf(oauth2Client, "Error", "ClientDeletion", "client %s: %s", oauth2Client.Name, err.Error())
						return ctrl.Result{}, err
					}
					return ctrl.Result{}, err
				}
			}
			// remove our finalizer from the list and update it.
			oauth2Client.ObjectMeta.Finalizers = removeString(oauth2Client.ObjectMeta.Finalizers, dexFinalizer)
			if err := r.Update(ctx, oauth2Client); err != nil {
				return ctrl.Result{}, err
			}
		}
		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	// if status is not set, set it to CREATING
	if oauth2Client.Status.State == "" || oauth2Client.Status.State == oauth2v1.PhaseCreating {
		oauth2Client.Status.State = oauth2v1.PhaseCreating
	}
	// Now let's make the main case distinction: implementing
	// the state diagram CREATING -> ACTIVE or CREATING -> FAILED
	switch oauth2Client.Status.State {
	case oauth2v1.PhaseCreating:
		log.Info("Creating dex client", "name", oauth2Client.Name)
		// Implement dex auth client creation here
		res, err := r.DexClient.CreateClient(
			ctx,
			oauth2Client.Spec.RedirectURIs,
			oauth2Client.Spec.TrustedPeers,
			oauth2Client.Spec.Public,
			oauth2Client.Spec.Name,
			oauth2Client.Name,
			oauth2Client.Spec.LogoURL,
			oauth2Client.Spec.Secret,
		)
		if err != nil {
			log.Error(err, "Client create failed", "client", oauth2Client.Name)
			oauth2Client.Status.State = oauth2v1.PhaseFailed
			oauth2Client.Status.Message = err.Error()
			r.Recorder.Eventf(oauth2Client, "Error", "ClientCreation", "client %s: %s", oauth2Client.Name, err.Error())
			clientFailures.Inc()
		} else {
			oauth2Client.Status.State = oauth2v1.PhaseActive
			log.Info("Client created", "client ID", res.GetId())
			r.Recorder.Eventf(oauth2Client, "Normal", "ClientCreation", "client %s", oauth2Client.Name)
			clientsCreated.Inc()
		}
	case oauth2v1.PhaseActive:
		log.Info("Client active")
	case oauth2v1.PhaseFailed:
		log.Info("Client failed")
	default:
		// Should never reach here
		log.Info("Got an invalid state", "state", oauth2Client.Status.State)
		return ctrl.Result{}, nil
	}
	// Update the object and return
	err := r.Update(ctx, oauth2Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *ClientReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&oauth2v1.Client{}).
		Complete(r)
}

// Helper functions to check and remove string from a slice of strings.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}
