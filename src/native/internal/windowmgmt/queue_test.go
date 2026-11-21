package windowmgmt

import (
	"context"
	"errors"
	"testing"
)

func TestQueueRunsTasksForOneProcessInArrivalOrder(t *testing.T) {
	queue := newPIDQueue()
	ctx := context.Background()
	started := make(chan struct{})
	gate := make(chan struct{})
	var order []string

	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		_, _ = runQueued(ctx, queue, 1, func(context.Context) (struct{}, error) {
			close(started)
			<-gate
			order = append(order, "first")
			return struct{}{}, nil
		})
	}()
	<-started
	secondDone := make(chan struct{})
	go func() {
		defer close(secondDone)
		_, _ = runQueued(ctx, queue, 1, func(context.Context) (struct{}, error) {
			order = append(order, "second")
			return struct{}{}, nil
		})
	}()
	close(gate)
	<-firstDone
	<-secondDone

	if len(order) != 2 || order[0] != "first" || order[1] != "second" {
		t.Fatalf("order = %v, want [first second]", order)
	}
}

func TestQueueWaiterGivesUpWhenItsContextIsCancelled(t *testing.T) {
	queue := newPIDQueue()
	started := make(chan struct{})
	gate := make(chan struct{})
	holderDone := make(chan struct{})
	go func() {
		defer close(holderDone)
		_, _ = runQueued(context.Background(), queue, 1, func(context.Context) (struct{}, error) {
			close(started)
			<-gate
			return struct{}{}, nil
		})
	}()
	<-started

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	ran := false
	_, err := runQueued(cancelled, queue, 1, func(context.Context) (struct{}, error) {
		ran = true
		return struct{}{}, nil
	})
	if !errors.Is(err, context.Canceled) || ran {
		t.Fatalf("cancelled waiter: err = %v, ran = %v", err, ran)
	}

	close(gate)
	<-holderDone
	if len(queue.lanes) != 0 {
		t.Fatalf("idle lanes were not released: %d", len(queue.lanes))
	}
}
