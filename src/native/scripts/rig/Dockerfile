# L4 rig image: the real app runs here under Xvfb with openbox, picom, an xfce4-panel tray host, a
# dbus session, PulseAudio with a virtual microphone, and xdotool/xclip for input. Build context is
# src/native (see scripts/rig/rig.sh and scripts/rig/README.md).
FROM debian:trixie-slim

ARG GO_VERSION=1.27.3
ENV DEBIAN_FRONTEND=noninteractive \
    LANG=C.UTF-8 \
    PATH=/usr/local/go/bin:/root/go/bin:$PATH \
    GOTOOLCHAIN=local \
    RIG_DISPLAY=:99 \
    RIG_WALLPAPER=/opt/rig/wall.png

# Desktop and input tools, then the runtime libraries MyGo loads with purego (GTK 3, cairo, pango,
# atk, epoxy, libsecret, the appindicator tray, dbusmenu, layer-shell, WebKit for web views) and GL
# for software rendering. The list mirrors the shared test box, which is the reference for the rig.
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      ca-certificates curl tar xz-utils bash procps \
      xvfb openbox picom xfce4-panel xfconf dbus-x11 \
      pulseaudio pulseaudio-utils \
      xdotool xclip xauth x11-utils x11-xserver-utils feh imagemagick mousepad \
      fonts-dejavu-core fonts-liberation fonts-noto-color-emoji \
      libgtk-3-0t64 libcairo2 libpango-1.0-0 libatk1.0-0t64 libepoxy0 libsecret-1-0 \
      libayatana-appindicator3-1 libdbusmenu-gtk3-4 libgtk-layer-shell0 \
      libwebkit2gtk-4.1-0 \
      libgl1 libgl1-mesa-dri libglx-mesa0 libegl1 libegl-mesa0 libgbm1 \
      libxkbcommon0 libxkbcommon-x11-0 libxcursor1 libxrandr2 libxi6 libxss1 libxtst6 \
      libxcomposite1 libxdamage1 libxfixes3 libxinerama1 \
 && rm -rf /var/lib/apt/lists/*

# The Go toolchain named by src/native/go.mod.
RUN arch="$(dpkg --print-architecture)" \
 && curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${arch}.tar.gz" | tar -C /usr/local -xz \
 && go version

COPY scripts/rig/entrypoint.sh /usr/local/bin/rig-entrypoint
COPY scripts/rig/assets/wall.png /opt/rig/wall.png
RUN chmod +x /usr/local/bin/rig-entrypoint

WORKDIR /work
ENTRYPOINT ["/usr/local/bin/rig-entrypoint"]
CMD ["sleep", "infinity"]
