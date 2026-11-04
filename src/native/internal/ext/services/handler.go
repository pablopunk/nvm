// Package services is the host side of the context services an extension calls: storage, secrets, settings,
// logs, text and network. The child sends each call over the protocol; the Handler of one extension answers it
// with that extension's namespace, its declared capabilities and its quotas. Nothing here takes an extension id
// from the call, so one extension cannot name another's data.
//
// Layer: ext. The package imports api, protocol, store, logx, features/calc, ai/extai, ai/quota, platform and the standard library.
package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/pablopunk/nvm/native/internal/ai/extai"
	aiquota "github.com/pablopunk/nvm/native/internal/ai/quota"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/features/clipboard"
	"github.com/pablopunk/nvm/native/internal/features/ocr"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/store"
)

// maxFailureMessage bounds the text a failure carries back to the child.
const maxFailureMessage = 512

// errHandlerClosed answers calls made after Close.
var errHandlerClosed = unavailable("this extension's services are closed")

// Deps are the ports the services use. Keychain, Network and Clipboard may be nil: the services that need them
// then answer unavailable, and nothing falls back to plaintext.
type Deps struct {
	Files platform.FS
	Clock platform.Clock
	// DataRoot holds one folder per extension, named after its id. Each folder is that extension's store.
	DataRoot string
	Keychain platform.Keychain
	// Network must be the public-address port: extensions never reach private or loopback addresses through it.
	Network   platform.HTTP
	Clipboard platform.Clipboard
	// Logs receives extension records. Nil means the process logger.
	Logs *logx.Logger
	// OS is the host's operating system name, reported by the system service.
	OS string
	// SettingsPanes lists the System Settings panes the host can open. Nil means the host offers none.
	SettingsPanes func() []api.SettingsPane
	// Places lists the standard folders of the home folder the palette opens. Nil means the host offers none.
	Places func() []api.Place
	// Capabilities are the host's platform capabilities; the system service and the OCR service read them.
	Capabilities platform.Capabilities
	// Shell opens paths and links, lists and launches apps and reads app icons. Nil answers unavailable.
	Shell platform.Shell
	// Commands runs the programs and scripts of the shell service. Nil answers unavailable.
	Commands platform.Commands
	// Home is the user's home folder, which a leading "~" in a shell argument stands for. Empty leaves "~" as it is.
	Home string
	// Windows lists and places windows through the window engine. Nil answers unavailable.
	Windows WindowEngine
	// Frontmost names the app in front. Nil answers unavailable.
	Frontmost FrontmostSource
	// Notifier shows operating-system notifications. Nil answers unavailable.
	Notifier platform.Notifier
	// Shortcuts reads the shortcut table. Nil answers unavailable.
	Shortcuts ShortcutTable
	// Indicators draws the passive indicators. Nil answers unavailable.
	Indicators IndicatorSurface
	// OCR reads text from images and the screen. Nil answers unavailable.
	OCR *ocr.Service
	// History returns the clipboard history, or nil while it is still starting.
	History func() *clipboard.History
	// Confirm asks the user before a destructive call runs. Nil refuses destructive calls.
	Confirm ConfirmFunc
	// AI makes the model calls of ctx.AI. Nil answers unavailable.
	AI *extai.Service
	// AIQuota shares each extension's model-call count with the app's other callers of the same owner. Nil counts
	// each extension alone.
	AIQuota *aiquota.Registry
	// MenuBarRefresh asks the host to re-render the menu bar items of one extension. It must not block: the call
	// returns before the items change. Nil makes the request a no-op.
	MenuBarRefresh func(extensionID string)
	// RootRefresh asks the host to run the root's rows again. It must not block, as MenuBarRefresh. Nil makes the
	// request a no-op.
	RootRefresh func(extensionID string)
}

// documents are the extension's files: one store folder holds all of them.
type documents struct {
	store    *store.Store
	storage  *store.Collection[storageDoc]
	secrets  *store.Collection[secretIndexDoc]
	settings *store.Collection[settingValuesDoc]
	data     *store.Collection[dataCacheDoc]
	drafts   *store.Collection[draftsDoc]
}

// Handler answers the service calls of one extension. It is safe for concurrent use.
type Handler struct {
	deps             Deps
	manifest         api.Manifest
	capabilities     map[api.Capability]bool
	logger           *logx.Logger
	fetches          *rateLimiter
	logWrites        *rateLimiter
	actions          *rateLimiter
	notifications    *rateLimiter
	processes        *rateLimiter
	indicatorUpdates *rateLimiter
	ocrReads         *rateLimiter
	screenReads      *rateLimiter
	rootRefreshes    *rateLimiter
	aiCalls          *aiquota.Window
	ai               *aiState
	openOnce         sync.Once
	openErr          error
	closeOnce        sync.Once
	docs             *documents
	secretsMu        sync.Mutex
}

// New returns the services of one extension. Nothing is opened until the extension makes its first call.
func New(deps Deps, manifest api.Manifest) *Handler {
	logs := deps.Logs
	if logs == nil {
		logs = logx.Default()
	}
	capabilities := map[api.Capability]bool{}
	for _, capability := range manifest.Capabilities {
		capabilities[capability] = true
	}
	return &Handler{
		deps:             deps,
		manifest:         manifest,
		capabilities:     capabilities,
		logger:           logs.With(logx.F{"source": "extension", "extension": manifest.ID}),
		fetches:          newRateLimiter(deps.Clock, fetchesPerMinute),
		logWrites:        newRateLimiter(deps.Clock, logWritesPerMinute),
		actions:          newRateLimiter(deps.Clock, desktopActionsPerMinute),
		notifications:    newRateLimiter(deps.Clock, notificationsPerMinute),
		processes:        newRateLimiter(deps.Clock, processRunsPerMinute),
		indicatorUpdates: newRateLimiter(deps.Clock, indicatorUpdatesPerMinute),
		ocrReads:         newRateLimiter(deps.Clock, ocrReadsPerMinute),
		screenReads:      newRateLimiter(deps.Clock, screenReadsPerMinute),
		rootRefreshes:    newBurstRateLimiter(deps.Clock, rootRefreshBurst, rootRefreshesPerMinute),
		aiCalls:          aiCallWindow(deps, manifest.ID),
		ai:               newAIState(),
	}
}

// Close flushes and closes the extension's store. A handler closed before its first call never opens the store,
// and every later call answers unavailable. Close is safe to call more than once.
func (h *Handler) Close() error {
	h.ai.close()
	h.openOnce.Do(func() { h.openErr = errHandlerClosed })
	var closeErr error
	h.closeOnce.Do(func() {
		if h.docs != nil {
			closeErr = h.docs.store.Close()
		}
	})
	return closeErr
}

// Serve answers one call. It never panics on a bad argument: every failure is a Failure in the reply.
func (h *Handler) Serve(ctx context.Context, call protocol.ServiceCall) protocol.ServiceReply {
	marker := &waitMark{}
	reply := h.serveMarked(context.WithValue(ctx, waitMarkKey{}, marker), call)
	reply.WaitedForUser = marker.waited
	return reply
}

func (h *Handler) serveMarked(ctx context.Context, call protocol.ServiceCall) protocol.ServiceReply {
	result, err := h.dispatch(ctx, call)
	if err != nil {
		failure := failureOf(err)
		return protocol.ServiceReply{Failure: &failure}
	}
	data, err := json.Marshal(result)
	if err != nil {
		return protocol.ServiceReply{Failure: &api.Failure{Code: api.CodeInternal, Message: "the service result cannot be encoded"}}
	}
	return protocol.ServiceReply{Data: data}
}

func (h *Handler) dispatch(ctx context.Context, call protocol.ServiceCall) (any, error) {
	switch call.Service {
	case protocol.ServiceStorage:
		return h.serveStorage(call)
	case protocol.ServiceSecrets:
		return h.serveSecrets(call)
	case protocol.ServiceSettings:
		return h.serveSettings(call)
	case protocol.ServiceLogs:
		return h.serveLogs(call)
	case protocol.ServiceText:
		return h.serveText(call)
	case protocol.ServiceNetwork:
		return h.serveNetwork(ctx, call)
	case protocol.ServiceDrafts:
		return h.serveDrafts(call)
	case protocol.ServiceDesktopClipboard:
		return h.serveDesktopClipboard(call)
	case protocol.ServiceDesktopApps:
		return h.serveDesktopApps(ctx, call)
	case protocol.ServiceDesktopFiles:
		return h.serveDesktopFiles(ctx, call)
	case protocol.ServiceDesktopShell:
		return h.serveDesktopShell(ctx, call)
	case protocol.ServiceDesktopWindows:
		return h.serveDesktopWindows(ctx, call)
	case protocol.ServiceClipboardHistory:
		return h.serveClipboardHistory(ctx, call)
	case protocol.ServiceOCR:
		return h.serveOCR(ctx, call)
	case protocol.ServiceSystem:
		return h.serveSystem(call)
	case protocol.ServiceShortcuts:
		return h.serveShortcuts(call)
	case protocol.ServiceViews:
		return h.serveViews(call)
	case protocol.ServiceIndicators:
		return h.serveIndicators(call)
	case protocol.ServiceAI:
		return h.serveAI(ctx, call)
	}
	return nil, invalid("unknown service %q", call.Service)
}

// require refuses a call whose capability the manifest does not declare, and records the refusal.
func (h *Handler) require(capability api.Capability) error {
	if h.capabilities[capability] {
		return nil
	}
	h.logger.Warn("refused a call that needs an undeclared capability", logx.F{"capability": string(capability)})
	return serviceError{
		failure: api.Failure{Code: api.CodePermission, Message: fmt.Sprintf("this extension did not declare the %s capability in its manifest", capability)},
		cause:   api.ErrPermission,
	}
}

// openedDocuments opens the extension's store on first use and returns its typed documents.
func (h *Handler) openedDocuments() (*documents, error) {
	h.openOnce.Do(func() { h.docs, h.openErr = h.openDocuments() })
	return h.docs, h.openErr
}

func (h *Handler) openDocuments() (*documents, error) {
	if h.deps.Files == nil || h.deps.DataRoot == "" {
		return nil, unavailable("extension storage is not available in this build")
	}
	dir := filepath.Join(h.deps.DataRoot, h.manifest.ID)
	st, err := store.Open(h.deps.Files, h.deps.Clock, dir)
	if err != nil {
		return nil, unavailable("cannot open the extension's data folder: %v", err)
	}
	return &documents{
		store:    st,
		storage:  store.Doc(st, storageFile, 1, newStorageDoc),
		secrets:  store.Doc(st, secretsFile, 1, newSecretIndex),
		settings: store.Doc(st, settingsFile, 1, newSettingValues),
		data:     store.Doc(st, dataCacheFile, 1, newDataCache),
		drafts:   store.Doc(st, draftsFile, 1, newDraftsDoc),
	}, nil
}

// serviceError is a failure with its code already chosen. Its cause keeps errors.Is working for the caller.
type serviceError struct {
	failure api.Failure
	cause   error
}

func (e serviceError) Error() string {
	return e.failure.Message
}

func (e serviceError) Unwrap() error {
	return e.cause
}

func unavailable(format string, args ...any) error {
	return serviceError{failure: api.Failure{Code: api.CodeUnavailable, Message: fmt.Sprintf(format, args...)}, cause: api.ErrNotAvailable}
}

func invalid(format string, args ...any) error {
	return fmt.Errorf(format+": %w", append(args, api.ErrInvalidInput)...)
}

// failureOf turns a service error into the wire failure, clipping the message to a palette-safe length.
func failureOf(err error) api.Failure {
	var typed serviceError
	failure := api.FailureOf(err)
	if errors.As(err, &typed) {
		failure = typed.failure
	}
	if len(failure.Message) > maxFailureMessage {
		failure.Message = failure.Message[:maxFailureMessage] + "..."
	}
	return failure
}

// decodeArgs reads the JSON arguments of one operation. Missing arguments decode as the zero value.
func decodeArgs(raw json.RawMessage, into any) error {
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, into); err != nil {
		return invalid("the arguments do not fit the operation")
	}
	return nil
}
