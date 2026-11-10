package child

import "github.com/pablopunk/nvm/native/internal/ext/api"

// childNavigation builds the results that change the palette stack. Each result is data the host applies, so the
// child needs no host call for it.
type childNavigation struct{}

func (childNavigation) Root(view *api.View) api.Result {
	return api.Result{View: view, Navigation: api.NavRoot}
}

func (childNavigation) Push(view *api.View) api.Result {
	return api.Result{View: view, Navigation: api.NavPush}
}

func (childNavigation) Replace(view *api.View) api.Result {
	return api.Result{View: view, Navigation: api.NavReplace}
}

func (childNavigation) Pop() api.Result {
	return api.Result{Navigation: api.NavPop}
}

func (childNavigation) Run(action api.Action) api.Result {
	return api.Result{Next: &action}
}
