// Package compositor keeps Nevermind's shortcuts and window appearance in step with a compositor's config, through the
// platform adapter (platform/hyprland). It caches what detection found, so the palette never waits on it, builds the
// desired module from the shortcut document, remembers whether the compositor owns the shortcuts, and wraps the hotkey
// port so an accelerator the compositor runs is not also registered through the portal, which would run it twice.
//
// Layer: shortcuts.
package compositor
