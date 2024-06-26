/*
Copyright 2018 The Kubernetes Authors.

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

package handler

import (
	"context"

	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// EventHandler enqueues reconcile.Requests in response to events (e.g. Pod Create).  EventHandlers map an Event
// for one object to trigger Reconciles for either the same object or different objects - e.g. if there is an
// Event for object with type Foo (using source.Kind) then reconcile one or more object(s) with type Bar.
//
// Identical reconcile.Requests will be batched together through the queuing mechanism before reconcile is called.
//
// * Use EnqueueRequestForObject to reconcile the object the event is for
// - do this for events for the type the Controller Reconciles. (e.g. Deployment for a Deployment Controller)
//
// * Use EnqueueRequestForOwner to reconcile the owner of the object the event is for
// - do this for events for the types the Controller creates.  (e.g. ReplicaSets created by a Deployment Controller)
//
// * Use EnqueueRequestsFromMapFunc to transform an event for an object to a reconcile of an object
// of a different type - do this for events for types the Controller may be interested in, but doesn't create.
// (e.g. If Foo responds to cluster size events, map Node events to Foo objects.)
//
// Unless you are implementing your own EventHandler, you can ignore the functions on the EventHandler interface.
// Most users shouldn't need to implement their own EventHandler.
type EventHandler TypedEventHandler[client.Object]

// TypedEventHandler enqueues reconcile.Requests in response to events (e.g. Pod Create). TypedEventHandlers map an Event
// for one object to trigger Reconciles for either the same object or different objects - e.g. if there is an
// Event for object with type Foo (using source.Kind) then reconcile one or more object(s) with type Bar.
//
// Identical reconcile.Requests will be batched together through the queuing mechanism before reconcile is called.
//
// * Use TypedEnqueueRequestForObject to reconcile the object the event is for
// - do this for events for the type the Controller Reconciles. (e.g. Deployment for a Deployment Controller)
//
// * Use TypedEnqueueRequestForOwner to reconcile the owner of the object the event is for
// - do this for events for the types the Controller creates.  (e.g. ReplicaSets created by a Deployment Controller)
//
// * Use TypedEnqueueRequestsFromMapFunc to transform an event for an object to a reconcile of an object
// of a different type - do this for events for types the Controller may be interested in, but doesn't create.
// (e.g. If Foo responds to cluster size events, map Node events to Foo objects.)
//
// Unless you are implementing your own TypedEventHandler, you can ignore the functions on the TypedEventHandler interface.
// Most users shouldn't need to implement their own TypedEventHandler.
//
// TypedEventHandler is experimental and subject to future change.
type TypedEventHandler[T any] interface {
	// Create is called in response to a create event - e.g. Pod Creation.
	Create(context.Context, event.TypedCreateEvent[T], workqueue.RateLimitingInterface)

	// Update is called in response to an update event -  e.g. Pod Updated.
	Update(context.Context, event.TypedUpdateEvent[T], workqueue.RateLimitingInterface)

	// Delete is called in response to a delete event - e.g. Pod Deleted.
	Delete(context.Context, event.TypedDeleteEvent[T], workqueue.RateLimitingInterface)

	// Generic is called in response to an event of an unknown type or a synthetic event triggered as a cron or
	// external trigger request - e.g. reconcile Autoscaling, or a Webhook.
	Generic(context.Context, event.TypedGenericEvent[T], workqueue.RateLimitingInterface)
}

var _ EventHandler = Funcs{}

// Funcs implements eventhandler.
type Funcs = TypedFuncs[client.Object]

// TypedFuncs implements eventhandler.
//
// TypedFuncs is experimental and subject to future change.
type TypedFuncs[T any] struct {
	// Create is called in response to an add event.  Defaults to no-op.
	// RateLimitingInterface is used to enqueue reconcile.Requests.
	CreateFunc func(context.Context, event.TypedCreateEvent[T], workqueue.RateLimitingInterface)

	// Update is called in response to an update event.  Defaults to no-op.
	// RateLimitingInterface is used to enqueue reconcile.Requests.
	UpdateFunc func(context.Context, event.TypedUpdateEvent[T], workqueue.RateLimitingInterface)

	// Delete is called in response to a delete event.  Defaults to no-op.
	// RateLimitingInterface is used to enqueue reconcile.Requests.
	DeleteFunc func(context.Context, event.TypedDeleteEvent[T], workqueue.RateLimitingInterface)

	// GenericFunc is called in response to a generic event.  Defaults to no-op.
	// RateLimitingInterface is used to enqueue reconcile.Requests.
	GenericFunc func(context.Context, event.TypedGenericEvent[T], workqueue.RateLimitingInterface)
}

// Create implements EventHandler.
func (h TypedFuncs[T]) Create(ctx context.Context, e event.TypedCreateEvent[T], q workqueue.RateLimitingInterface) {
	if h.CreateFunc != nil {
		h.CreateFunc(ctx, e, q)
	}
}

// Delete implements EventHandler.
func (h TypedFuncs[T]) Delete(ctx context.Context, e event.TypedDeleteEvent[T], q workqueue.RateLimitingInterface) {
	if h.DeleteFunc != nil {
		h.DeleteFunc(ctx, e, q)
	}
}

// Update implements EventHandler.
func (h TypedFuncs[T]) Update(ctx context.Context, e event.TypedUpdateEvent[T], q workqueue.RateLimitingInterface) {
	if h.UpdateFunc != nil {
		h.UpdateFunc(ctx, e, q)
	}
}

// Generic implements EventHandler.
func (h TypedFuncs[T]) Generic(ctx context.Context, e event.TypedGenericEvent[T], q workqueue.RateLimitingInterface) {
	if h.GenericFunc != nil {
		h.GenericFunc(ctx, e, q)
	}
}
