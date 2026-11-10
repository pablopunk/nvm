package extview

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func TestInitialViewOpensBeforeCommandAndCancelsOnLeave(t *testing.T) {
	var callCtx context.Context
	var finish func(api.Result, *api.Failure)
	deps := Deps{InvokeCommandContext: func(ctx context.Context, _ string, in api.Invocation, done func(api.Result, *api.Failure)) {
		callCtx, finish = ctx, done
		if !in.AwaitInitialView {
			t.Error("missing host pending marker")
		}
	}, InvokeCommand: func(string, api.Invocation, func(api.Result, *api.Failure)) {
		t.Fatal("missing cancellable invocation")
	}}
	initial := api.GridView("Images", 3)
	host := &fakeHost{}
	CommandActionFor(deps, "acme", api.Run("open", "Images", nil), &initial).Run(host)
	if len(host.pushed) != 1 || host.pushed[0].Grid == nil || !host.pushed[0].Loading() {
		t.Fatal("grid was not immediately loading")
	}
	host.pushed[0].OnLeave()
	if callCtx.Err() != context.Canceled {
		t.Fatalf("context %v", callCtx.Err())
	}
	view := api.ListView("late")
	finish(api.Result{View: &view}, nil)
	if len(host.pushed) != 1 || host.pops != 0 {
		t.Fatal("obsolete result navigated")
	}
}

func TestInitialViewShowsFailureInPlace(t *testing.T) {
	var finish func(api.Result, *api.Failure)
	deps := Deps{InvokeCommandContext: func(_ context.Context, _ string, _ api.Invocation, done func(api.Result, *api.Failure)) {
		finish = done
	}, InvokeCommand: func(string, api.Invocation, func(api.Result, *api.Failure)) {}}
	initial := api.GridView("Images", 3)
	host := &fakeHost{}
	CommandActionFor(deps, "acme", api.Run("open", "Images", nil), &initial).Run(host)
	finish(api.Result{}, &api.Failure{Code: api.CodeTimeout, Message: "slow"})
	if host.pops != 0 || len(host.pushed) != 1 || host.pushed[0].Loading() {
		t.Fatal("error closed or kept loading view")
	}
	rows := host.pushed[0].Search("")
	if len(rows) != 1 || rows[0].Subtitle != "slow" {
		t.Fatalf("failure rows %+v", rows)
	}
}
