// Package hyprland is the Hyprland adapter of the compositor capabilities: it detects the running compositor's version
// and config format, renders Nevermind's shortcuts and window appearance as one Nevermind-owned config module, and
// applies, undoes and removes that module with atomic writes, validation and reload. It never edits a user's config
// beyond the two include lines setup adds with approval, and removal takes away only those lines and the module.
//
// Callers pass portable intent (accelerators, a shell command, which windows to undecorate); this package owns the
// syntax. Extensions never reach it directly.
//
// Supported versions (from the Hyprland wiki and release notes; see support.go):
//
//	version      config                         binds                       window rules
//	< 0.45       -                              unsupported                 unsupported
//	0.45 - 0.52  hyprland.conf (hyprlang)       bind = MODS, key, exec, cmd unsupported (pre-v3 rule syntax)
//	0.53 - 0.54  hyprland.conf (hyprlang)       bind = MODS, key, exec, cmd windowrule = match:class ..., effect value
//	>= 0.55      hyprland.lua, or a .conf only  hl.bind + hl.dsp.exec_cmd   hl.window_rule({ match = ..., effect = ... })
//	             (deprecated) when no .lua
//
// When both hyprland.lua and hyprland.conf exist the loaded one is not known, so setup is manual.
//
// Layer: platform. It imports only the standard library.
package hyprland
