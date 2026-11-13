// Package windowmgmt is the window management engine (WP4.1b): commands that place, raise, full-screen,
// and display-move other apps' windows through the platform.WindowManager port.
//
// Every command either reaches its window and returns a Result (exact or partial, verified by reading the
// frame back) or fails with a *Failure whose reason and copy the palette shows. Commands for one process
// run one at a time; per-window history drives cycling, restore and undo; listed windows get expiring
// opaque handles. Pure frame arithmetic lives in the geometry subpackage.
//
// Layer: service tier, above platform and below palette. The engine does not know which OS it runs on.
package windowmgmt
