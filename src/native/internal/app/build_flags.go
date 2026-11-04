package app

import "fmt"

// BuildFlags reports the release link flags of this binary. The packaging checks run it on every artifact, so a
// packaged build that lost its flags, or a development build that gained them, fails before it ships.
func BuildFlags() string {
	return fmt.Sprintf("packaged=%t release=%t", packagedBuild(), releaseBuild == "true")
}
