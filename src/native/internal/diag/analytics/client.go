package analytics

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pablopunk/nvm/native/internal/diag"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// IngestEndpoint is the first-party route release builds send to. Development builds leave Config.Endpoint empty.
const IngestEndpoint = "https://api.nvm.fyi/api/ingest/batch/"

// IngestKey is the placeholder the route replaces with the project key. The app ships no real key.
const IngestKey = "nevermind-desktop"

const (
	flushAt       = 10
	flushInterval = 10 * time.Second
	// maxQueue bounds the events held while the endpoint cannot be reached. Past it, new events are counted and dropped.
	maxQueue = 1000
	maxBatch = 100
	// maxAttempts is how many unavailable answers (5xx, 429) one batch gets before it is dropped.
	maxAttempts = 3
)

// Config describes one client. Endpoint empty means the build never sends, and every call is a no-op.
type Config struct {
	Endpoint  string
	APIKey    string
	Transport platform.HTTP
	FS        platform.FS
	Clock     platform.Clock
	// Dir holds the install identifier file.
	Dir       string
	Version   string
	OS        string
	OSRelease string
	Arch      string
	// Consent is the starting state of "Send Technical Diagnostics". SetConsent changes it later.
	Consent bool
	Log     *logx.Logger
}

// Reporting says what happens to a record's report, for the label the Diagnostics view shows.
type Reporting int

// The reporting states. LocalOnly is a build that never sends; Off is a person who turned diagnostics off; Requested
// is a record the client may send, whose arrival at the endpoint is not confirmed.
const (
	LocalOnly Reporting = iota
	Off
	Requested
)

// queued is one event waiting for the sender.
type queued struct {
	event string
	at    time.Time
	props map[string]any
}

// Client queues events and sends them. A nil *Client ignores every call, so features need no check in development
// builds or before the consent switch exists.
type Client struct {
	cfg     Config
	common  map[string]any
	consent atomic.Bool
	kick    chan struct{}

	mu      sync.Mutex
	queue   []queued
	dropped int

	// sendMu serializes flushes, and guards the identifier and the attempt count, which only flushes touch.
	sendMu         sync.Mutex
	installID      string
	ownerMu        sync.Mutex
	observedOwners map[string]struct{}
	attempts       int
}

// New returns a client with its consent set to cfg.Consent. It sends nothing until Start or Flush runs.
func New(cfg Config) *Client {
	c := &Client{cfg: cfg, common: commonProperties(cfg), kick: make(chan struct{}, 1), observedOwners: make(map[string]struct{})}
	c.consent.Store(cfg.Consent)
	return c
}

// Channel names semantic releases stable, except explicit Go/Next prereleases. CalVer 2000+ and malformed versions
// are prerelease so unknown builds never count as stable users.
func Channel(version string) string {
	if strings.Contains(version, "-") {
		return "prerelease"
	}
	core, _, _ := strings.Cut(version, "+")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return "prerelease"
	}
	for _, part := range parts {
		if part == "" {
			return "prerelease"
		}
		if _, err := strconv.Atoi(part); err != nil {
			return "prerelease"
		}
	}
	n, _ := strconv.Atoi(parts[0])
	if n >= 2000 {
		return "prerelease"
	}
	return "stable"
}

// commonProperties are the fields every event carries. Callers cannot set them.
func commonProperties(cfg Config) map[string]any {
	properties := map[string]any{
		"$process_person_profile": false,
		"app_version":             cfg.Version,
		"os_type":                 cfg.OS,
		"arch":                    cfg.Arch,
		"channel":                 Channel(cfg.Version),
	}
	if cfg.OSRelease != "" {
		properties["os_release"] = cfg.OSRelease
	}
	return properties
}

// SetConsent turns sending on or off. Turning it off drops what is queued at once, so a batch that was waiting
// never goes out after the person said no.
func (c *Client) SetConsent(on bool) {
	if c == nil {
		return
	}
	c.consent.Store(on)
	if !on {
		c.discard()
	}
}

// Consent reports whether events are being kept for sending.
func (c *Client) Consent() bool {
	return c != nil && c.consent.Load()
}

// Reporting says what a record's report means at this moment.
func (c *Client) Reporting() Reporting {
	switch {
	case c == nil || c.cfg.Endpoint == "":
		return LocalOnly
	case !c.consent.Load():
		return Off
	default:
		return Requested
	}
}

// AppLaunched records that the app started.
func (c *Client) AppLaunched() bool {
	return c.capture(EventAppLaunched, nil)
}

// FeatureUsed records that the person used a feature. variant is a closed-set id, or empty when there is none.
func (c *Client) FeatureUsed(feature, variant string) bool {
	props := map[string]any{"feature": feature}
	if variant != "" {
		props["variant"] = variant
	}
	return c.capture(EventFeatureUsed, props)
}

// Exception sends a recorded failure under its support reference. The record's text is already redacted and bounded
// by the recorder; this only trims what the ingest route would trim anyway.
func (c *Client) Exception(r diag.Record) bool {
	attributes := r.Attributes
	if attributes == nil {
		attributes = map[string]string{}
	}
	return c.capture(EventException, map[string]any{
		"support_ref": r.Ref,
		"component":   r.Component,
		"message":     clip(r.Message, maxMessage),
		"frames":      clipFrames(r.Frames),
		"attributes":  attributes,
	})
}

// SupportReport sends a problem report that carries only its support reference.
func (c *Client) SupportReport(ref string) bool {
	return c.capture(EventSupportReport, map[string]any{"support_ref": ref})
}

// ObserveInstallOwner records the signed-in account as a one-way hash once per owner per run.
// capture queues one event. It returns false when the event was not kept: sending is off, the event is unknown, or the
// queue is full. It never waits on the network.
func (c *Client) capture(event string, props map[string]any) bool {
	if !c.accepting() {
		return false
	}
	allowed, known := schema[event]
	if !known {
		return false
	}
	kept := c.keepAllowed(props, allowed)
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.queue) >= maxQueue {
		c.dropped++
		return false
	}
	c.queue = append(c.queue, queued{event: event, at: c.cfg.Clock.Now(), props: kept})
	if len(c.queue) >= flushAt {
		c.signal()
	}
	return true
}

// accepting reports whether this client keeps events at all.
func (c *Client) accepting() bool {
	return c != nil && c.cfg.Endpoint != "" && c.consent.Load()
}

// keepAllowed returns the common fields plus the props whose keys the event's schema lists.
func (c *Client) keepAllowed(props map[string]any, allowed []string) map[string]any {
	kept := make(map[string]any, len(c.common)+len(allowed))
	for key, value := range c.common {
		kept[key] = value
	}
	for _, key := range allowed {
		if value, ok := props[key]; ok {
			kept[key] = value
		}
	}
	return kept
}

// signal asks the sender to flush early, without blocking when it already has a request pending.
func (c *Client) signal() {
	select {
	case c.kick <- struct{}{}:
	default:
	}
}

// discard drops every queued event.
func (c *Client) discard() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dropped += len(c.queue)
	c.queue = nil
}

// Dropped counts the events the client did not keep or send, for diagnostics and tests.
func (c *Client) Dropped() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.dropped
}

// Pending counts the events waiting to be sent.
func (c *Client) Pending() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.queue)
}

// Start runs the sender until ctx ends. A batch goes out every flush interval, or as soon as flushAt events wait.
func (c *Client) Start(ctx context.Context) {
	if c == nil || c.cfg.Endpoint == "" {
		return
	}
	safego.Go("analytics.send", c.run, safego.WithContext(ctx))
}

// run is the sender loop. Failures are logged by Flush and retried on the next tick.
func (c *Client) run(ctx context.Context) {
	ticker := c.cfg.Clock.NewTicker(flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C():
		case <-c.kick:
		}
		_ = c.Flush(ctx)
	}
}

// head copies the first n queued events without removing them, so a batch that fails is still queued.
func (c *Client) head(n int) []queued {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.queue[:min(n, len(c.queue))])
}

// ack removes the first n events, once the endpoint has taken them or refused them for good.
func (c *Client) ack(n int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.queue = slices.Delete(c.queue, 0, min(n, len(c.queue)))
}

// clip cuts s to at most limit runes.
func clip(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit])
}

// clipFrames keeps the first maxFrames frames.
func clipFrames(frames []string) []string {
	return slices.Clone(frames[:min(maxFrames, len(frames))])
}
