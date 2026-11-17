// The film: about 22 seconds from black to black. The helpers are in motion.js and the themes in themes.js.

const themeItem = spot("#mb-theme");
document.getElementById("menu").style.left = themeItem.x - 22 + "px";
const P = {
  aiRow: spot("#ai-row", 0.42),
  cells: [0, 1, 2, 3].map((i) => spot("#cell" + i, 0.55, 0.42)),
  theme: themeItem,
  midnight: spot("#mi-midnight", 0.3),
  files: [1, 2, 3].map((i) => spot("#fi" + i + " .th", 0.5, 0.55)),
};
// Camera framings, as in the earlier video but gentler: in on the query while it is typed, a little in on a list,
// close on the Dock while two themes land, and on the window or menu being used.
const QUERY = { x: 550, y: 200 };
const LIST = { x: 550, y: 297 };
const DOCK = { x: 550, y: 500 };
const FINDER = { x: 550, y: 250 };
const MENU = { x: 850, y: 160 };
const DURATION = 22.5;

// Rest state: a black screen with nothing on it.
init(".view", { visibility: "hidden" });
init(".ch", { display: "none" });
init(["#palette", "#menubar", "#dock", "#finder", "#menu", "#cursor", ".wall", "#vid-done", ".answer .w", "#tool-done", "#fc1", "#fc2", "#fc3"], { opacity: 0 });
init("#cam", { scale: 1, x: 0, y: 0, transformOrigin: "0 0" });
init("#cursor", { x: 640, y: 420, transformOrigin: "1.5px 1.3px" });
init("#palette", { transformOrigin: "50% 40%" });
init("#finder", { scale: 0.97, transformOrigin: "50% 50%" });
init(["#fc1", "#fc2", "#fc3"], { scale: 0.8, transformOrigin: "50% 30%" });
init("#menu", { scale: 0.96 });
init(".st", { display: "none" });
init("#tool-done", { display: "none" });
init(".dk .tint", { backgroundColor: TINT.midnight });
for (const g of ["vs0", "vs1", "vs2", "va0", "va1", "va2"]) init("#" + g + "-0", { display: "inline-flex" });
for (const id of ["calq-h1", "calq-h2", "cal-h1"]) init("#" + id, { display: "none" });
tl.fromTo(".spin", { rotation: 0 }, { rotation: 360 * 27, duration: DURATION, ease: "none" }, 0);

// 1. Nevermind opens on a black desktop and writes a theme switcher with AI.
tl.to(["#menubar", "#dock"], { opacity: 1, duration: 0.4, ease: "power1.out" }, 0.2);
show("v-root", 0.3);
openPalette(0.3);
camera(0.5, 0.5, 1.3, QUERY);
cut("v-root", "v-ai", 0.65);
type("q1", 0.65, 0.032);
tl.to("#cursor", { opacity: 1, duration: 0.1 }, 1.45);
cursorTo(P.aiRow, 1.5, 0.45);
camera(1.7, 0.5, 1);
click(1.98);
cut("v-ai", "v-chat", 2.05);
tl.fromTo("#bubble", { opacity: 0, y: 4 }, { opacity: 1, y: 0, duration: 0.16 }, 2.05);
tl.fromTo("#tool", { opacity: 0, y: 4 }, { opacity: 1, y: 0, duration: 0.16 }, 2.2);
tl.fromTo("#checking", { opacity: 0 }, { opacity: 1, duration: 0.1 }, 2.2);
tl.to("#cursor", { opacity: 0, duration: 0.2 }, 2.3);
camera(2.25, 1.8, 1.06, { x: 550, y: 260 }, "sine.inOut");
tl.set("#tool-run", { display: "none" }, 3.0);
tl.set("#tool-done", { display: "block", opacity: 1 }, 3.0);
tl.to("#checking", { opacity: 0, duration: 0.1 }, 3.0);
document.querySelectorAll(".answer .w").forEach((w, i) => tl.set(w, { opacity: 1 }, 3.08 + i * 0.024));

// 2. Theme Switcher runs. Each theme is a look, a wallpaper and a Dock tint, applied as the selection moves; the
// camera moves close on the Dock for the second and third, so the tint sweeping across the icons fills the frame.
camera(4.1, 0.3, 1.3, QUERY);
cut("v-chat", "v-theme-q", 4.15);
type("q2", 4.15, 0.05);
cut("v-theme-q", "v-grid", 4.7);
camera(4.75, 0.45, 1);
const cells = [...document.querySelectorAll(".cell")];
select(cells, 0, 0);
tl.set("#cursor", { x: 860, y: 450 }, 4.75);
tl.to("#cursor", { opacity: 1, duration: 0.1 }, 4.8);
const picks = [
  [1, 5.3, LIGHT, "w-sunset", TINT.sunset],
  [2, 6.05, DARK, "w-ocean", TINT.ocean],
  [3, 6.85, LIGHT, "w-terraces", TINT.terraces],
];
camera(5.55, 0.45, 1.7, DOCK);
for (const [i, t, theme, wall, tint] of picks) {
  cursorTo(P.cells[i], t - 0.42, 0.38);
  click(t);
  select(cells, i, t + 0.02);
  setTheme(theme, t + 0.04, 0.3);
  wallpaper(wall, t + 0.04, 0.35);
  tintDock(tint, t + 0.06);
}
closePalette(7.65);
tl.to("#cursor", { opacity: 0, duration: 0.2 }, 7.7);
hide("v-grid", 7.8);
camera(7.75, 0.5, 1);

// 3. Pull requests.
show("v-root", 8.35);
openPalette(8.35);
camera(8.4, 0.4, 1.3, QUERY);
cut("v-root", "v-pr-q", 8.6);
type("q3", 8.6, 0.05);
cut("v-pr-q", "v-pr", 9.05);
camera(9.05, 0.45, 1.15, LIST);
select(rows("v-pr"), 1, 9.5);
select(rows("v-pr"), 2, 9.85);

// 4. Calendar events.
camera(10.3, 0.3, 1.3, QUERY);
cut("v-pr", "v-root", 10.35);
cut("v-root", "v-cal-q", 10.5);
type("q4", 10.5, 0.05);
select(rows("v-cal-q"), 1, 10.95);
only(["calq-h0", "calq-h1", "calq-h2"], 1, 10.95);
select(rows("v-cal-q"), 2, 11.15);
only(["calq-h0", "calq-h1", "calq-h2"], 2, 11.15);
cut("v-cal-q", "v-cal", 11.4);
camera(11.4, 0.4, 1.15, LIST);
select(rows("v-cal"), 1, 11.9);
only(["cal-h0", "cal-h1"], 1, 11.9);
select(rows("v-cal"), 2, 12.25);
closePalette(12.85);
hide("v-cal", 13.0);
camera(12.85, 0.5, 1);

// 5. Three videos selected in Finder, compressed from the palette. The compressed copies then appear in the window,
// each next to its original.
tl.to("#finder", { opacity: 1, scale: 1, duration: 0.25, ease: "power2.out" }, 13.0);
only(["fs0", "fs1", "fs2", "fs3", "fs4"], 0, 0);
tl.set("#cursor", { x: 470, y: 330 }, 13.05);
tl.to("#cursor", { opacity: 1, duration: 0.1 }, 13.1);
P.files.forEach((p, i) => {
  const t = 13.43 + i * 0.3;
  cursorTo(p, t - 0.26, 0.22);
  click(t);
  tl.set("#fi" + (i + 1), { attr: { class: "fi sel" } }, t + 0.02);
  only(["fs0", "fs1", "fs2", "fs3", "fs4"], i + 1, t + 0.02);
});
tl.to("#cursor", { opacity: 0, duration: 0.2 }, 14.15);
const v = 14.35;
show("v-root", v);
openPalette(v);
camera(v + 0.05, 0.35, 1.3, QUERY);
cut("v-root", "v-vid-q", v + 0.2);
type("q5", v + 0.2, 0.04);
cut("v-vid-q", "v-vid", v + 0.8);
camera(v + 0.8, 0.4, 1.15, LIST);
for (let i = 0; i < 3; i++) {
  const t = v + 0.9 + i * 0.35;
  state("va" + i, 1, 3, t);
  for (let k = 1; k <= 4; k++) state("vs" + i, k, 6, t + (k - 1) * 0.2);
  state("vs" + i, 5, 6, t + 0.9);
  state("va" + i, 2, 3, t + 0.9);
}
tl.to("#vid-done", { opacity: 1, duration: 0.2 }, v + 2.6);
const shown = v + 3.3;
closePalette(shown);
camera(shown, 0.5, 1.25, FINDER);
for (const [id, n] of Object.entries(FILES_AFTER)) {
  if (id.startsWith("fi")) tl.to("#" + id, { left: slot(n).left, top: slot(n).top, duration: 0.35, ease: "power2.inOut" }, shown + 0.12);
}
tl.to(["#fc1", "#fc2", "#fc3"], { opacity: 1, scale: 1, duration: 0.25, ease: "back.out(1.6)", stagger: 0.1 }, shown + 0.3);
only(["fs0", "fs1", "fs2", "fs3", "fs4"], 4, shown + 0.3);
tl.to("#finder", { opacity: 0, scale: 0.98, duration: 0.2, ease: "power2.in" }, shown + 1.25);

// 6. The extension's menu bar item: Midnight brings back the dark look, the grey Dock and the black desktop the film
// started on.
const m = shown + 1.4;
camera(m + 0.05, 0.55, 1.25, MENU);
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
tintDock(TINT.midnight, m + 1.62);
camera(m + 1.6, 0.8, 1);
tl.to("#cursor", { opacity: 0, duration: 0.3 }, m + 1.95);
tl.to(["#menubar", "#dock"], { opacity: 0, duration: 0.45, ease: "power1.in" }, m + 2.45);
tl.set({}, {}, DURATION);

window.__timelines = window.__timelines || {};
window.__timelines["main"] = tl;
tl.seek(0);
