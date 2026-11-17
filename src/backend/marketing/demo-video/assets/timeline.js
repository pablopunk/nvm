// The film's one timeline. Every state is a function of timeline time, so HyperFrames can seek to any frame.

// The design tokens of src/native/internal/design/tokens.go, plus the translucent surfaces of the window material.
const DARK = {
  "--panel": "rgba(26, 31, 40, 1)", "--glass": "rgba(26, 31, 40, 0.6)", "--card": "rgba(34, 40, 52, 0.55)",
  "--raised": "rgba(34, 40, 52, 1)", "--interactive": "rgba(41, 48, 61, 1)", "--selected": "rgba(52, 61, 77, 1)",
  "--edge-subtle": "rgba(211, 224, 245, 0.1)", "--edge-strong": "rgba(211, 224, 245, 0.18)", "--shadow": "rgba(0, 0, 0, 0.55)",
  "--strong": "rgba(240, 243, 247, 1)", "--primary": "rgba(224, 229, 236, 1)", "--secondary": "rgba(184, 192, 204, 1)",
  "--tertiary": "rgba(156, 166, 180, 1)", "--muted": "rgba(143, 153, 167, 1)", "--faint": "rgba(143, 153, 167, 0.75)",
  "--accent": "rgba(241, 207, 84, 1)", "--accent-fill": "rgba(226, 189, 57, 1)",
  "--accent-soft": "rgba(241, 207, 84, 0.12)", "--accent-muted": "rgba(241, 207, 84, 0.24)", "--accent-emph": "rgba(241, 207, 84, 0.5)",
  "--hi": "rgba(220, 231, 249, 0.1)", "--lo": "rgba(0, 0, 0, 0.5)", "--wash": "rgba(0, 0, 0, 0.2)", "--success": "rgba(159, 201, 168, 1)",
  "--tone-yellow": "rgba(223, 206, 132, 1)", "--tone-blue": "rgba(169, 201, 223, 1)", "--tone-purple": "rgba(200, 182, 223, 1)",
  "--tone-green": "rgba(172, 209, 178, 1)", "--tone-orange": "rgba(223, 189, 150, 1)", "--tone-pink": "rgba(223, 178, 204, 1)",
  "--bar-bg": "rgba(12, 14, 22, 0.32)", "--bar-fg": "rgba(240, 243, 247, 1)", "--dock": "rgba(40, 44, 56, 0.38)",
};
const LIGHT = {
  "--panel": "rgba(245, 242, 235, 1)", "--glass": "rgba(245, 242, 235, 0.66)", "--card": "rgba(250, 248, 243, 0.6)",
  "--raised": "rgba(250, 248, 243, 1)", "--interactive": "rgba(232, 228, 220, 1)", "--selected": "rgba(221, 216, 206, 1)",
  "--edge-subtle": "rgba(54, 49, 41, 0.14)", "--edge-strong": "rgba(54, 49, 41, 0.24)", "--shadow": "rgba(62, 53, 40, 0.3)",
  "--strong": "rgba(40, 39, 33, 1)", "--primary": "rgba(52, 50, 44, 1)", "--secondary": "rgba(94, 90, 81, 1)",
  "--tertiary": "rgba(109, 104, 94, 1)", "--muted": "rgba(115, 110, 100, 1)", "--faint": "rgba(115, 110, 100, 0.75)",
  "--accent": "rgba(138, 106, 0, 1)", "--accent-fill": "rgba(230, 189, 50, 1)",
  "--accent-soft": "rgba(138, 106, 0, 0.11)", "--accent-muted": "rgba(138, 106, 0, 0.22)", "--accent-emph": "rgba(138, 106, 0, 0.52)",
  "--hi": "rgba(255, 254, 250, 0.9)", "--lo": "rgba(62, 53, 40, 0.13)", "--wash": "rgba(62, 53, 40, 0.05)", "--success": "rgba(53, 109, 69, 1)",
  "--tone-yellow": "rgba(116, 93, 0, 1)", "--tone-blue": "rgba(40, 93, 130, 1)", "--tone-purple": "rgba(101, 76, 134, 1)",
  "--tone-green": "rgba(55, 107, 69, 1)", "--tone-orange": "rgba(135, 83, 37, 1)", "--tone-pink": "rgba(136, 79, 114, 1)",
  "--bar-bg": "rgba(250, 248, 243, 0.42)", "--bar-fg": "rgba(40, 39, 33, 1)", "--dock": "rgba(245, 242, 235, 0.45)",
};

const tl = gsap.timeline({ paused: true, defaults: { immediateRender: false } });
const ROW = 42;
const CELL_W = 176;
const CELL_H = 140;
let wallLayer = 1;

function init(target, vars) {
  tl.set(target, { ...vars, immediateRender: true }, 0);
}
function showState(q, v, t) {
  tl.set("#" + q, { visibility: "visible" }, t);
  tl.set("#" + v, { visibility: "visible" }, t);
}
function hideState(q, v, t) {
  tl.set("#" + q, { visibility: "hidden" }, t);
  tl.set("#" + v, { visibility: "hidden" }, t);
}
function cutTo(from, to, t) {
  hideState(from[0], from[1], t);
  showState(to[0], to[1], t);
  tl.fromTo("#" + to[1] + " > *", { opacity: 0.4, y: 3 }, { opacity: 1, y: 0, duration: 0.14, ease: "power2.out" }, t);
}
function typeQuery(q, v, t0, perChar) {
  const groups = [document.querySelectorAll("#" + q + " .type .ch")];
  if (v) for (const el of document.querySelectorAll("#" + v + " .type")) groups.push(el.querySelectorAll(".ch"));
  groups[0].forEach((_, i) => {
    for (const g of groups) tl.set(g[i], { display: "inline" }, t0 + i * perChar);
  });
  return t0 + groups[0].length * perChar;
}
function movePlate(v, index, t, extra = 0) {
  tl.to("#" + v + " .plate", { y: index * ROW + extra, duration: 0.07, ease: "power2.out" }, t);
}
function pressEnter(plate, t) {
  tl.fromTo(plate, { scaleX: 1, scaleY: 1 }, { scaleX: 0.985, scaleY: 0.93, duration: 0.06, yoyo: true, repeat: 1, ease: "power1.inOut" }, t);
}
// Fades the next wallpaper in on top of the current one, then drops the old one.
function changeWallpaper(from, to, t, d) {
  tl.set("#" + to, { zIndex: ++wallLayer }, t);
  tl.fromTo("#" + to, { opacity: 0 }, { opacity: 1, duration: d, ease: "power1.inOut" }, t);
  tl.set("#" + from, { opacity: 0 }, t + d);
}
function setTheme(from, to, t, d) {
  tl.fromTo("#stage", { ...from }, { ...to, duration: d, ease: "power2.inOut" }, t);
  const [outIcon, inIcon] = from === DARK ? ["#mb-moon", "#mb-sun"] : ["#mb-sun", "#mb-moon"];
  tl.fromTo(outIcon, { opacity: 1 }, { opacity: 0, duration: d * 0.5 }, t);
  tl.fromTo(inIcon, { opacity: 0 }, { opacity: 1, duration: d * 0.5 }, t + d * 0.5);
}
function openPalette(t) {
  tl.fromTo("#palette", { scale: 0.97 }, { scale: 1, duration: 0.16, ease: "power3.out" }, t);
  tl.fromTo("#palette .pane", { opacity: 0 }, { opacity: 1, duration: 0.14, ease: "power2.out" }, t);
}
function closePalette(t) {
  tl.to("#palette", { scale: 0.98, duration: 0.12, ease: "power2.in" }, t);
  tl.to("#palette .pane", { opacity: 0, duration: 0.12, ease: "power2.in" }, t);
}

// Initial state: dark desktop under the night wallpaper, palette closed. The last frame comes back here.
init("#stage", DARK);
init("#palette", { scale: 0.97 });
init(["#b-user", "#b-tool", "#b-reply", "#b-ext", "#b-status"], { opacity: 0 });
init("#mb-sun", { opacity: 0 });
init(["#p-total", ".sbar div"], { scaleX: 0 });
init(["#q0", "#v0"], { visibility: "visible" });
tl.fromTo(".caret", { opacity: 1 }, { opacity: 0, duration: 0.01, repeat: 37, yoyo: true, repeatDelay: 0.52, ease: "none" }, 0.1);

// 1. Ask the palette for a theme switcher; the AI builder writes it.
openPalette(0.2);
tl.set("#q0", { visibility: "hidden" }, 0.62);
tl.set("#q1", { visibility: "visible" }, 0.62);
tl.set("#v0", { visibility: "hidden" }, 0.66);
tl.set("#v1", { visibility: "visible" }, 0.66);
let t = typeQuery("q1", "v1", 0.62, 0.042);
pressEnter("#v1 .plate", t + 0.18);
cutTo(["q1", "v1"], ["q2", "v2"], t + 0.32);
const b = t + 0.32;
tl.fromTo("#b-user", { opacity: 0, y: 8 }, { opacity: 1, y: 0, duration: 0.16, ease: "power3.out" }, b + 0.05);
tl.fromTo("#b-status", { opacity: 0 }, { opacity: 1, duration: 0.1 }, b + 0.2);
tl.fromTo("#b-status .i", { rotate: 0 }, { rotate: 540, duration: 1.4, ease: "none" }, b + 0.2);
tl.fromTo("#b-tool", { opacity: 0, y: 8 }, { opacity: 1, y: 0, duration: 0.16, ease: "power3.out" }, b + 0.3);
document.querySelectorAll("#b-tool .fw").forEach((el, i) => {
  tl.fromTo(el, { opacity: 0, x: -4 }, { opacity: 1, x: 0, duration: 0.12 }, b + 0.5 + i * 0.22);
});
tl.set(["#b-st1", "#b-secs1"], { display: "none" }, b + 1.1);
tl.set(["#b-st2", "#b-secs2"], { display: "inline" }, b + 1.1);
tl.to("#b-status", { opacity: 0, duration: 0.1 }, b + 1.55);
tl.to("#b-sub1", { opacity: 0, duration: 0.1 }, b + 1.5);
tl.to("#b-sub2", { opacity: 1, duration: 0.1 }, b + 1.5);
tl.fromTo("#b-reply", { opacity: 0, y: 6 }, { opacity: 1, y: 0, duration: 0.18, ease: "power3.out" }, b + 1.6);
tl.fromTo("#b-ext", { opacity: 0, y: 6 }, { opacity: 1, y: 0, duration: 0.18, ease: "power3.out" }, b + 1.75);

// 2. The new command works: Toggle Theme switches to light and to the light wallpaper.
const s2 = b + 2.55;
cutTo(["q2", "v2"], ["q3", "v3"], s2);
t = typeQuery("q3", null, s2 + 0.05, 0.04);
pressEnter("#v3 .plate", t + 0.2);
setTheme(DARK, LIGHT, t + 0.28, 0.5);
changeWallpaper("w-night", "w-misty", t + 0.28, 0.5);

// 3. Browse wallpapers: the desktop changes live as the selection moves.
const sw = t + 1.05;
cutTo(["q3", "v3"], ["qw", "vw"], sw);
t = typeQuery("qw", null, sw + 0.05, 0.045);
pressEnter("#vw .plate", t + 0.15);
cutTo(["qw", "vw"], ["qg", "vg"], t + 0.28);
const g0 = t + 0.28;
const browse = [
  [1, 0, "w-misty", "w-alpine"],
  [2, 0, "w-alpine", "w-dunes"],
  [2, 1, "w-dunes", "w-sand"],
  [1, 1, "w-sand", "w-glacier"],
];
browse.forEach(([col, row, from, to], i) => {
  const at = g0 + 0.5 + i * 0.5;
  tl.to("#vg .gplate", { x: col * CELL_W, y: row * CELL_H, duration: 0.08, ease: "power2.out" }, at);
  changeWallpaper(from, to, at + 0.04, 0.3);
});
pressEnter("#vg .gplate", g0 + 2.5);

// 4. Browse pull requests.
const s3 = g0 + 2.8;
cutTo(["qg", "vg"], ["q4", "v4"], s3);
t = typeQuery("q4", null, s3 + 0.05, 0.045);
pressEnter("#v4 .plate", t + 0.15);
cutTo(["q4", "v4"], ["q5", "v5"], t + 0.28);
const p = t + 0.28;
tl.set("#d1", { opacity: 1 }, p);
movePlate("v5", 1, p + 0.7);
tl.set("#d1", { opacity: 0 }, p + 0.7);
tl.set("#d2", { opacity: 1 }, p + 0.7);
movePlate("v5", 2, p + 1.2);
tl.set("#d2", { opacity: 0 }, p + 1.2);
tl.set("#d3", { opacity: 1 }, p + 1.2);

// 5. Browse calendar events.
const s4 = p + 1.8;
cutTo(["q5", "v5"], ["q6", "v6"], s4);
t = typeQuery("q6", null, s4 + 0.05, 0.045);
pressEnter("#v6 .plate", t + 0.15);
cutTo(["q6", "v6"], ["q7", "v7"], t + 0.28);
const e = t + 0.28;
movePlate("v7", 1, e + 0.65);
movePlate("v7", 3, e + 1.1, 26);

// 6. Process the selected videos.
const s5 = e + 1.65;
closePalette(s5);
tl.set(["#q7", "#v7"], { visibility: "hidden" }, s5 + 0.13);
tl.fromTo("#files", { opacity: 0, y: 10, scale: 0.98 }, { opacity: 1, y: 0, scale: 1, duration: 0.2, ease: "power3.out" }, s5 + 0.08);
tl.fromTo("#cursor", { opacity: 0, x: 640, y: 420 }, { opacity: 1, duration: 0.1 }, s5 + 0.2);
["#f1", "#f2", "#f3"].forEach((id, i) => {
  const at = s5 + 0.42 + i * 0.22;
  const travel = i ? 0.16 : 0.2;
  tl.to("#cursor", { x: 520 + i * 6, y: 215 + i * 34, duration: travel, ease: "power2.inOut" }, at - travel);
  tl.fromTo(id + " .sel-bg", { opacity: 0 }, { opacity: 1, duration: 0.05 }, at);
});
tl.to("#cursor", { opacity: 0, duration: 0.12 }, s5 + 1.1);
const s5b = s5 + 1.2;
showState("q8", "v8", s5b);
openPalette(s5b);
t = typeQuery("q8", null, s5b + 0.15, 0.04);
pressEnter("#v8 .plate", t + 0.15);
cutTo(["q8", "v8"], ["q9", "v9"], t + 0.28);
const g = t + 0.32;
tl.fromTo("#p-total", { scaleX: 0 }, { scaleX: 1, duration: 1.7, ease: "none" }, g);
["#s1", "#s2", "#s3"].forEach((id, i) => {
  const st = g + i * 0.38;
  const d = 0.7 + i * 0.1;
  tl.fromTo(id + " .live", { opacity: 0 }, { opacity: 1, duration: 0.05 }, st);
  tl.fromTo(id + " .sbar div", { scaleX: 0 }, { scaleX: 1, duration: d, ease: "power1.inOut" }, st);
  tl.to(id + " .live", { opacity: 0, duration: 0.05 }, st + d);
  tl.to(id + " .ring", { opacity: 0, duration: 0.05 }, st + d);
  tl.fromTo(id + " .done", { opacity: 0, scale: 0.6 }, { opacity: 1, scale: 1, duration: 0.12, ease: "back.out(2)" }, st + d);
  tl.to(id + " .a1", { opacity: 0, duration: 0.08 }, st + d);
  tl.to(id + " .a2", { opacity: 1, duration: 0.08 }, st + d);
});
const done = g + 1.72;
tl.to(["#p-h1a", "#p-h2a"], { opacity: 0, duration: 0.1 }, done);
tl.to(["#p-h1b", "#p-h2b"], { opacity: 1, duration: 0.1 }, done);

// 7. The extension's menu bar item: pick Dark, and the desktop is back where the film started.
const s6 = done + 0.7;
closePalette(s6);
tl.to("#files", { opacity: 0, y: 6, duration: 0.16, ease: "power2.in" }, s6 + 0.04);
tl.fromTo("#cursor", { opacity: 0, x: 620, y: 330 }, { opacity: 1, duration: 0.12 }, s6 + 0.15);
tl.to("#cursor", { x: 968, y: 10, duration: 0.55, ease: "power3.inOut" }, s6 + 0.2);
const click = s6 + 0.82;
tl.fromTo("#mb-theme-hl", { opacity: 0 }, { opacity: 1, duration: 0.05 }, click);
tl.fromTo("#menu", { opacity: 0, scale: 0.96, y: -4 }, { opacity: 1, scale: 1, y: 0, duration: 0.14, ease: "power3.out" }, click + 0.03);
tl.to("#cursor", { x: 950, y: 86, duration: 0.34, ease: "power2.inOut" }, click + 0.3);
tl.set("#menu-hl", { y: 51, opacity: 1 }, click + 0.62);
tl.set("#mi-dark", { attr: { class: "mi hot" } }, click + 0.62);
const pick = click + 1.05;
tl.to("#menu-hl", { opacity: 0, duration: 0.04, yoyo: true, repeat: 1 }, pick);
tl.to("#menu", { opacity: 0, duration: 0.14, ease: "power2.in" }, pick + 0.1);
tl.to("#mb-theme-hl", { opacity: 0, duration: 0.1 }, pick + 0.1);
setTheme(LIGHT, DARK, pick + 0.12, 0.5);
changeWallpaper("w-glacier", "w-night", pick + 0.12, 0.5);
tl.to("#cursor", { opacity: 0, x: 900, y: 170, duration: 0.4, ease: "power2.in" }, pick + 0.5);

window.__timelines = window.__timelines || {};
window.__timelines["main"] = tl;
tl.seek(0);
