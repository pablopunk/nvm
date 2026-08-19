# Backend API sunset template

Use this document when a real breaking change requires a new backend API major such as `/api/v2`.

## Breaking change

- Current API major:
- New API major:
- Breaking change summary:
- Why additive fields, feature flags, or compatibility shims are insufficient:

## Client count evidence

Before setting a sunset date, query `desktop_client_seen` logs for the last 30 and 90 days and record active clients by desktop version and requested API major.

- Log event: `desktop_client_seen`
- Required fields: `client_version`, `client_api_version`, `client_platform`, `client_arch`, `compatible`
- 30-day active clients on old major:
- 90-day active clients on old major:
- Supported desktop versions still on old major:

## Sunset

- First deprecation notice date:
- Forced-update date:
- Final backend removal date:
- Owner:
- Rollback plan:

## User-visible update message

Title: Update Nevermind

Message: This version of Nevermind uses an older backend API that will stop working on <date>. Install the latest Nevermind to keep using AI features.

Primary action: Check for Update

Fallback URL: https://github.com/pablopunk/nvm/releases/latest
