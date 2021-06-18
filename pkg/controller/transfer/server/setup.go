/*
Copyright 2019 The Crossplane Authors.

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

package server

import (
	"context"

	svcsdk "github.com/aws/aws-sdk-go/service/transfer"
	svcapitypes "github.com/crossplane/provider-aws/apis/transfer/v1alpha1"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	aws "github.com/crossplane/provider-aws/pkg/clients"
)

// SetupServer adds a controller that reconciles Server.
func SetupServer(mgr ctrl.Manager, l logging.Logger, rl workqueue.RateLimiter) error {
	name := managed.ControllerName(svcapitypes.ServerGroupKind)

	opts := []option{
		func(e *external) {
			e.postObserve = postObserve
		},
	}
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(controller.Options{
			RateLimiter: ratelimiter.NewDefaultManagedRateLimiter(rl),
		}).
		For(&svcapitypes.Server{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(svcapitypes.ServerGroupVersionKind),
			managed.WithExternalConnecter(&connector{kube: mgr.GetClient(), opts: opts}),
			managed.WithLogger(l.WithValues("controller", name)),
			managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name)))))
}

func postObserve(_ context.Context, cr *svcapitypes.Server, resp *svcsdk.DescribeServerOutput, obs managed.ExternalObservation, err error) (managed.ExternalObservation, error) {
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	switch aws.StringValue(resp.Server.State) {
	case string(svcapitypes.State_OFFLINE):
		cr.SetConditions(xpv1.Unavailable())
	case string(svcapitypes.State_ONLINE):
		cr.SetConditions(xpv1.Available())
	case string(svcapitypes.State_STARTING):
		cr.SetConditions(xpv1.Creating())
	case string(svcapitypes.State_STOPPING):
		cr.SetConditions(xpv1.Deleting())
	case string(svcapitypes.State_START_FAILED):
		cr.SetConditions(xpv1.ReconcileError(err))
	case string(svcapitypes.State_STOP_FAILED):
		cr.SetConditions(xpv1.ReconcileError(err))
	}
	return obs, nil
}
