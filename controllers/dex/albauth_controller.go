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
	"fmt"

	dexv1 "github.com/BetssonGroup/dex-operator/apis/dex/v1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ALBAuthReconciler reconciles a ALBAuth object
type ALBAuthReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=dex.betssongroup.com,resources=albauths,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dex.betssongroup.com,resources=albauths/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=create;list;watch;delete
// +kubebuilder:rbac:groups="extensions",resources=ingresses,verbs=list;watch;update;patch

// Reconcile reconciles ALB oidc
func (r *ALBAuthReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("albauth", req.NamespacedName)

	dexv1ALBAuth := &dexv1.ALBAuth{}
	if err := r.Get(ctx, req.NamespacedName, dexv1ALBAuth); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Delete
	albFinalizer := "albauth.dex.finalizers.betssongroup.com"
	// examine DeletionTimestamp to determine if object is under deletion
	if dexv1ALBAuth.ObjectMeta.DeletionTimestamp.IsZero() {
		if !containsString(dexv1ALBAuth.ObjectMeta.Finalizers, albFinalizer) {
			// append our finalizer if state is active
			if dexv1ALBAuth.Status.State == dexv1.PhaseActive {
				log.Info("Adding alb finalizer")
				dexv1ALBAuth.ObjectMeta.Finalizers = append(dexv1ALBAuth.ObjectMeta.Finalizers, albFinalizer)
				if err := r.Update(ctx, dexv1ALBAuth); err != nil {
					return ctrl.Result{}, err
				}
			}
		}
	} else {
		if containsString(dexv1ALBAuth.ObjectMeta.Finalizers, albFinalizer) {
			dexv1ALBAuth.Status.State = dexv1.PhaseDeleting
		}

	}

	// If the ALBAuth is already setup return
	if dexv1ALBAuth.Status.State == dexv1.PhaseActive {
		return ctrl.Result{}, nil
	}
	// Get the client
	dexv1Client := &dexv1.Client{}
	namespacedClientName := k8stypes.NamespacedName{
		Name:      dexv1ALBAuth.Spec.Client,
		Namespace: dexv1ALBAuth.Namespace,
	}
	if err := r.Get(ctx, namespacedClientName, dexv1Client); err != nil {
		dexv1ALBAuth.Status.State = dexv1.PhaseFailed
		log.Info("Client not found", "client", dexv1ALBAuth.Spec.Client)
		if err := r.Update(ctx, dexv1ALBAuth); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, err
	}

	// Reconcile the secret
	secret, err := r.reconcileSecret(ctx, dexv1ALBAuth, dexv1Client)
	if err != nil {
		log.Error(err, "unable to reconcile secret", "client", dexv1Client.Name)
		return ctrl.Result{}, err
	}
	// Set controller reference
	if err := ctrl.SetControllerReference(dexv1ALBAuth, secret, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	// Set status
	dexv1ALBAuth.Status.Secret = corev1.ObjectReference{
		Kind:      secret.Kind,
		Namespace: secret.Namespace,
		Name:      secret.Name,
	}

	// Reconcile the ingress
	ingress, err := r.reconcileIngress(ctx, dexv1ALBAuth, dexv1Client)
	if err != nil {
		log.Error(err, "unable to reconcile ingress", "client", dexv1Client.Name)
		if client.IgnoreNotFound(err) == nil {
			// ingress not found, set phase to not found
			dexv1ALBAuth.Status.State = dexv1.PhaseFailed
			if err := r.Update(ctx, dexv1ALBAuth); err != nil {
				return ctrl.Result{}, err
			}
		} else {
			return ctrl.Result{}, err
		}
	}
	if dexv1ALBAuth.Status.State == dexv1.PhaseDeleting || dexv1ALBAuth.Status.State == dexv1.PhaseFailed {
		// Remove our finalizer since we cleaned up
		dexv1ALBAuth.ObjectMeta.Finalizers = removeString(dexv1ALBAuth.ObjectMeta.Finalizers, albFinalizer)
	} else {
		// Set status, make it active
		dexv1ALBAuth.Status.Ingress = corev1.ObjectReference{
			Kind:      ingress.Kind,
			Namespace: ingress.Namespace,
			Name:      ingress.Name,
		}
		dexv1ALBAuth.Status.State = dexv1.PhaseActive
	}
	err = r.Update(ctx, dexv1ALBAuth)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *ALBAuthReconciler) reconcileSecret(ctx context.Context, dexv1ALBAuth *dexv1.ALBAuth, dexv1Client *dexv1.Client) (*corev1.Secret, error) {
	// Check if secret exists and is up to date
	namespacedName := k8stypes.NamespacedName{
		Name:      fmt.Sprintf("alb-secret-%s", dexv1Client.Name),
		Namespace: dexv1ALBAuth.Namespace,
	}
	secret := &corev1.Secret{}
	if err := r.Get(ctx, namespacedName, secret); err != nil {
		// No secret found, create it
		if client.IgnoreNotFound(err) == nil {
			newSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      make(map[string]string),
					Annotations: make(map[string]string),
					Name:        fmt.Sprintf("alb-secret-%s", dexv1Client.Name),
					Namespace:   dexv1Client.Namespace,
				},
				StringData: map[string]string{
					"clientId":     dexv1Client.Name,
					"clientSecret": dexv1Client.Spec.Secret,
				},
			}
			if err := r.Create(ctx, newSecret); err != nil {
				return nil, err
			}
			return newSecret, nil
		}
		return nil, err
	}
	return secret, nil
}

func (r *ALBAuthReconciler) reconcileIngress(ctx context.Context, dexv1ALBAuth *dexv1.ALBAuth, dexv1Client *dexv1.Client) (*extensionsv1beta1.Ingress, error) {
	log := r.Log.WithValues("albauth", dexv1ALBAuth.Name)
	ingress := &extensionsv1beta1.Ingress{}
	namespacedIngressName := k8stypes.NamespacedName{
		Name:      dexv1ALBAuth.Spec.Ingress,
		Namespace: dexv1ALBAuth.Namespace,
	}
	if err := r.Get(ctx, namespacedIngressName, ingress); err != nil {
		log.Error(err, "unable to find ingress", "client", dexv1Client.Name)
		return nil, err
	}
	// check if it is an ALB ingress
	if !mapContains(ingress.Annotations, "kubernetes.io/ingress.class", "alb") {
		return nil, nil
	}
	// alb.ingress.kubernetes.io/auth-type: oidc
	// alb.ingress.kubernetes.io/auth-idp-oidc: '{"Issuer":"https://albingress.auth0.com/","AuthorizationEndpoint":"https://albingress.auth0.com/authorize","TokenEndpoint":"https://albingress.auth0.com/oauth/token","UserInfoEndpoint":"https://albingress.auth0.com/userinfo","SecretName":"odic-secret"}'
	authIdpOidc := fmt.Sprintf(
		`{"Issuer":"%s","AuthorizationEndpoint":"%s","TokenEndpoint":"%s","UserInfoEndpoint":"%s","SecretName":"%s"}`,
		dexv1ALBAuth.Spec.Issuer,
		fmt.Sprintf("%s/auth", dexv1ALBAuth.Spec.Issuer),
		fmt.Sprintf("%s/token", dexv1ALBAuth.Spec.Issuer),
		fmt.Sprintf("%s/userinfo", dexv1ALBAuth.Spec.Issuer),
		fmt.Sprintf("alb-secret-%s", dexv1Client.Name),
	)
	neededAnnotations := map[string]string{
		"alb.ingress.kubernetes.io/auth-idp-oidc":                   authIdpOidc,
		"alb.ingress.kubernetes.io/auth-type":                       "oidc",
		"alb.ingress.kubernetes.io/auth-on-unauthenticated-request": "authenticate",
	}
	// The object is being deleted, remove annotations

	currentAnnotations := ingress.GetAnnotations()
	if dexv1ALBAuth.ObjectMeta.DeletionTimestamp.IsZero() {
		annotations, err := makeAnnotations(currentAnnotations, neededAnnotations, false)
		if err != nil {
			return nil, err
		}
		ingress.SetAnnotations(annotations)
		// Update the objects
		err = r.Update(ctx, ingress)
		if err != nil {
			return nil, err
		}
		return ingress, nil
	}
	// object is being deleted
	annotations, err := makeAnnotations(currentAnnotations, neededAnnotations, true)
	ingress.SetAnnotations(annotations)
	// Update the objects
	err = r.Update(ctx, ingress)
	if err != nil {
		return nil, err
	}
	return ingress, nil
}

// SetupWithManager sets up the mananager
func (r *ALBAuthReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dexv1.ALBAuth{}).
		Complete(r)
}

func makeAnnotations(currentAnnotations map[string]string, neededAnnotations map[string]string, deleting bool) (map[string]string, error) {
	// loop through needed annotations, add those that are not in current.
	for k, v := range neededAnnotations {
		if !deleting {
			if !mapContains(currentAnnotations, k, v) {
				currentAnnotations[k] = v
			}
		} else {
			if mapContains(currentAnnotations, k, v) {
				delete(currentAnnotations, k)
			}
		}
	}
	return currentAnnotations, nil
}

func mapContains(current map[string]string, key string, value string) bool {
	for k, v := range current {
		if k == key && v == value {
			return true
		}
	}
	return false
}
