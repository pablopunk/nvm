// The page set-up and the helpers the timeline is written with. Every helper adds tweens or sets to one paused GSAP
// timeline, so each state is a function of timeline time and HyperFrames can seek to any frame.

const stage = document.getElementById("stage");
buildPalette(document.getElementById("palette"));
hydrateIcons(document);
for (const [k, v] of Object.entries(DARK)) stage.style.setProperty(k, v);

// Each Dock icon gets a tint layer cut to its own shape; the Nevermind icon is a rounded square.
for (const dk of document.querySelectorAll(".dk")) {
  const tint = document.createElement("i");
  tint.className = "tint";
  if (!dk.classList.contains("nvm")) {
    const mask = "url(" + dk.querySelector("img").getAttribute("src") + ")";
    tint.style.webkitMaskImage = mask;
    tint.style.maskImage = mask;
  }
  dk.appendChild(tint);
}

// Finder's icon view: four icons a row. Before compressing, the four videos sit side by side; after, each compressed
// copy sits right after its original.
const FILES_BEFORE = { fi1: 0, fi2: 1, fi3: 2, fi4: 3, fc1: 1, fc2: 3, fc3: 5 };
const FILES_AFTER = { fi1: 0, fc1: 1, fi2: 2, fc2: 3, fi3: 4, fc3: 5, fi4: 6 };
function slot(n) {
  return { left: (n % 4) * 150, top: Math.floor(n / 4) * 124 };
}
for (const [id, n] of Object.entries(FILES_BEFORE)) Object.assign(document.getElementById(id).style, { left: slot(n).left + "px", top: slot(n).top + "px" });

// Points on the 1100 × 619 screen, measured before any tween moves things.
const SCALE = 1920 / 1100;
const stageBox = stage.getBoundingClientRect();
function spot(el, fx = 0.5, fy = 0.5) {
  const r = (typeof el === "string" ? document.querySelector(el) : el).getBoundingClientRect();
  return { x: (r.left - stageBox.left + r.width * fx) / SCALE, y: (r.top - stageBox.top + r.height * fy) / SCALE };
}

const tl = gsap.timeline({ paused: true, defaults: { immediateRender: false } });

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

// A theme is a look, a wallpaper and a Dock tint. The tint sweeps across the Dock from left to right.
// The menu bar text flips at the midpoint instead of fading, so it never sits grey on a grey bar mid-change.
function setTheme(to, t, dur) {
  const { "--bar-fg": barText, "--bar-glow": barGlow, ...rest } = to;
  tl.to("#stage", { ...rest, duration: dur, ease: "power1.inOut" }, t);
  tl.set("#stage", { "--bar-fg": barText, "--bar-glow": barGlow }, t + dur / 2);
}
function tintDock(color, t) {
  tl.to(".dk .tint", { backgroundColor: color, duration: 0.28, ease: "power1.inOut", stagger: 0.04 }, t);
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

// The camera frames the screen point focus at zoom z, without showing anything past the screen's edges. Moves must
// not overlap in time.
const CENTER = { x: 550, y: 309.5 };
function camera(t, dur, z, focus = CENTER, ease = "power2.inOut") {
  const clamp = (v, lo) => Math.min(0, Math.max(lo, v));
  const x = clamp(960 - focus.x * SCALE * z, 1920 - 1920 * z);
  const y = clamp(540 - focus.y * SCALE * z, 1080 - 1080 * z);
  tl.to("#cam", { scale: z, x, y, duration: dur, ease }, t);
}
