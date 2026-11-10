#!/usr/bin/env python3
# <swiftbar.title>Power outage status</swiftbar.title>
# <swiftbar.desc>Current power status and time to the next change, from the sso-notifier schedule API</swiftbar.desc>
# <swiftbar.hideAbout>true</swiftbar.hideAbout>
# <swiftbar.hideRunInTerminal>true</swiftbar.hideRunInTerminal>
# <swiftbar.hideLastUpdated>true</swiftbar.hideLastUpdated>
# <swiftbar.hideDisablePlugin>true</swiftbar.hideDisablePlugin>
# <swiftbar.hideSwiftBar>true</swiftbar.hideSwiftBar>
# <swiftbar.environment>[SCHEDULE_API_URL=http://127.0.0.1:47653/api/schedule, GROUP=4, STALE_AFTER_MIN=20, STYLE=pill]</swiftbar.environment>

import base64
import json
import os
import re
import struct
import sys
import urllib.error
import urllib.request
import zlib
from datetime import datetime, timedelta, timezone

URL = os.environ.get("SCHEDULE_API_URL", "http://127.0.0.1:47653/api/schedule")
GROUP = os.environ.get("GROUP", "4")
STALE_AFTER = timedelta(minutes=int(os.environ.get("STALE_AFTER_MIN", "20")))
# "pill" draws the menu bar title as an image on a dark background so it is readable on any wallpaper
STYLE = os.environ.get("STYLE", "pill")
UPCOMING_SHOWN = 8

ICONS = {"ON": "🟢", "OFF": "🔴", "MAYBE": "🟡"}
LABELS = {"ON": "Power ON", "OFF": "Power OFF", "MAYBE": "Maybe OFF"}
COLORS = {"ON": "green", "OFF": "red", "MAYBE": "orange"}

PILL_BG = (28, 28, 30)
PILL_FG = (255, 255, 255)
PILL_DOTS = {"ON": (52, 199, 89), "OFF": (255, 69, 58), "MAYBE": (255, 159, 10), "ERR": (255, 69, 58)}

# 5x7 pixel font, only the glyphs the title needs
FONT = {
    "0": " ### |#   #|#  ##|# # #|##  #|#   #| ### ",
    "1": "  #  | ##  |  #  |  #  |  #  |  #  | ### ",
    "2": " ### |#   #|    #|   # |  #  | #   |#####",
    "3": " ### |#   #|    #|  ## |    #|#   #| ### ",
    "4": "   # |  ## | # # |#  # |#####|   # |   # ",
    "5": "#####|#    |#### |    #|    #|#   #| ### ",
    "6": "  ## | #   |#    |#### |#   #|#   #| ### ",
    "7": "#####|    #|   # |  #  | #   | #   | #   ",
    "8": " ### |#   #|#   #| ### |#   #|#   #| ### ",
    "9": " ### |#   #|#   #| ####|    #|   # | ##  ",
    "h": "#    |#    |#### |#   #|#   #|#   #|#   #",
    "m": "     |     |## # |# # #|# # #|# # #|# # #",
    "+": "     |  #  |  #  |#####|  #  |  #  |     ",
    "!": "  #  |  #  |  #  |  #  |  #  |     |  #  ",
    "?": " ### |#   #|    #|   # |  #  |     |  #  ",
}


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


def png_chunk(tag, data):
    return struct.pack(">I", len(data)) + tag + data + struct.pack(">I", zlib.crc32(tag + data) & 0xFFFFFFFF)


def pill_png(text, dot):
    """Renders a dark rounded pill with a status dot and white text as a base64 PNG (drawn at 2x for retina)."""
    scale, pad_y, pad_x, dot_d, gap = 3, 7, 12, 12, 6

    x = pad_x + dot_d + gap
    glyphs = []
    for ch in text:
        if ch != " ":
            glyphs.append((x, FONT.get(ch, FONT["?"]).split("|")))
        x += (3 if ch == " " else 6) * scale
    width = x - scale + pad_x
    height = 7 * scale + 2 * pad_y

    radius = height / 2
    dot_r = dot_d / 2
    dot_cx = pad_x + dot_r
    rows = []
    for py in range(height):
        row = bytearray(width * 4)
        for px in range(width):
            cx, cy = px + 0.5, py + 0.5
            edge = ((cx - min(max(cx, radius), width - radius)) ** 2 + (cy - radius) ** 2) ** 0.5 - radius
            alpha = min(max(0.5 - edge, 0.0), 1.0)
            cover = min(max(0.5 - (((cx - dot_cx) ** 2 + (cy - radius) ** 2) ** 0.5 - dot_r), 0.0), 1.0)
            color = [round(b * (1 - cover) + d * cover) for b, d in zip(PILL_BG, dot)]
            row[px * 4 : px * 4 + 4] = bytes(color + [round(alpha * 255)])
        rows.append(row)

    for gx, glyph in glyphs:
        for gy, line in enumerate(glyph):
            for col, mark in enumerate(line):
                if mark != "#":
                    continue
                for dy in range(scale):
                    for dx in range(scale):
                        px, py = gx + col * scale + dx, pad_y + gy * scale + dy
                        rows[py][px * 4 : px * 4 + 4] = bytes(PILL_FG + (255,))

    png = b"\x89PNG\r\n\x1a\n"
    png += png_chunk(b"IHDR", struct.pack(">IIBBBBB", width, height, 8, 6, 0, 0, 0))
    png += png_chunk(b"pHYs", struct.pack(">IIB", 5669, 5669, 1))  # 144 dpi, so it is drawn at half the pixel size
    png += png_chunk(b"IDAT", zlib.compress(b"".join(b"\x00" + bytes(r) for r in rows), 9))
    png += png_chunk(b"IEND", b"")

    return base64.b64encode(png).decode("ascii")


def render_title(fallback, color, status, text):
    if STYLE == "pill":
        print(" | image=%s" % pill_png(text, PILL_DOTS[status]))
    else:
        print("%s | color=%s" % (fallback, color))


def warn(title, *details):
    render_title("⚠️ %s" % title, "red", "ERR", "!")
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
        render_title("🚨 emergency", "red", "ERR", "!!")
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
    render_title(title, COLORS[status], status, remaining + (" !" if stale else ""))

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
