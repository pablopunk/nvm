package main

import "github.com/pablopunk/nvm/native/internal/ext/api"

// Only Commands and RunAction: nothing queries this extension unasked, so it waits dormant once its commands are saved.

func Commands(ctx api.Context) ([]api.Command, error) {
	return []api.Command{{ID: "ping", Title: "Command line ping", Mode: api.ModeNoView}}, nil
}

func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	return api.Result{Toast: &api.Toast{Message: "pinged " + in.Ref.ID}}, nil
}
