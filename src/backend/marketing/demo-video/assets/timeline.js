// The film's one timeline. Every state is a function of timeline time, so HyperFrames can seek to any frame.

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

const stage = document.getElementById("stage");
buildPalette(document.getElementById("palette"));
hydrateIcons(document);
for (const [k, v] of Object.entries(DARK)) stage.style.setProperty(k, v);

// Points on the 1100 × 619 screen, measured before any tween moves things.
const SCALE = 1920 / 1100;
const stageBox = stage.getBoundingClientRect();
function spot(el, fx = 0.5, fy = 0.5) {
  const r = (typeof el === "string" ? document.querySelector(el) : el).getBoundingClientRect();
  return { x: (r.left - stageBox.left + r.width * fx) / SCALE, y: (r.top - stageBox.top + r.height * fy) / SCALE };
}
const themeItem = spot("#mb-theme");
document.getElementById("menu").style.left = themeItem.x - 22 + "px";
const P = {
  aiRow: spot("#ai-row", 0.42),
  cells: [0, 1, 2, 3].map((i) => spot("#cell" + i, 0.55, 0.42)),
  theme: themeItem,
  midnight: spot("#mi-midnight", 0.3),
  files: [1, 2, 3].map((i) => spot("#fi" + i + " .th", 0.5, 0.55)),
};

const tl = gsap.timeline({ paused: true, defaults: { immediateRender: false } });
const DURATION = 21.7;

function init(target, vars) {
  tl.set(target, { ...vars, immediateRender: true }, 0);
}
function show(id, t) {
  tl.set("#" + id, { visibility: "visible" }, t);
}
function hide(id, t) {
  tl.set("#" + id, { visibility: "hidden" }, t);
}
// A page change inside the palette: the new page's rows settle in, as the app's list does after a query.
function cut(from, to, t) {
  hide(from, t);
  show(to, t);
  tl.fromTo("#" + to + " .list, #" + to + " .grid, #" + to + " .chat", { opacity: 0.35, y: 3 }, { opacity: 1, y: 0, duration: 0.14, ease: "power2.out" }, t);
}
function type(key, t0, perChar) {
  const groups = [...document.querySelectorAll('.type[data-key="' + key + '"]')].map((el) => el.querySelectorAll(".ch"));
  groups[0].forEach((_, i) => {
    for (const g of groups) tl.set(g[i], { display: "inline" }, t0 + i * perChar);
  });
  return t0 + groups[0].length * perChar;
}
function select(items, index, t) {
  items.forEach((el, i) => tl.set(el, { attr: { class: el.dataset.base + (i === index ? " sel" : "") } }, t));
}
function rows(id) {
  return [...document.querySelectorAll("#" + id + " .row")];
}
function only(ids, index, t) {
  ids.forEach((id, i) => tl.set("#" + id, { display: i === index ? "flex" : "none" }, t));
}
function state(group, k, count, t) {
  for (let j = 0; j < count; j++) tl.set("#" + group + "-" + j, { display: j === k ? "inline-flex" : "none" }, t);
}
function setTheme(to, t, dur) {
  tl.to("#stage", { ...to, duration: dur, ease: "power1.inOut" }, t);
}
let wallZ = 1;
let currentWall = null;
function wallpaper(id, t, dur) {
  const prev = currentWall;
  if (id) {
    tl.set("#" + id, { zIndex: ++wallZ }, t);
    tl.fromTo("#" + id, { opacity: 0 }, { opacity: 1, duration: dur, ease: "power1.inOut" }, t);
  }
  if (prev && id) tl.set("#" + prev, { opacity: 0 }, t + dur);
  if (prev && !id) tl.to("#" + prev, { opacity: 0, duration: dur, ease: "power1.inOut" }, t);
  currentWall = id;
}
function openPalette(t) {
  tl.fromTo("#palette", { opacity: 0, scale: 0.985 }, { opacity: 1, scale: 1, duration: 0.14, ease: "power2.out" }, t);
}
function closePalette(t) {
  tl.to("#palette", { opacity: 0, scale: 0.985, duration: 0.12, ease: "power2.in" }, t);
}
function cursorTo(p, t, dur) {
  tl.to("#cursor", { x: p.x, y: p.y, duration: dur, ease: "power3.inOut" }, t);
}
function click(t) {
  tl.to("#cursor", { scale: 0.86, duration: 0.05, yoyo: true, repeat: 1, ease: "power1.inOut" }, t);
}

// Rest state: a black screen with nothing on it.
init(".view", { visibility: "hidden" });
init(".ch", { display: "none" });
init(["#palette", "#menubar", "#dock", "#finder", "#menu", "#cursor", ".wall", "#vid-done", ".answer .w", "#tool-done"], { opacity: 0 });
init("#cursor", { x: 640, y: 420, transformOrigin: "1.5px 1.3px" });
init("#palette", { transformOrigin: "50% 40%" });
init("#finder", { scale: 0.97, transformOrigin: "50% 50%" });
init("#menu", { scale: 0.96 });
init(".st", { display: "none" });
init("#tool-done", { display: "none" });
for (const g of ["vs0", "vs1", "vs2", "va0", "va1", "va2"]) init("#" + g + "-0", { display: "inline-flex" });
for (const id of ["calq-h1", "calq-h2", "cal-h1"]) init("#" + id, { display: "none" });
tl.fromTo(".spin", { rotation: 0 }, { rotation: 360 * 26, duration: DURATION, ease: "none" }, 0);

// 1. Nevermind opens on a black desktop and writes a theme switcher with AI.
tl.to(["#menubar", "#dock"], { opacity: 1, duration: 0.4, ease: "power1.out" }, 0.2);
show("v-root", 0.3);
openPalette(0.3);
cut("v-root", "v-ai", 0.65);
type("q1", 0.65, 0.032);
tl.to("#cursor", { opacity: 1, duration: 0.1 }, 1.45);
cursorTo(P.aiRow, 1.5, 0.45);
click(1.98);
cut("v-ai", "v-chat", 2.05);
tl.fromTo("#bubble", { opacity: 0, y: 4 }, { opacity: 1, y: 0, duration: 0.16 }, 2.05);
tl.fromTo("#tool", { opacity: 0, y: 4 }, { opacity: 1, y: 0, duration: 0.16 }, 2.2);
tl.fromTo("#checking", { opacity: 0 }, { opacity: 1, duration: 0.1 }, 2.2);
tl.to("#cursor", { opacity: 0, duration: 0.2 }, 2.3);
tl.set("#tool-run", { display: "none" }, 3.0);
tl.set("#tool-done", { display: "block", opacity: 1 }, 3.0);
tl.to("#checking", { opacity: 0, duration: 0.1 }, 3.0);
document.querySelectorAll(".answer .w").forEach((w, i) => tl.set(w, { opacity: 1 }, 3.08 + i * 0.024));

// 2. Theme Switcher runs: each theme is a look and a wallpaper, applied as the selection moves.
cut("v-chat", "v-theme-q", 4.3);
type("q2", 4.3, 0.05);
cut("v-theme-q", "v-grid", 4.85);
select([...document.querySelectorAll(".cell")], 0, 0);
tl.set("#cursor", { x: 860, y: 450 }, 4.9);
tl.to("#cursor", { opacity: 1, duration: 0.1 }, 4.95);
const cells = [...document.querySelectorAll(".cell")];
const picks = [
  [1, 5.4, LIGHT, "w-glacier"],
  [2, 6.15, DARK, "w-aurora"],
  [3, 6.9, LIGHT, "w-misty"],
];
for (const [i, t, theme, wall] of picks) {
  cursorTo(P.cells[i], t - 0.42, 0.38);
  click(t);
  select(cells, i, t + 0.02);
  setTheme(theme, t + 0.04, 0.3);
  wallpaper(wall, t + 0.04, 0.35);
}
closePalette(7.5);
tl.to("#cursor", { opacity: 0, duration: 0.2 }, 7.55);
hide("v-grid", 7.62);

// 3. Pull requests.
show("v-root", 8.3);
openPalette(8.3);
cut("v-root", "v-pr-q", 8.55);
type("q3", 8.55, 0.05);
cut("v-pr-q", "v-pr", 9.0);
rows("v-pr").slice(1, 4).forEach((_, i) => select(rows("v-pr"), i + 1, 9.45 + i * 0.35));

// 4. Calendar events.
cut("v-pr", "v-root", 10.7);
cut("v-root", "v-cal-q", 10.85);
type("q4", 10.85, 0.05);
select(rows("v-cal-q"), 1, 11.3);
only(["calq-h0", "calq-h1", "calq-h2"], 1, 11.3);
select(rows("v-cal-q"), 2, 11.5);
only(["calq-h0", "calq-h1", "calq-h2"], 2, 11.5);
cut("v-cal-q", "v-cal", 11.75);
select(rows("v-cal"), 1, 12.25);
only(["cal-h0", "cal-h1"], 1, 12.25);
select(rows("v-cal"), 2, 12.6);
closePalette(13.2);
hide("v-cal", 13.34);

// 5. Three videos selected in Finder, compressed from the palette.
tl.to("#finder", { opacity: 1, scale: 1, duration: 0.25, ease: "power2.out" }, 13.35);
only(["fs0", "fs1", "fs2", "fs3"], 0, 0);
tl.set("#cursor", { x: 470, y: 330 }, 13.4);
tl.to("#cursor", { opacity: 1, duration: 0.1 }, 13.45);
P.files.forEach((p, i) => {
  const t = 13.78 + i * 0.3;
  cursorTo(p, t - 0.26, 0.22);
  click(t);
  tl.set("#fi" + (i + 1), { attr: { class: "fi sel" } }, t + 0.02);
  only(["fs0", "fs1", "fs2", "fs3"], i + 1, t + 0.02);
});
tl.to("#cursor", { opacity: 0, duration: 0.2 }, 14.5);
const v = 14.7;
show("v-root", v);
openPalette(v);
cut("v-root", "v-vid-q", v + 0.2);
type("q5", v + 0.2, 0.04);
cut("v-vid-q", "v-vid", v + 0.8);
for (let i = 0; i < 3; i++) {
  const t = v + 0.9 + i * 0.35;
  state("va" + i, 1, 3, t);
  for (let k = 1; k <= 4; k++) state("vs" + i, k, 6, t + (k - 1) * 0.2);
  state("vs" + i, 5, 6, t + 0.9);
  state("va" + i, 2, 3, t + 0.9);
}
tl.to("#vid-done", { opacity: 1, duration: 0.2 }, v + 2.6);
closePalette(v + 3.3);
tl.to("#finder", { opacity: 0, scale: 0.98, duration: 0.2, ease: "power2.in" }, v + 3.42);

// 6. The extension's menu bar item: Midnight brings back the dark look and the black desktop the film started on.
const m = v + 3.55;
tl.set("#cursor", { x: 640, y: 340 }, m);
tl.to("#cursor", { opacity: 1, duration: 0.12 }, m + 0.05);
cursorTo(P.theme, m + 0.07, 0.55);
click(m + 0.65);
tl.set("#mb-theme", { attr: { class: "item on" } }, m + 0.67);
tl.to("#menu", { opacity: 1, scale: 1, duration: 0.12, ease: "power3.out" }, m + 0.69);
cursorTo(P.midnight, m + 0.93, 0.3);
tl.set("#mi-midnight", { attr: { class: "mi hot" } }, m + 1.23);
click(m + 1.5);
tl.to("#mi-midnight", { opacity: 0.4, duration: 0.04, yoyo: true, repeat: 1 }, m + 1.5);
tl.to("#menu", { opacity: 0, duration: 0.14, ease: "power2.in" }, m + 1.6);
tl.set("#mb-theme", { attr: { class: "item" } }, m + 1.65);
setTheme(DARK, m + 1.6, 0.5);
wallpaper(null, m + 1.6, 0.55);
tl.to("#cursor", { opacity: 0, duration: 0.3 }, m + 1.95);
tl.to(["#menubar", "#dock"], { opacity: 0, duration: 0.45, ease: "power1.in" }, m + 2.45);
tl.set({}, {}, DURATION);

window.__timelines = window.__timelines || {};
window.__timelines["main"] = tl;
tl.seek(0);
