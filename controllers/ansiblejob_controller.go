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

package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	towerv1alpha1 "github.com/ansible/awx-resource-go-operator/api/v1alpha1"
)

// AnsibleJobReconciler reconciles a AnsibleJob object
type AnsibleJobReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=tower.ansible.com,resources=ansiblejobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tower.ansible.com,resources=ansiblejobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tower.ansible.com,resources=ansiblejobs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the AnsibleJob object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *AnsibleJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	ansibleJob := &towerv1alpha1.AnsibleJob{}
	err := r.Get(ctx, req.NamespacedName, ansibleJob)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("AnsibleJob resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get AnsibleJob")
		return ctrl.Result{}, err
	}

	jobFound := &batchv1.Job{}
	err = r.Get(ctx, types.NamespacedName{Name: ansibleJob.Name, Namespace: ansibleJob.Namespace}, jobFound)
	if err != nil && errors.IsNotFound(err) {
		job, err := r.jobForAnsibleJob(ansibleJob)
		if err != nil {
			log.Error(err, "Failed to create Job object")
			return ctrl.Result{}, err
		}

		log.Info("Creating a new Job", "Job.Namespace", job.Namespace, "Job.Name", job.Name)
		if err = r.Create(ctx, job); err != nil {
			log.Error(err, "Failed to create new Job", "Job.Namespace", job.Namespace, "Job.Name", job.Name)
			return ctrl.Result{}, err
		}

		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Job")
		return ctrl.Result{}, err
	}

	podList := &v1.PodList{}
	listOpts := []client.ListOption{client.InNamespace(ansibleJob.Namespace), client.MatchingLabels{"app.kubernetes.io/instance": jobFound.Name}}
	err = r.Client.List(ctx, podList, listOpts...)
	if err != nil {
		log.Error(err, "Failed to get Pod object")
		return ctrl.Result{}, err
	}

	if len(podList.Items) > 0 {
		ansibleJob.Status = podList.Items[0].Status
		if err := r.Status().Update(ctx, ansibleJob); err != nil {
			log.Error(err, "Failed to update AnsibleJob status")
			return ctrl.Result{}, err
		}

		if err := r.Get(ctx, req.NamespacedName, ansibleJob); err != nil {
			log.Error(err, "Failed to re-fetch AnsibleJob")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *AnsibleJobReconciler) jobForAnsibleJob(ansibleJob *towerv1alpha1.AnsibleJob) (*batchv1.Job, error) {
	labels := labelsForAnsibleJob(ansibleJob.Name)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ansibleJob.Name,
			Namespace: ansibleJob.Namespace,
		},
		Spec: batchv1.JobSpec{
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: v1.PodSpec{
					SecurityContext: &v1.PodSecurityContext{
						RunAsNonRoot: &[]bool{true}[0],
						SeccompProfile: &v1.SeccompProfile{
							Type: v1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Containers: []v1.Container{
						{
							Name:    "awx",
							Image:   "quay.io/ansible/awx-ee:latest",
							Command: []string{"/bin/sh"},
							Args:    buildAWXCommand(ansibleJob),
							Env: []v1.EnvVar{
								{
									Name: "TOKEN",
									ValueFrom: &v1.EnvVarSource{
										SecretKeyRef: &v1.SecretKeySelector{
											LocalObjectReference: v1.LocalObjectReference{Name: ansibleJob.Spec.TowerAuthSecret},
											Key:                  "token",
										},
									},
								},
								{
									Name: "HOST",
									ValueFrom: &v1.EnvVarSource{
										SecretKeyRef: &v1.SecretKeySelector{
											LocalObjectReference: v1.LocalObjectReference{Name: ansibleJob.Spec.TowerAuthSecret},
											Key:                  "host",
										},
									},
								},
							},
							SecurityContext: &v1.SecurityContext{
								RunAsNonRoot:             &[]bool{true}[0],
								AllowPrivilegeEscalation: &[]bool{false}[0],
								Capabilities: &v1.Capabilities{
									Drop: []v1.Capability{
										"ALL",
									},
								},
							},
						},
					},
					RestartPolicy: v1.RestartPolicyNever,
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(ansibleJob, job, r.Scheme); err != nil {
		return nil, err
	}
	return job, nil
}

func buildAWXCommand(ansibleJob *towerv1alpha1.AnsibleJob) []string {
	cmd := []string{"-c"}
	bashScript := strings.Builder{}
	fmt.Fprintf(&bashScript, "ID=$(awx --conf.host $HOST --conf.token $TOKEN job_template get '%s' | awk 'NR==2 { print substr($2, 1, length($2)-1) }'); ", ansibleJob.Spec.JobTemplateName)
	bashScript.WriteString("awx --conf.host $HOST --conf.token $TOKEN job_template launch $ID")

	if ansibleJob.Spec.ExtraVars != nil {
		extraVars, _ := json.Marshal(ansibleJob.Spec.ExtraVars)
		bashScript.WriteString(fmt.Sprintf(" --extra_vars '%s'", string(extraVars)))
	}
	if ansibleJob.Spec.Inventory != "" {
		bashScript.WriteString(fmt.Sprintf(" --inventory %s", ansibleJob.Spec.Inventory))
	}

	if ansibleJob.Spec.JobTTL != nil {
		bashScript.WriteString(fmt.Sprintf(" --timeout %s", strconv.Itoa(*ansibleJob.Spec.JobTTL)))
	}

	if ansibleJob.Spec.JobTags != "" {
		bashScript.WriteString(fmt.Sprintf(" --job_tags %s", ansibleJob.Spec.JobTags))
	}

	if ansibleJob.Spec.SkipTags != "" {
		bashScript.WriteString(fmt.Sprintf(" --skip_tags %s", ansibleJob.Spec.SkipTags))
	}

	cmd = append(cmd, bashScript.String())
	return cmd
}

func labelsForAnsibleJob(name string) map[string]string {
	return map[string]string{"app.kubernetes.io/name": "AnsibleJob",
		"app.kubernetes.io/instance":   name,
		"app.kubernetes.io/part-of":    "ansiblejob-operator",
		"app.kubernetes.io/created-by": "controller-manager",
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *AnsibleJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&towerv1alpha1.AnsibleJob{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
