package child

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// hostContext is the api.Context every entry point receives. The accessors for the context services are wired
// here; every other service is the embedded nil interface: calling it panics, and the dispatcher reports the call
// as an internal failure with a reference. Services are wired by later work behind the host.
type hostContext struct {
	api.Context
	identity api.Identity
	launch   api.LaunchContext
	clock    platform.Clock
	ctx      context.Context
	link     *hostLink
	cache    *processCache
	flights  *flightGroup
}

var _ api.Context = hostContext{}

func newHostContext(identity api.Identity, clock platform.Clock, link *hostLink) hostContext {
	return hostContext{
		identity: identity,
		clock:    clock,
		ctx:      context.Background(),
		link:     link,
		cache:    newProcessCache(clock),
		flights:  newFlightGroup(),
	}
}

func (c hostContext) Std() context.Context {
	return c.ctx
}

func (c hostContext) Launch() api.LaunchContext {
	return c.launch
}

func (c hostContext) Identity() api.Identity {
	return c.identity
}

func (c hostContext) Clock() api.Clock {
	return c.clock
}

func (c hostContext) Storage() api.Storage {
	return remoteStorage{link: c.link, memo: c.memo()}
}

func (c hostContext) Secrets() api.Secrets {
	return remoteSecrets{link: c.link}
}

func (c hostContext) Cache() api.Cache {
	return c.cache
}

func (c hostContext) Settings() api.Settings {
	return remoteSettings{link: c.link}
}

func (c hostContext) Logs() api.Logs {
	return remoteLogs{link: c.link}
}

func (c hostContext) Text() api.Text {
	return remoteText{link: c.link}
}

func (c hostContext) Drafts() api.Drafts {
	return remoteDrafts{link: c.link}
}

func (c hostContext) Input() api.Input {
	return api.NewInput()
}

func (c hostContext) Data() api.Data {
	return dataSources{}
}

func (c hostContext) Network() api.Network {
	return remoteNetwork{link: c.link}
}

func (c hostContext) Clipboard() api.ClipboardHistory {
	return remoteHistory{link: c.link}
}

func (c hostContext) Desktop() api.Desktop {
	return remoteDesktop{link: c.link}
}

func (c hostContext) OCR() api.OCR {
	return remoteOCR{link: c.link}
}

func (c hostContext) System() api.System {
	return remoteSystem{link: c.link}
}

func (c hostContext) Shortcuts() api.Shortcuts {
	return remoteShortcuts{link: c.link}
}

func (c hostContext) Indicators() api.Indicators {
	return remoteIndicators{link: c.link}
}

func (c hostContext) Views() api.Views {
	return remoteViews{link: c.link}
}

func (c hostContext) memo() memoCore {
	return memoCore{kv: storageKV{link: c.link}, clock: c.clock, flights: c.flights}
}
