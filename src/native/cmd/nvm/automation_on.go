//go:build automation

// Test builds only (WP0.3 rig): links the MyGo automation plugin so a test runner can drive the
// running app through MYGO_AUTOMATION. Default builds do not compile this file.
package main

import _ "github.com/egoist/mygo/plugins/automation"
