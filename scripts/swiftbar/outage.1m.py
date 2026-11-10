#!/usr/bin/env python3
# <swiftbar.title>Power outage status</swiftbar.title>
# <swiftbar.desc>Current power status and time to the next change, from the sso-notifier schedule API</swiftbar.desc>
# <swiftbar.hideAbout>true</swiftbar.hideAbout>
# <swiftbar.hideRunInTerminal>true</swiftbar.hideRunInTerminal>
# <swiftbar.hideLastUpdated>true</swiftbar.hideLastUpdated>
# <swiftbar.hideDisablePlugin>true</swiftbar.hideDisablePlugin>
# <swiftbar.hideSwiftBar>true</swiftbar.hideSwiftBar>
# <swiftbar.environment>[SCHEDULE_API_URL=http://127.0.0.1:47653/api/schedule, GROUP=4, STALE_AFTER_MIN=20]</swiftbar.environment>

import json
import os
import re
import sys
import urllib.error
import urllib.request
from datetime import datetime, timedelta, timezone

URL = os.environ.get("SCHEDULE_API_URL", "http://127.0.0.1:47653/api/schedule")
GROUP = os.environ.get("GROUP", "4")
STALE_AFTER = timedelta(minutes=int(os.environ.get("STALE_AFTER_MIN", "20")))
UPCOMING_SHOWN = 8

ICONS = {"ON": "🟢", "OFF": "🔴", "MAYBE": "🟡"}
LABELS = {"ON": "Power ON", "OFF": "Power OFF", "MAYBE": "Maybe OFF"}
COLORS = {"ON": "green", "OFF": "red", "MAYBE": "orange"}


def parse_time(value):
    # fromisoformat before 3.11 rejects "Z" and some fraction lengths
    value = re.sub(r"\.\d+", "", value).replace("Z", "+00:00")
    return datetime.fromisoformat(value)


def fmt_duration(delta):
    minutes = max(int(delta.total_seconds() // 60), 0)
    hours, minutes = divmod(minutes, 60)
    if hours:
        return "%dh %02dm" % (hours, minutes)
    return "%dm" % minutes


def fmt_clock(moment):
    return moment.astimezone().strftime("%H:%M")


def warn(title, *details):
    print("⚠️ %s | color=red" % title)
    print("---")
    for line in details:
        print("%s | color=gray" % line)
    print("Refresh | refresh=true")
    sys.exit(0)


def fetch():
    try:
        with urllib.request.urlopen(URL, timeout=5) as resp:
            return json.load(resp)
    except urllib.error.HTTPError as exc:
        # 503 means the API is up but has not fetched a schedule yet
        try:
            detail = json.load(exc).get("last_error") or "no schedule fetched yet"
        except ValueError:
            detail = str(exc)
        warn("no data", "API responded %s: %s" % (exc.code, detail), URL)
    except (urllib.error.URLError, OSError, ValueError) as exc:
        warn("no data", "Schedule API is unreachable: %s" % exc, URL, "Restart: make schedule-api-start")


def main():
    data = fetch()
    now = datetime.now(timezone.utc)

    if data.get("emergency"):
        print("🚨 emergency | color=red")
        print("---")
        print("Schedule is suspended (emergency mode)")
        print("Refresh | refresh=true")
        return

    intervals = []
    for item in data.get("groups", {}).get(GROUP, []):
        intervals.append((parse_time(item["start"]), parse_time(item["end"]), item["status"]))

    current = next((i for i, (start, end, _) in enumerate(intervals) if start <= now < end), None)
    if current is None:
        warn("no schedule", "No interval covers the current time for group %s" % GROUP, URL)

    start, end, status = intervals[current]
    following = intervals[current + 1] if current + 1 < len(intervals) else None
    # without a contiguous next interval the real change time is unknown (e.g. tomorrow not published)
    change_known = following is not None and following[0] == end

    fetched_at = parse_time(data["fetched_at"])
    stale = now - fetched_at > STALE_AFTER

    remaining = fmt_duration(end - now) + ("" if change_known else "+")
    title = "%s %s" % (ICONS[status], remaining)
    if stale:
        title = "⚠️ " + title
    print("%s | color=%s" % (title, COLORS[status]))

    print("---")
    print("%s until %s | color=%s" % (LABELS[status], fmt_clock(end), COLORS[status]))
    if change_known:
        print("Then: %s (in %s)" % (LABELS[following[2]], fmt_duration(end - now)))
    else:
        print("Schedule ends at %s, next day not published yet | color=gray" % fmt_clock(end))

    if stale:
        print("---")
        print("⚠️ Data is stale: last updated %s ago | color=red" % fmt_duration(now - fetched_at))
        if data.get("last_error"):
            print("%s | color=gray" % data["last_error"])

    upcoming = intervals[current + 1 : current + 1 + UPCOMING_SHOWN]
    if upcoming:
        print("---")
        print("Group %s" % GROUP)
    for start, end, status in upcoming:
        print("%s  %s–%s  %s | color=%s font=Menlo" % (ICONS[status], fmt_clock(start), fmt_clock(end), status, COLORS[status]))

    print("---")
    print("Updated %s | color=gray" % fmt_clock(fetched_at))
    print("Refresh | refresh=true")


if __name__ == "__main__":
    main()
