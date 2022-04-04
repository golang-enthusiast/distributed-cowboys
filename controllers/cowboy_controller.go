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
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	demov1 "ai.cast/distributed-cowboys/api/v1"
)

// CowboyReconciler reconciles a Cowboy object
type CowboyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=demo.ai.cast,resources=cowboys,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=demo.ai.cast,resources=cowboys/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=demo.ai.cast,resources=cowboys/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Cowboy object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *CowboyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	l.Info("Starting cowboy reconcile", "req", req)
	cowboy := &demov1.Cowboy{}
	err := r.Client.Get(ctx, req.NamespacedName, cowboy)
	if err != nil {
		if errors.IsNotFound(err) {
			l.Info("Resource not found", err.Error())
			return ctrl.Result{}, nil
		}
		l.Error(err, err.Error())
		return ctrl.Result{}, err
	}

	// Check if the deployment exists.
	// If the deployment is not found, we create a new one.
	foundDeployment := &appsv1.Deployment{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: cowboy.Name, Namespace: cowboy.Namespace}, foundDeployment)
	if err != nil && errors.IsNotFound(err) {
		if len(cowboy.Spec.Image) == 0 {
			l.Error(err, "Failed to create deployment, specification error: image required")
			return ctrl.Result{}, err
		}
		if len(cowboy.Spec.Cowboys) == 0 {
			l.Error(err, "Failed to create deployment, specification error: specify at least one cowboy name")
			return ctrl.Result{}, err
		}
		dep := r.createDeployment(cowboy)
		l.Info("Creating a new deployment", "namespace", dep.Namespace, "name", dep.Name)
		err = r.Client.Create(ctx, dep)
		if err != nil {
			l.Error(err, "Failed to create deployment", "namespace", dep.Namespace, "name", dep.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		l.Error(err, err.Error())
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CowboyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&demov1.Cowboy{}).
		Complete(r)
}

func (r *CowboyReconciler) createDeployment(cowboy *demov1.Cowboy) *appsv1.Deployment {
	var replicas int32 = 1
	labels := map[string]string{"app": "distributed-cowboys"}
	containers := make([]corev1.Container, len(cowboy.Spec.Cowboys))
	for i, n := range cowboy.Spec.Cowboys {
		envs := make([]corev1.EnvVar, len(cowboy.Spec.EnvVars)+1)
		for j, env := range cowboy.Spec.EnvVars {
			envs[j] = corev1.EnvVar{
				Name:      env.Name,
				Value:     env.Value,
				ValueFrom: env.ValueFrom,
			}
		}
		envs[len(envs)-1] = corev1.EnvVar{
			Name:  "COWBOY_NAME",
			Value: n,
		}
		containers[i] = corev1.Container{
			Image:           cowboy.Spec.Image,
			ImagePullPolicy: corev1.PullPolicy(cowboy.Spec.ImagePullPolicy),
			Name:            fmt.Sprintf("%s-%s", cowboy.Name, strings.ToLower(n)),
			Env:             envs,
		}
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cowboy.Name,
			Namespace: cowboy.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: containers,
				},
			},
		},
	}
	_ = ctrl.SetControllerReference(cowboy, deployment, r.Scheme)
	return deployment
}
