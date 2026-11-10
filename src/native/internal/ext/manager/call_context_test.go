package manager

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/bundle"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
)

type callContextProbe struct {
	runner
	seen context.Context
}

func (p *callContextProbe) Call(ctx context.Context, method, _ string, _ *api.Invocation) (protocol.Response, error) {
	p.seen = ctx
	data := json.RawMessage(`[]`)
	if method == protocol.MethodRun {
		data = json.RawMessage(`{}`)
	}
	return protocol.Response{Data: data}, nil
}

func contextProbeManager(builtin bool, probe *callContextProbe) (*Manager, string, runtimeTarget) {
	id, key := "acme.probe", "acme.probe"
	if builtin {
		id = "nevermind.probe"
		key = builtinKeyPrefix + id
	}
	methods := []string{protocol.MethodSearch, protocol.MethodRun, protocol.MethodLoad, protocol.MethodCommands, protocol.MethodMenuBars, protocol.MethodMenuBarTriggers}
	budgets := make(map[string]supervisor.Budget)
	for i, method := range methods {
		budgets[method] = supervisor.Budget{Hard: time.Duration(i+1) * time.Hour}
	}
	m := &Manager{entries: map[string]*entry{key: {builtin: builtin, runtime: probe, bundle: bundle.Bundle{Manifest: api.Manifest{Identity: api.Identity{ID: id}}}}}, config: supervisor.Config{Budgets: budgets}}
	return m, id, runtimeTarget{id: id, runtime: probe}
}

func TestManagerCallBudgetOwnership(t *testing.T) {
	for _, builtin := range []bool{false, true} {
		for _, method := range []string{protocol.MethodSearch, protocol.MethodRun, protocol.MethodLoad, protocol.MethodCommands, protocol.MethodMenuBars, protocol.MethodMenuBarTriggers} {
			name := "supervised/" + method
			if builtin {
				name = "builtin/" + method
			}
			t.Run(name, func(t *testing.T) {
				probe := &callContextProbe{}
				m, id, target := contextProbeManager(builtin, probe)
				ctx := context.Background()
				budget := m.config.BudgetFor(method)
				switch method {
				case protocol.MethodSearch:
					if r := m.callOne(ctx, target, method, "windows", budget); r.Failure != nil {
						t.Fatal(r.Failure)
					}
				case protocol.MethodRun:
					if _, failure, err := m.Run(ctx, id, api.Invocation{}); failure != nil || err != nil {
						t.Fatal(failure, err)
					}
				case protocol.MethodLoad:
					if _, _, failure, err := m.Load(ctx, id, "loader"); failure != nil || err != nil {
						t.Fatal(failure, err)
					}
				case protocol.MethodCommands:
					if r := m.commandsOf(ctx, target, budget); r.Failure != nil {
						t.Fatal(r.Failure)
					}
				default:
					var data []json.RawMessage
					if failure, err := m.callProvider(ctx, id, method, nil, &data); failure != nil || err != nil {
						t.Fatal(failure, err)
					}
				}
				if probe.seen == nil {
					t.Fatal("runtime was not called")
				}
				deadline, bounded := probe.seen.Deadline()
				if bounded != builtin {
					t.Fatalf("manager added conflicting supervised deadline or lost built-in budget: bounded=%t builtin=%t", bounded, builtin)
				}
				if builtin {
					remaining := time.Until(deadline)
					if remaining < budget.Hard-time.Minute || remaining > budget.Hard+time.Minute {
						t.Fatalf("%s uses wrong method budget: %s want %s", method, remaining, budget.Hard)
					}
				}
			})
		}
	}
}

type callContextValue struct{}

func TestManagerCallContextKeepsCallerLifetime(t *testing.T) {
	for _, builtin := range []bool{false, true} {
		m, id, _ := contextProbeManager(builtin, &callContextProbe{})
		parent, cancel := context.WithTimeout(context.WithValue(context.Background(), callContextValue{}, "session"), time.Minute)
		call, end := m.callContext(parent, id, supervisor.Budget{Hard: time.Hour})
		parentDeadline, _ := parent.Deadline()
		deadline, ok := call.Deadline()
		if !ok || !deadline.Equal(parentDeadline) || call.Value(callContextValue{}) != "session" {
			t.Fatal("caller deadline or correlation replaced")
		}
		cancel()
		select {
		case <-call.Done():
		default:
			t.Fatal("caller cancellation did not end the wait")
		}
		end()
	}
}
