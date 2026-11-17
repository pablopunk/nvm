// Builds the palette's pages from the Go app's components: SearchRow, Row, Tile, Chip, Footer and Keycap in
// src/native/internal/design, and the chat page of src/native/internal/palette/aichat. Every name, repo, event and
// file is made up.

function chars(text, key) {
  const spans = [...text].map((c) => '<span class="ch">' + (c === " " ? "&nbsp;" : c) + "</span>").join("");
  return '<span class="type" data-key="' + key + '">' + spans + "</span>";
}

function search(d) {
  let field;
  if (d.title) field = '<div class="field">' + d.title + "</div>";
  else if (d.key) field = '<div class="field">' + chars(d.text, d.key) + '<span class="caret"></span></div>';
  else field = '<div class="field"><span class="caret"></span><span class="ph">' + d.ph + "</span></div>";
  const chip = d.chip ? '<div class="chip"><span class="i" data-i="sparkles"></span><b>Tab</b>to ask AI</div>' : "";
  return '<div class="search"><span class="i zap" data-i="zap"></span>' + field + chip + "</div>";
}

function tile(t) {
  if (t.app) return '<div class="tile app"><img src="assets/apps/' + t.app + '.png" alt="" /></div>';
  if (t.img) return '<div class="tile img" style="background-image: url(' + t.img + ')"></div>';
  if (t.prom) return '<div class="tile key prom"><span class="i" data-i="zap"></span></div>';
  return '<div class="tile key' + (t.tone ? " tone-" + t.tone : "") + '"><span class="i" data-i="' + t.icon + '"></span></div>';
}

function row(r) {
  const cls = "row" + (r.prom ? " prom" : "") + (r.sel ? " sel" : "") + (r.tone && r.tint ? " tint-" + r.tone : "");
  const sub = r.sub ? '<div class="s"' + (r.subId ? ' id="' + r.subId + '"' : "") + ">" + r.sub + "</div>" : "";
  const acc = r.acc ? '<span class="acc"' + (r.accId ? ' id="' + r.accId + '"' : "") + ">" + r.acc + "</span>" : "";
  return '<div class="' + cls + '" data-base="' + cls.replace(" sel", "") + '"' + (r.id ? ' id="' + r.id + '"' : "") + ">" +
    tile(r.tile || { prom: r.prom }) + '<div class="tx"><div class="t">' + r.title + "</div>" + sub + "</div>" + acc +
    '<span class="i enter" data-i="corner-down-left"></span></div>';
}

function section(title) {
  return '<div class="section">' + title + "</div>";
}

function hint(h) {
  const cls = "hint" + (h.emph ? " emph" : "");
  return '<span class="' + cls + '"' + (h.id ? ' id="' + h.id + '"' : "") + ">" + h.title + '<span class="keycap">' + h.key + "</span></span>";
}

function footer(f) {
  const name = f.name ? '<div class="plate name"><span class="i" data-i="zap"></span><span>' + f.name + "</span></div>" : "";
  const control = f.model ? '<div class="plate model"><b>' + f.model + '</b><span class="i" data-i="chevrons-up-down"></span></div>' : "";
  const primary = f.primary.map(hint).join("");
  return '<div class="footer">' + name + control + '<div class="grow"></div><div class="plate hints">' + primary +
    hint({ title: "Actions", key: "⌘K" }) + "</div></div>";
}

function view(id, parts) {
  return '<div class="view" id="' + id + '">' + parts.join("") + "</div>";
}

const automate = (key, text) => ({ title: 'Automate "' + chars(text, key) + '"', sub: "Build a reusable command with AI", prom: true, tone: "yellow", tint: true });

const THEMES = [
  ["Midnight", "Dark · no wallpaper", null],
  ["Sunset", "Light", "sunset"],
  ["Deep Ocean", "Dark", "ocean"],
  ["Terraces", "Light", "terraces"],
  ["Dusk", "Dark", "dusk"],
  ["Lagoon", "Light", "lagoon"],
];

function grid() {
  const cells = THEMES.map(([title, sub, img], i) => {
    const pic = img ? '<div class="pic" style="background-image: url(assets/wallpapers/' + img + '.webp)"></div>' : '<div class="pic black"></div>';
    return '<div class="cell" id="cell' + i + '" data-base="cell">' + pic + '<div class="ct">' + title + '</div><div class="cs">' + sub + "</div></div>";
  });
  return '<div class="grid">' + cells.join("") + "</div>";
}

// A text that changes over time is one span per state; the timeline shows one at a time.
function states(id, texts) {
  return texts.map((t, k) => '<span class="st" id="' + id + "-" + k + '">' + t + "</span>").join("");
}

function chatPage() {
  const ask = "theme switcher with wallpapers";
  const tool =
    '<div class="tool" id="tool"><div class="tl"><span class="i" data-i="sparkles"></span><b>Writing extension</b><span class="m">user.theme-switcher</span></div>' +
    '<div class="ts" id="tool-run">Writing main.go…</div><div class="ts" id="tool-done">Installed and enabled</div></div>';
  const answer = "Theme Switcher is installed. Each theme sets the look and the wallpaper. Pick one here or from the menu bar.";
  const words = answer.split(" ").map((w) => '<span class="w">' + w + " </span>").join("");
  return view("v-chat", [
    '<div class="chat"><div class="bubble" id="bubble">' + ask + "</div>" + tool + '<p class="answer" id="answer">' + words + "</p></div>",
    '<div class="checking" id="checking"><span class="i spin" data-i="loader-circle"></span><span class="a">Checking…</span><span class="m">1s</span></div>',
    '<div class="composer"><span class="ph">Describe the extension you want</span></div>',
    footer({ name: ask, model: "Fast", primary: [{ title: "Send", key: "↵", emph: true }] }),
  ]);
}

const PRS = [
  ["git-pull-request", "green", "Remember window size per display", "acme/desktop #214 · 2h ago", "Approved"],
  ["git-pull-request", "green", "Add keyboard navigation to the grid", "acme/desktop #211 · 5h ago", "Review requested"],
  ["git-pull-request-draft", "gray", "Sync settings across devices", "acme/desktop #209 · yesterday", "Draft"],
  ["git-pull-request", "green", "Cache app icons on first launch", "acme/launcher #88 · yesterday", "Changes requested"],
  ["git-merge", "purple", "Faster fuzzy matching for long lists", "acme/search #57 · 2 days ago", "Merged"],
  ["git-pull-request", "green", "Fix focus after closing a panel", "acme/desktop #205 · 3 days ago", "Approved"],
  ["git-merge", "purple", "Ship the new onboarding copy", "acme/site #31 · 4 days ago", "Merged"],
  ["git-pull-request", "green", "Retry downloads on flaky networks", "acme/updater #19 · 5 days ago", "Checks passing"],
];

const VIDEOS = [
  ["coast.mov", "sandscape", "1.4 GB", "286 MB"],
  ["forest-walk.mov", "fog-forest", "2.1 GB", "402 MB"],
  ["night-drive.mov", "night-lake", "860 MB", "174 MB"],
];

function buildPalette(el) {
  const pages = [
    view("v-root", [
      search({ ph: "Watcha gonna do?" }),
      '<div class="list">',
      row({ title: "Safari", tile: { app: "safari" }, sel: true }),
      row({ title: "Messages", tile: { app: "messages" } }),
      row({ title: "Notes", tile: { app: "notes" } }),
      row({ title: "Clipboard History", sub: "Browse and paste what you copied", tile: { icon: "clipboard", tone: "blue" } }),
      row({ title: "Extensions", sub: "Install, create and manage extensions", tile: { icon: "puzzle", tone: "green" } }),
      "</div>",
      footer({ primary: [{ title: "Open Safari", key: "↵" }] }),
    ]),
    view("v-ai", [
      search({ key: "q1", text: "theme switcher with wallpapers", chip: true }),
      '<div class="list">',
      row({ ...automate("q1", "theme switcher with wallpapers"), sel: true, id: "ai-row" }),
      row({ title: 'Search the web for "' + chars("theme switcher with wallpapers", "q1") + '"', sub: "Search instead", tile: { icon: "search" } }),
      "</div>",
      footer({ primary: [{ title: "Automate", key: "↵" }] }),
    ]),
    chatPage(),
    view("v-theme-q", [
      search({ key: "q2", text: "theme", chip: true }),
      '<div class="list">',
      row({ title: "Theme Switcher", sub: "Pick a theme", tile: { icon: "palette", tone: "purple" }, sel: true }),
      row({ title: "Next Theme", sub: "Theme Switcher", tile: { icon: "shuffle", tone: "purple" } }),
      row(automate("q2", "theme")),
      "</div>",
      footer({ primary: [{ title: "Open Theme Switcher", key: "↵" }] }),
    ]),
    view("v-grid", [search({ ph: "Search themes" }), grid(), footer({ name: "Theme Switcher", primary: [{ title: "Apply Theme", key: "↵" }] })]),
    view("v-pr-q", [
      search({ key: "q3", text: "pull", chip: true }),
      '<div class="list">',
      row({ title: "My Pull Requests", sub: "GitHub", tile: { icon: "git-pull-request", tone: "blue" }, sel: true }),
      row({ title: "Review Requests", sub: "GitHub", tile: { icon: "git-pull-request", tone: "blue" } }),
      row(automate("q3", "pull")),
      "</div>",
      footer({ primary: [{ title: "Open My Pull Requests", key: "↵" }] }),
    ]),
    view("v-pr", [
      search({ ph: "Search pull requests" }),
      '<div class="list" id="pr-list">',
      ...PRS.map(([icon, tone, title, sub, acc], i) => row({ title, sub, acc, tile: { icon, tone }, sel: i === 0 })),
      "</div>",
      footer({ name: "My Pull Requests", primary: [{ title: "Open in Browser", key: "↵" }] }),
    ]),
    view("v-cal-q", [
      search({ key: "q4", text: "cal", chip: true }),
      '<div class="list" id="calq-list">',
      row({ title: "Calendar", tile: { app: "calendar" }, sel: true }),
      row({ title: "Calculator", tile: { app: "calculator" } }),
      row({ title: "Upcoming Events", sub: "Calendar", tile: { icon: "calendar", tone: "red" } }),
      row(automate("q4", "cal")),
      "</div>",
      footer({ primary: [{ title: "Open Calendar", key: "↵", id: "calq-h0" }, { title: "Open Calculator", key: "↵", id: "calq-h1" }, { title: "Open Upcoming Events", key: "↵", id: "calq-h2" }] }),
    ]),
    view("v-cal", [
      search({ ph: "Search events" }),
      '<div class="list" id="cal-list">',
      section("Today"),
      row({ title: "Design review", sub: "10:00 – 10:45 · Video call", acc: "in 20 min", tile: { icon: "video", tone: "blue" }, sel: true }),
      row({ title: "Lunch with Sam", sub: "13:00 – 14:00 · Café Lumen", acc: "in 3 h", tile: { icon: "utensils", tone: "orange" } }),
      row({ title: "Release planning", sub: "16:30 – 17:00 · Room 4B", acc: "in 6 h", tile: { icon: "calendar", tone: "red" } }),
      section("Tomorrow"),
      row({ title: "Team standup", sub: "09:30 – 09:45 · Video call", acc: "Wed", tile: { icon: "video", tone: "blue" } }),
      row({ title: "Dentist", sub: "11:15 – 12:00", acc: "Wed", tile: { icon: "calendar", tone: "red" } }),
      "</div>",
      footer({ name: "Upcoming Events", primary: [{ title: "Join Video Call", key: "↵", id: "cal-h0" }, { title: "Open in Calendar", key: "↵", id: "cal-h1" }] }),
    ]),
    view("v-vid-q", [
      search({ key: "q5", text: "compress", chip: true }),
      '<div class="list">',
      row({ title: "Compress Videos", sub: "3 videos selected in Finder", tile: { icon: "film", tone: "orange" }, sel: true }),
      row(automate("q5", "compress")),
      "</div>",
      footer({ primary: [{ title: "Compress Videos", key: "↵" }] }),
    ]),
    view("v-vid", [
      search({ title: "Compress Videos" }),
      '<div class="list" id="vid-list">',
      ...VIDEOS.map(([name, img, from, to], i) =>
        row({
          title: name,
          sub: states("vs" + i, [from + " · Waiting", "Compressing · 21%", "Compressing · 48%", "Compressing · 76%", "Compressing · 93%", from + " → " + to]),
          acc: states("va" + i, ["", '<span class="i spin" data-i="loader-circle"></span>', '<span class="done"><span class="i" data-i="circle-check"></span>Done</span>']),
          tile: { img: "assets/wallpapers/" + img + ".webp" },
          sel: i === 0,
        }),
      ),
      '<div class="summary" id="vid-done"><span class="i" data-i="circle-check"></span>3 videos compressed · 3.5 GB saved</div>',
      "</div>",
      footer({ name: "Compress Videos", primary: [{ title: "Show in Finder", key: "↵" }] }),
    ]),
  ];
  el.innerHTML = pages.join("");
}
