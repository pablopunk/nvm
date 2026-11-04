// Package update is the updater port: the version the app runs, what it knows about newer versions, and the service
// that checks, downloads and installs them. The service (service.go, download.go, schedule.go) runs over an Engine
// (engine.go); the real engine is the platform SelfUpdater (platform/real, over MyGo's signed updater), and tests use
// update/fake. NoUpdates is the updater of builds that cannot update themselves: it never contacts the network.
//
// Layer: service tier (with jobs, shortcuts, windowmgmt, dictation, menubar and diag). It imports platform only.
package update
