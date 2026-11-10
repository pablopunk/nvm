---
name: nevermind-box-signin
description: Use when a Nevermind feature needs a signed-in account (extension AI, decisions, quotas) and must be seen running in the real Electron app on a disposable `box` LXC with a screenshot, instead of on the user's machine. Covers box setup, running the dev app under Xvfb, switching the backend to Production, the device sign-in browser hop, and the disposable `@fldubai.com` account.
---

# Nevermind in a box, signed in

Run the dev app on a throwaway `box` (see the `box` skill) so nothing opens on the user's machine, then sign in to see AI-backed behavior. The repo rule "CI-only runtime verification" is lifted only because the user asked for this run; say so in the handoff.

## 1. Box and app

```sh
BOX_FORCE=1 box new --name nvm-signin --mem 4096 --tools "node@22.22.3 pnpm@11.21.0"   # BOX_FORCE only with the user's OK when MAX_RUNNING is reached
tar --exclude=node_modules --exclude=.git --exclude=.tmp --exclude=out --exclude=dist -cf - . | box exec <id> -- 'mkdir -p /root/nvm && tar -xf - -C /root/nvm'
box exec <id> -- 'cd /root/nvm && git config --global --add safe.directory /root/nvm && git init -q && git add -A >/dev/null && git -c user.email=a@b -c user.name=x commit -qm snap && mise exec -- pnpm install --frozen-lockfile && node scripts/ensure-electron.cjs'
box shot <id> /tmp/x.png          # starts Xvfb on :99
box exec <id> -- 'cd /root/nvm && DISPLAY=:99 nohup mise exec -- node_modules/.bin/electron-vite dev --remoteDebuggingPort 9222 -- --no-sandbox > /root/dev.log 2>&1 &'
```

- The copy has no `.git`; `electron.vite.config.ts` runs `git rev-parse HEAD`, so a throwaway commit in the box is required.
- Skip `pnpm dev`: it also runs DB migrations and the Astro backend. `electron-vite dev` alone is enough; the app talks to a remote backend.
- Drive the palette with `DISPLAY=:99 xdotool type/key` and look with `box shot <id> out.png`. The palette is already visible on Xvfb.
- Kill the box when done (`box kill <id> -f`).

## 2. Point the app at Production

An unpackaged build defaults to `http://localhost:4321`, so "Log in" fails with "Sign-in could not be started". In the palette run `Switch Backend` -> `Production` (`https://api.nvm.fyi`); selecting it starts device sign-in.

## 3. Open the sign-in page inside the box

The app opens `https://www.nvm.fyi/auth/device?code=...` through `xdg-open`. Install a wrapper so it lands in the box's Chromium:

```sh
cat > /usr/local/bin/xdg-open <<'EOF'
#!/bin/sh
echo "$1" > /root/verify-url.txt
DISPLAY=:99 nohup chromium --no-sandbox --user-data-dir=/root/chrome-profile --remote-debugging-port=9333 --window-size=1280,800 "$1" >/root/chromium.log 2>&1 &
EOF
chmod +x /usr/local/bin/xdg-open
```

The page redirects to WorkOS AuthKit (Production client `client_01KSQEK0H6S03V6W5G98QCMP5P`): email + magic code, Google, and a Sign up link. Click through with `xdotool mousemove X Y click 1` using coordinates from `box shot`.

## 4. Disposable account and inbox (verified)

`@fldubai.com` is a mail.tm domain, so the inbox is readable over their public API (no auth to create). Create one per run:

```sh
DOM=$(curl -s https://api.mail.tm/domains | python3 -c "import sys,json;print(json.load(sys.stdin)['hydra:member'][0]['domain'])")
U="nvmbox$(date +%s)@$DOM"; P=$(head -c 9 /dev/urandom | od -An -tx1 | tr -d ' \n')
curl -s -X POST https://api.mail.tm/accounts -H 'Content-Type: application/json' -d "{\"address\":\"$U\",\"password\":\"$P\"}"
TOK=$(curl -s -X POST https://api.mail.tm/token -H 'Content-Type: application/json' -d "{\"address\":\"$U\",\"password\":\"$P\"}" | python3 -c "import sys,json;print(json.load(sys.stdin)['token'])")
curl -s https://api.mail.tm/messages -H "Authorization: Bearer $TOK"   # poll; the WorkOS one-time code is in `intro`
```

Keep the address, password and token in the scratchpad, never in the repo. Sign-up (first name `Box`, last name `Tester`) emails "Your one-time code is: NNNNNN" from `access@workos-mail.com`; it expires in 10 minutes.

## 5. Human verification: the user clicks it

Sign-up shows a Cloudflare Turnstile "Verify you are human" challenge on the AuthKit page. Do not click, solve, or script around it. Give the user a remote desktop into the box so they complete it themselves:

```sh
box exec <id> -- 'apt-get update -qq && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq x11vnc novnc websockify'
PW=$(head -c 6 /dev/urandom | od -An -tx1 | tr -d " \n")
box exec <id> -- "x11vnc -storepasswd $PW /root/vncpass; (nohup x11vnc -display :99 -rfbauth /root/vncpass -forever -shared -rfbport 5900 -quiet >/root/x11vnc.log 2>&1 &); (nohup websockify --web /usr/share/novnc 6080 localhost:5900 >/root/novnc.log 2>&1 &)"
```

Send the user `http://<box-ip>:6080/vnc.html?autoconnect=1` and the password (LAN-only, random per run). Tell them to use your disposable address and Sign up; when they say they submitted, fetch the code from the inbox and send it back for them to type. Do not use WorkOS admin tools to create the user (the user ruled that out).

## 6. Back in the app

After sign-in the palette hides on blur and Xvfb shows black. Bring it back by launching a second instance (single-instance lock shows the palette):

```sh
box exec <id> -- 'cd /root/nvm && DISPLAY=:99 timeout 20 $(ls -d node_modules/.pnpm/electron@*/node_modules/electron/dist/electron) . --no-sandbox'
```

Then type with xdotool and screenshot with `box shot`. Confirm sign-in from `"authed": true` in `/root/dev.log`.

## Handoff

State: environment used (Production backend, which WorkOS client), the account email created, whether it was deleted, and the strongest boundary reached (e.g. "reached Turnstile on sign-up; not signed in").
