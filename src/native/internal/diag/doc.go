// Package diag is the supportability core: short error references that people can read to support, a
// bounded recorder of recent errors for the Diagnostics view, and the glue that sends every safego failure
// to the log and the recorder under one reference.
//
// The recorder's Record type is the analytics schema for errors: it can hold a reference, a code-defined
// component label, a time, redacted error text, function and file-name frames, and allow-listed
// attributes, and nothing else. A test reflects over the struct so a new field cannot slip in unreviewed.
//
// Wire it once in the composition root:
//
//	recorder := diag.NewRecorder(clock, logx.NewRedactor())
//	reporter := diag.NewReporter(logger, recorder)
//	safego.SetDefaults(safego.Defaults{Clock: clock, Reporter: reporter})
//	defer diag.InstallPanicHandler(reporter)()
//
// Layer: rank 7, above logx and safego; it imports platform, logx and safego, never store.
package diag
