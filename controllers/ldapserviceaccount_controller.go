/*
Copyright 2024.

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
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-ldap/ldap"
	ldapserviceaccountv1 "github.com/lajosbencz/k8s-sa-ldap/api/v1"
)

const commonLabelKey = "ldapserviceaccount.lazos.me"
const commonLabelValue = "ldapserviceaccount"

// LdapServiceAccountReconciler reconciles a LdapServiceAccount object
type LdapServiceAccountReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *LdapServiceAccountReconciler) getServiceAccountToken(ctx context.Context, username, namespace string) (string, error) {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      username,
			Namespace: namespace,
		},
	}

	if err := r.Client.Create(ctx, serviceAccount); err != nil {
		if client.IgnoreAlreadyExists(err) != nil {
			return "", err
		}
	}

	if len(serviceAccount.Secrets) == 0 {
		return "", fmt.Errorf("service account has no secrets")
	}

	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: serviceAccount.Secrets[0].Name, Namespace: namespace}, secret); err != nil {
		return "", err
	}

	token, ok := secret.Data["token"]
	if !ok {
		return "", fmt.Errorf("secret does not contain a token")
	}
	return string(token), nil
}

func (r *LdapServiceAccountReconciler) createRole(ctx context.Context, resourcePrefix, username, namespace string) error {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", resourcePrefix, username),
			Namespace: namespace,
			Labels: map[string]string{
				commonLabelKey: commonLabelValue,
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"*"},
				Verbs:     []string{"*"},
				Resources: []string{"*"},
			},
		},
	}

	if err := r.Client.Create(ctx, role); err != nil {
		if client.IgnoreAlreadyExists(err) != nil {
			return err
		}
	}

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-rolebinding-%s", resourcePrefix, username),
			Namespace: namespace,
			Labels: map[string]string{
				commonLabelKey: commonLabelValue,
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      username,
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind: "Role",
			Name: role.Name,
		},
	}

	if err := r.Client.Create(ctx, roleBinding); err != nil {
		if client.IgnoreAlreadyExists(err) != nil {
			return err
		}
	}

	return nil
}

func (r *LdapServiceAccountReconciler) searchLdap(ldapConn *ldap.Conn, ldapSync ldapserviceaccountv1.LdapServiceAccount) (*ldap.SearchResult, error) {
	request := ldap.NewSearchRequest(
		ldapSync.Spec.BaseDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		ldapSync.Spec.Filter,
		[]string{"dn", "cn", "uid", "kubeNamespaces", "kubeToken"},
		nil,
	)

	result, err := ldapConn.Search(request)
	if err != nil {
		log.Log.Error(err, "failed to search on LDAP server")
		return nil, err
	}

	return result, nil
}

//+kubebuilder:rbac:groups=ldapserviceaccount.lazos.me,resources=ldapserviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ldapserviceaccount.lazos.me,resources=ldapserviceaccounts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ldapserviceaccount.lazos.me,resources=ldapserviceaccounts/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *LdapServiceAccountReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	var ldapSync ldapserviceaccountv1.LdapServiceAccount
	if err := r.Get(ctx, req.NamespacedName, &ldapSync); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	interval, err := time.ParseDuration(ldapSync.Spec.PollInterval)
	if err != nil {
		log.Log.Error(err, fmt.Sprintf("invalid spec.pollInterval value: %s, defaulting to 10m", ldapSync.Spec.PollInterval))
		interval = 10 * time.Minute
	}

	ldapConn, err := ldap.DialURL(ldapSync.Spec.URL)
	if err != nil {
		log.Log.Error(err, "failed to connect to LDAP server")
		return ctrl.Result{}, err
	}
	defer ldapConn.Close()

	err = ldapConn.Bind(ldapSync.Spec.BindDN, ldapSync.Spec.BindPW)
	if err != nil {
		log.Log.Error(err, "failed to bind to LDAP server")
		return ctrl.Result{}, err
	}

	ldapResult, err := r.searchLdap(ldapConn, ldapSync)
	if err != nil {
		return ctrl.Result{}, err
	}

	for _, entry := range ldapResult.Entries {
		username := entry.GetAttributeValue("uid")
		namespaces := strings.Split(entry.GetAttributeValue("kubeNamespaces"), ",")

		ldapToken := entry.GetAttributeValue("kubeToken")

		token, err := r.getServiceAccountToken(ctx, username, ldapSync.Namespace)
		if err != nil {
			log.Log.Error(err, "failed to get service account token")
			continue
		}

		if token != ldapToken {
			modifyRequest := ldap.NewModifyRequest(entry.DN, nil)
			modifyRequest.Replace("kubeToken", []string{token})

			if err := ldapConn.Modify(modifyRequest); err != nil {
				log.Log.Error(err, "failed to update kubeToken field in LDAP")
				continue
			}
		}

		for _, namespace := range namespaces {
			if err := r.createRole(ctx, ldapSync.Spec.ResourcePrefix, username, namespace); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	ldapSync.Status.LastSyncTime = metav1.Now()
	if err := r.Status().Update(ctx, &ldapSync); err != nil {
		log.Log.Error(err, "failed to update LDAP server")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: interval}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LdapServiceAccountReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ldapserviceaccountv1.LdapServiceAccount{}).
		Complete(r)
}
