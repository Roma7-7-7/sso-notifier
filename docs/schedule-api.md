# Schedule API + SwiftBar menu bar plugin

A small standalone service (`cmd/scheduleapi`) that scrapes the schedule on an interval, caches the last
successful result and serves it as JSON. A SwiftBar plugin on macOS polls it and shows the current status
and time to the next change in the menu bar:

```
🟢 1h 19m          <- power ON, 1h 19m until the next change
🔴 44m+            <- "+" = schedule ends there, tomorrow not published yet
⚠️ no data         <- API unreachable / nothing fetched yet
⚠️ 🟢 44m          <- data is stale (older than STALE_AFTER_MIN)
🚨 emergency       <- utility is in emergency mode, schedule suspended
```

It is independent of the bot: no Telegram token, no database.

## Run the API

```bash
make schedule-api-start     # builds, starts detached (nohup), pid/log in bin/
make schedule-api-status
make schedule-api-logs
make schedule-api-restart   # after changing env vars
make schedule-api-stop
```

| Env var | Default | Description |
|---------|---------|-------------|
| `SCHEDULE_API_ADDR` | `127.0.0.1:47653` | Listen address. Loopback only by default; pick another port if it clashes |
| `REFRESH_SHUTDOWNS_INTERVAL` | `5m` | How often to scrape |
| `SCHEDULE_URL` | `https://oblenergo.cv.ua/shutdowns/` | Schedule page |
| `PROXY_TOKEN` | | Same meaning as in the bot |
| `DEV` | `false` | Text logs instead of JSON |

Example: `SCHEDULE_API_ADDR=127.0.0.1:5555 make schedule-api-start`. The process does not survive a reboot;
run `make schedule-api-start` again.

## Endpoint

`GET /api/schedule` returns all groups, today and tomorrow (when published), as merged ranges with Kyiv offsets:

```json
{
  "fetched_at": "2025-11-05T10:00:00+02:00",
  "last_attempt_at": "2025-11-05T10:05:00+02:00",
  "last_error": "fetch today: ...",
  "emergency": false,
  "groups": {
    "4": [{ "start": "2025-11-05T00:00:00+02:00", "end": "2025-11-05T08:00:00+02:00", "status": "ON" }]
  }
}
```

- `status` is `ON`, `OFF` or `MAYBE`. Adjacent ranges with the same status are merged, including across midnight.
- On a failed refresh the last good data is kept and `last_error` is set. Clients should compare
  `fetched_at` with the current time to detect stale data.
- `503` with an empty `groups` means nothing has been fetched successfully yet.

## SwiftBar plugin

Requires macOS with `python3` (stdlib only; the Xcode Command Line Tools one is fine).

1. Install SwiftBar: `brew install --cask swiftbar`, and choose a plugin folder on first launch.
2. Link the plugin into that folder (the `1m` in the file name is the refresh rate):
   ```bash
   chmod +x scripts/swiftbar/outage.1m.py
   ln -s "$PWD/scripts/swiftbar/outage.1m.py" "<your SwiftBar plugin folder>/"
   ```
3. Start the API (`make schedule-api-start`) and click SwiftBar → Refresh All.

Settings are environment variables declared in the plugin header (`<swiftbar.environment>`); edit them via
SwiftBar → the plugin → Preferences, or in the file:

| Variable | Default | Description |
|----------|---------|-------------|
| `SCHEDULE_API_URL` | `http://127.0.0.1:47653/api/schedule` | Must match `SCHEDULE_API_ADDR` |
| `GROUP` | `4` | Your group number (1-12) |
| `STALE_AFTER_MIN` | `20` | Show ⚠️ when data is older than this |
| `STYLE` | `pill` | `pill` draws the title as an image on a dark rounded background (readable on any wallpaper); `text` uses plain colored text |

In `pill` mode the title is a generated PNG (status dot + countdown; `!` marks stale data or an error, details are in the dropdown).

The countdown is computed locally from the cached ranges on every refresh, so it stays accurate between scrapes.
If the menu bar shows a red dot with `!` (`⚠️ no data` in `text` mode), open the dropdown for the reason; if the
API is down, run `make schedule-api-start`.

Security: the plugin only reads JSON and prints text; response fields are never used in `bash=`/`href=` parameters.
Keep the API on loopback (or behind HTTPS if you ever move it off the laptop).
