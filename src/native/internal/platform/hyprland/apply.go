package hyprland

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"time"
)

// Loaded is what Nevermind could tell about the compositor after a reload.
type Loaded string

const (
	// LoadedConfirmed: the compositor lists every generated shortcut.
	LoadedConfirmed Loaded = "confirmed"
	// LoadedMissing: the compositor lists none of them; the include is probably not loaded.
	LoadedMissing Loaded = "missing"
	// LoadedUnverified: nothing could be checked, such as a module with no shortcuts, or no reload.
	LoadedUnverified Loaded = "unverified"
)

// Result is what a lifecycle step did.
type Result struct {
	Changed   bool
	Reloaded  bool
	Loaded    Loaded
	Conflicts []string
	Skipped   []Skipped
}

const validateTimeout = 10 * time.Second

// undoRecord is the state before the last apply, kept in Env.UndoFile.
type undoRecord struct {
	ModulePath    string `json:"modulePath"`
	ModuleBefore  []byte `json:"moduleBefore,omitempty"`
	ModuleExisted bool   `json:"moduleExisted"`
	ModuleAfter   []byte `json:"moduleAfter"`
	Entry         string `json:"entry"`
	IncludeAdded  bool   `json:"includeAdded"`
	Format        Format `json:"format"`
}

// Apply writes the plan: the module, and the include when addInclude and it is missing. It refuses when either file
// changed since the preview. With the include in place it validates, reloads and checks the compositor's errors; on a
// failure it puts both files back and reloads again, so the previous working state stays. The previous state is kept
// for Undo.
func Apply(ctx context.Context, env Env, plan Plan, addInclude bool) (Result, error) {
	result := Result{Skipped: plan.Skipped, Loaded: LoadedUnverified}
	if err := plan.unchangedSincePreview(); err != nil {
		return result, err
	}
	addInclude = addInclude && !plan.IncludePresent
	if plan.ModuleUnchanged() && !addInclude {
		return verifyAfterReload(ctx, env, plan, result)
	}
	s := plan.Session
	record := undoRecord{
		ModulePath: s.ModulePath, ModuleBefore: plan.moduleBefore, ModuleExisted: plan.moduleExisted,
		ModuleAfter: plan.ModuleAfter, Entry: s.Entry, IncludeAdded: addInclude, Format: s.Format,
	}
	errorsBefore := configErrors(ctx, env)
	if err := writeAtomic(s.ModulePath, plan.ModuleAfter); err != nil {
		return result, fmt.Errorf("write %s: %w", s.ModulePath, err)
	}
	if addInclude {
		if err := writeAtomic(s.Entry, withInclude(plan.entryBefore, IncludeLines(s.Format, s.ModulePath))); err != nil {
			rollback(plan)
			return result, fmt.Errorf("add the include to %s: %w", s.Entry, err)
		}
	}
	result.Changed = true
	loaded := plan.IncludePresent || addInclude
	if err := validate(ctx, env, s, loaded); err != nil {
		rollback(plan)
		return Result{Skipped: plan.Skipped, Loaded: LoadedUnverified}, err
	}
	if loaded {
		if err := reloadChecked(ctx, env, errorsBefore); err != nil {
			rollback(plan)
			_, _, _ = run(ctx, env, "hyprctl", "reload")
			return Result{Skipped: plan.Skipped, Loaded: LoadedUnverified}, err
		}
		result.Reloaded = true
	}
	if err := saveUndo(env.UndoFile, record); err != nil {
		return result, fmt.Errorf("applied, but the undo record was not saved: %w", err)
	}
	return verifyAfterReload(ctx, env, plan, result)
}

// rollback restores the module and entry the plan read.
func rollback(plan Plan) {
	s := plan.Session
	if plan.moduleExisted {
		_ = writeAtomic(s.ModulePath, plan.moduleBefore)
	} else {
		_ = os.Remove(s.ModulePath)
	}
	_ = writeAtomic(s.Entry, plan.entryBefore)
}

// verifyCrashed records that this Hyprland's --verify-config crashed instead of answering. Some versions segfault on
// exit even for a valid config; every crash also shows the user a crash notification, so it is not run again in this
// process, and the reload check below stands in for it.
var verifyCrashed atomic.Bool

// validate checks the generated config with Hyprland --verify-config when this Hyprland has it: the entry when it
// loads the module, else the module alone. A check that crashes, rather than rejecting the config, counts as no
// check; reloadChecked still undoes a change the compositor reports errors for.
func validate(ctx context.Context, env Env, s Session, loaded bool) error {
	if !s.VerifyConfig || verifyCrashed.Load() {
		return nil
	}
	target := s.ModulePath
	if loaded {
		target = s.Entry
	}
	ctx, cancel := context.WithTimeout(ctx, validateTimeout)
	defer cancel()
	out, code, err := env.Run(ctx, "Hyprland", "--verify-config", "--config", target)
	if err != nil {
		return fmt.Errorf("could not check the config with Hyprland: %w", err)
	}
	if code < 0 || code >= 128 {
		verifyCrashed.Store(true)
		return nil
	}
	if code != 0 {
		return fmt.Errorf("the config was rejected by Hyprland, so nothing changed: %s", firstLines(out, 4))
	}
	return nil
}

// reloadChecked reloads the compositor and fails when it reports config errors it did not report before.
func reloadChecked(ctx context.Context, env Env, before []string) error {
	out, code, err := run(ctx, env, "hyprctl", "reload")
	if err != nil || code != 0 {
		return fmt.Errorf("hyprctl reload failed (%s): %s", describeFailure(code, err), firstLines(out, 2))
	}
	var added []string
	for _, e := range configErrors(ctx, env) {
		if !slices.Contains(before, e) {
			added = append(added, e)
		}
	}
	if len(added) > 0 {
		return fmt.Errorf("the reloaded config has errors, so the change was undone: %s", strings.Join(added, "; "))
	}
	return nil
}

// configErrors lists the errors "hyprctl configerrors" reports now.
func configErrors(ctx context.Context, env Env) []string {
	out, code, err := run(ctx, env, "hyprctl", "configerrors")
	if err != nil || code != 0 {
		return nil
	}
	var errs []string
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			errs = append(errs, line)
		}
	}
	return errs
}

func firstLines(out []byte, n int) string {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, " ")
}

func saveUndo(path string, record undoRecord) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return writeAtomic(path, data)
}

func loadUndo(path string) (undoRecord, bool, error) {
	data, ok, err := readOptional(path)
	if err != nil || !ok {
		return undoRecord{}, false, err
	}
	var record undoRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return undoRecord{}, false, fmt.Errorf("the undo record is unreadable: %w", err)
	}
	return record, true, nil
}

// ErrNothingToUndo reports that no apply is recorded.
var ErrNothingToUndo = errors.New("there is no Hyprland change to undo")

// Undo puts back the module and include as they were before the last apply, if the module is still what that apply
// wrote, and reloads.
func Undo(ctx context.Context, env Env, s Session) (Result, error) {
	record, ok, err := loadUndo(env.UndoFile)
	if err != nil || !ok {
		return Result{}, errors.Join(ErrNothingToUndo, err)
	}
	current, _, err := readOptional(record.ModulePath)
	if err != nil {
		return Result{}, err
	}
	if !bytes.Equal(current, record.ModuleAfter) {
		return Result{}, fmt.Errorf("%s changed since Nevermind wrote it, so it was left as it is", record.ModulePath)
	}
	if record.ModuleExisted {
		err = writeAtomic(record.ModulePath, record.ModuleBefore)
	} else {
		err = os.Remove(record.ModulePath)
	}
	if err != nil {
		return Result{}, err
	}
	if record.IncludeAdded {
		if err := removeInclude(record.Entry, record.Format, record.ModulePath); err != nil {
			return Result{Changed: true}, err
		}
	}
	_ = os.Remove(env.UndoFile)
	return reloadIfRunning(ctx, env, s, Result{Changed: true, Loaded: LoadedUnverified})
}

// Remove deletes the module Nevermind wrote and the include lines setup added, and nothing else. It is idempotent.
func Remove(ctx context.Context, env Env, s Session) (Result, error) {
	if !s.Ready() {
		return Result{}, errors.New(s.Problem)
	}
	result := Result{Loaded: LoadedUnverified}
	data, existed, err := readOptional(s.ModulePath)
	if err != nil {
		return result, err
	}
	if existed {
		if !ownedModule(data) {
			return result, fmt.Errorf("%w: %s", ErrNotOwned, s.ModulePath)
		}
		if err := os.Remove(s.ModulePath); err != nil {
			return result, err
		}
		result.Changed = true
	}
	entry, err := os.ReadFile(s.Entry)
	if err != nil {
		return result, err
	}
	if _, found := withoutInclude(entry, IncludeLines(s.Format, s.ModulePath)); found {
		if err := removeInclude(s.Entry, s.Format, s.ModulePath); err != nil {
			return result, err
		}
		result.Changed = true
	}
	_ = os.Remove(env.UndoFile)
	if !result.Changed {
		return result, nil
	}
	return reloadIfRunning(ctx, env, s, result)
}

func removeInclude(entryPath string, f Format, modulePath string) error {
	entry, err := os.ReadFile(entryPath)
	if err != nil {
		return err
	}
	if out, found := withoutInclude(entry, IncludeLines(f, modulePath)); found {
		return writeAtomic(entryPath, out)
	}
	return nil
}

func reloadIfRunning(ctx context.Context, env Env, s Session, result Result) (Result, error) {
	if !s.Running {
		return result, nil
	}
	if err := reloadChecked(ctx, env, configErrors(ctx, env)); err != nil {
		return result, err
	}
	result.Reloaded = true
	return result, nil
}
