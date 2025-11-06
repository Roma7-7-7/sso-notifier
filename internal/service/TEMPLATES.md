# Message Templates - Design Documentation

> **IMPORTANT FOR AI ASSISTANTS (Claude):**
> This file documents the message rendering system. If you modify:
> - `messageTemplate` in `messages.go`
> - Emojis or labels in `buildGroupSchedule()`
> - Any rendering logic
>
> You **must** update:
> - This file (TEMPLATES.md) - examples and documentation
> - `/CLAUDE.md` - "Message Templates" section with new format examples

## Overview

The message rendering system has been designed with a generic, hierarchical structure that can support:
- Multiple dates (today, tomorrow, etc.)
- Multiple groups per user
- Human-readable, well-formatted output

## Data Structure

```go
NotificationMessage
  └── Dates []DateSchedule
        └── Date string (e.g., "20 жовтня")
        └── Groups []GroupSchedule
              └── GroupNum string (e.g., "5")
              └── StatusLines []StatusLine
                    └── Emoji string (🟢/🟡/🔴)
                    └── Label string (Заживлено/Можливо заживлено/Відключено)
                    └── Periods []Period
                          └── From string (e.g., "14:00")
                          └── To string (e.g., "18:00")
```

## Example Output

### Single Date, Single Group

```
Графік стабілізаційних відключень:

📅 20 жовтня:
Група 5:
  🟢 Заживлено:  14:00 - 18:00; 20:00 - 24:00;
  🔴 Відключено:  18:00 - 20:00;
```

### Single Date, Multiple Groups

```
Графік стабілізаційних відключень:

📅 20 жовтня:
Група 3:
  🟢 Заживлено:  12:00 - 24:00;
Група 5:
  🟢 Заживлено:  14:00 - 18:00; 20:00 - 24:00;
  🔴 Відключено:  18:00 - 20:00;
```

### Multiple Dates, Multiple Groups

```
Графік стабілізаційних відключень:

📅 20 жовтня:
Група 3:
  🟢 Заживлено:  12:00 - 24:00;
Група 5:
  🟢 Заживлено:  14:00 - 18:00; 20:00 - 24:00;
  🔴 Відключено:  18:00 - 20:00;

📅 21 жовтня:
Група 3:
  🟢 Заживлено:  00:00 - 08:00; 16:00 - 24:00;
  🔴 Відключено:  08:00 - 16:00;
Група 5:
  🟢 Заживлено:  00:00 - 24:00;
```


## Upcoming Notification Template

### Overview

The upcoming notification template is used for 10-minute advance alerts before power status changes. It follows the same template-based approach as the main notification system but with simpler structure since it only deals with future events.

### Location

`internal/service/upcoming_messages.go`

### Data Structure

```go
UpcomingMessage
  └── IsRestoration bool          // true if any alert is for ON status
  └── Alerts []UpcomingAlert
        └── Status dal.Status     // OFF, MAYBE, or ON
        └── StartTime string      // e.g., "08:30"
        └── Groups []string       // Group numbers (e.g., ["5", "7"])
        └── Emoji string          // Status emoji (🟢/🟡/🔴)
        └── Label string          // Ukrainian status label
```

### Template Features

- **Conditional Header**: Shows "⚡ Гарні новини!" for power restoration, "⚠️ Увага!" for outages
- **Group Formatting**: Automatically handles singular ("Група 5") vs plural ("Групи 5, 7")
- **Custom Function**: `joinGroups` - joins group numbers with comma+space
- **Emoji Support**: Status-specific emojis for quick visual recognition
- **Sorted Output**: Groups are numerically sorted, alerts sorted by time then status priority

### Example Outputs

#### Single Group, Single Status

```
⚠️ Увага! Незабаром зміниться електропостачання:

Група 5:
🔴 Відключення електропостачання об 08:30
```

#### Multiple Groups, Same Time and Status

```
⚠️ Увага! Незабаром зміниться електропостачання:

Групи 5, 7:
🔴 Відключення електропостачання об 08:30
```

#### Multiple Groups, Different Times

```
⚠️ Увага! Незабаром зміниться електропостачання:

Група 5:
🔴 Відключення електропостачання об 08:30

Група 7:
🟡 Можливе відключення електропостачання об 09:00
```

#### Multiple Groups, Mixed Statuses Same Time

```
⚠️ Увага! Незабаром зміниться електропостачання:

Групи 5, 7:
🔴 Відключення електропостачання об 08:30

Група 9:
🟡 Можливе відключення електропостачання об 08:30
```

#### Power Restoration

```
⚡ Гарні новини! Незабаром зміниться електропостачання:

Групи 3, 5:
🟢 Відновлення електропостачання об 14:00
```

### Status Labels and Emojis

| Status | Emoji | Label |
|--------|-------|-------|
| ON | 🟢 | Відновлення електропостачання |
| OFF | 🔴 | Відключення електропостачання |
| MAYBE | 🟡 | Можливе відключення електропостачання |

### Maintenance Notes

**If you modify the template or rendering logic**, update:
1. This file (TEMPLATES.md) - examples and documentation
2. CLAUDE.md - "Alerts Service" section
3. ALERTS_DESIGN.md (if still present) - message format examples
