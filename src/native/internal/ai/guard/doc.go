// Package guard decides whether a path a model names may be touched. It refuses credential locations (SSH keys, cloud
// and git credentials, browser profiles, keychains), dotenv files that hold secrets, and anything outside the roots it
// is given. Symbolic links are judged where they point, so a link into a credential location is refused too.
//
// Layer: ai. The guard never reads a file; it resolves paths through the Canonicalize function its owner supplies.
package guard
