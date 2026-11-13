package app

import "testing"

func TestDevelopmentBuildsAreNotPackaged(t *testing.T) {
	if packagedBuild() {
		t.Fatal("a build without the packaging link flags reports a packaged build")
	}
	if releaseBuild == "true" {
		t.Fatal("a build without the packaging link flags reports a release build")
	}
}

// asPackagedBuild links the packaged flag for one test, as the packaging workflow links a shipped build.
func asPackagedBuild(t *testing.T) {
	t.Helper()
	previous := isPackagedBuild
	isPackagedBuild = "true"
	t.Cleanup(func() { isPackagedBuild = previous })
}
