// The appearances the theme switcher applies.

// The token sets of src/native/internal/design/tokens.go, plus the desktop around the palette. The pane is the
// palette's translucent window material; the bar, Dock and menu bar highlight follow the wallpaper's appearance.
const DARK = {
  "--pane": "rgba(26, 31, 40, 0.86)", "--pane-edge": "rgba(211, 224, 245, 0.14)",
  "--surface": "rgba(34, 40, 52, 1)", "--interactive": "rgba(41, 48, 61, 1)", "--selected": "rgba(52, 61, 77, 1)",
  "--edge-subtle": "rgba(211, 224, 245, 0.1)", "--edge-strong": "rgba(211, 224, 245, 0.18)",
  "--strong": "rgba(240, 243, 247, 1)", "--primary": "rgba(224, 229, 236, 1)", "--secondary": "rgba(184, 192, 204, 1)",
  "--tertiary": "rgba(156, 166, 180, 1)", "--muted": "rgba(143, 153, 167, 1)", "--faint": "rgba(143, 153, 167, 0.75)",
  "--accent": "rgba(241, 207, 84, 1)", "--accent-fill": "rgba(226, 189, 57, 1)", "--accent-contrast": "rgba(36, 32, 21, 1)",
  "--accent-soft": "rgba(241, 207, 84, 0.12)", "--accent-muted": "rgba(241, 207, 84, 0.24)", "--accent-emph": "rgba(241, 207, 84, 0.5)",
  "--hi": "rgba(220, 231, 249, 0.1)", "--lo": "rgba(0, 0, 0, 0.5)", "--success": "rgba(159, 201, 168, 1)",
  "--tone-yellow": "rgba(223, 206, 132, 1)", "--tone-blue": "rgba(169, 201, 223, 1)", "--tone-purple": "rgba(200, 182, 223, 1)",
  "--tone-green": "rgba(172, 209, 178, 1)", "--tone-red": "rgba(224, 170, 170, 1)", "--tone-orange": "rgba(223, 189, 150, 1)",
  "--bar-fg": "rgba(255, 255, 255, 1)", "--bar-glow": "rgba(0, 0, 0, 0.3)", "--bar-hl": "rgba(255, 255, 255, 0.2)",
  "--dock-bg": "rgba(48, 48, 54, 0.5)", "--dock-edge": "rgba(255, 255, 255, 0.16)", "--dock-hi": "rgba(255, 255, 255, 0.1)",
};
const LIGHT = {
  "--pane": "rgba(245, 242, 235, 0.82)", "--pane-edge": "rgba(255, 255, 255, 0.5)",
  "--surface": "rgba(250, 248, 243, 1)", "--interactive": "rgba(232, 228, 220, 1)", "--selected": "rgba(221, 216, 206, 1)",
  "--edge-subtle": "rgba(54, 49, 41, 0.14)", "--edge-strong": "rgba(54, 49, 41, 0.24)",
  "--strong": "rgba(40, 39, 33, 1)", "--primary": "rgba(52, 50, 44, 1)", "--secondary": "rgba(94, 90, 81, 1)",
  "--tertiary": "rgba(109, 104, 94, 1)", "--muted": "rgba(115, 110, 100, 1)", "--faint": "rgba(115, 110, 100, 0.75)",
  "--accent": "rgba(138, 106, 0, 1)", "--accent-fill": "rgba(230, 189, 50, 1)", "--accent-contrast": "rgba(36, 32, 21, 1)",
  "--accent-soft": "rgba(138, 106, 0, 0.11)", "--accent-muted": "rgba(138, 106, 0, 0.22)", "--accent-emph": "rgba(138, 106, 0, 0.52)",
  "--hi": "rgba(255, 254, 250, 0.9)", "--lo": "rgba(62, 53, 40, 0.13)", "--success": "rgba(53, 109, 69, 1)",
  "--tone-yellow": "rgba(116, 93, 0, 1)", "--tone-blue": "rgba(40, 93, 130, 1)", "--tone-purple": "rgba(101, 76, 134, 1)",
  "--tone-green": "rgba(55, 107, 69, 1)", "--tone-red": "rgba(152, 70, 70, 1)", "--tone-orange": "rgba(135, 83, 37, 1)",
  "--bar-fg": "rgba(29, 29, 31, 1)", "--bar-glow": "rgba(255, 255, 255, 0.25)", "--bar-hl": "rgba(0, 0, 0, 0.12)",
  "--dock-bg": "rgba(255, 255, 255, 0.3)", "--dock-edge": "rgba(255, 255, 255, 0.55)", "--dock-hi": "rgba(255, 255, 255, 0.45)",
};

// The Dock's icon tint for each theme, a hue taken from its wallpaper. Midnight, the black desktop, is grey: the
// icons keep only their light and shade.
const TINT = {
  midnight: "rgb(128, 128, 128)",
  glacier: "rgb(70, 140, 196)",
  aurora: "rgb(46, 176, 124)",
  misty: "rgb(206, 116, 132)",
};
