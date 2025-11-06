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

## Template

The generic template supports arbitrary nesting:

```
Графік стабілізаційних відключень:

📅 {Date 1}:
Група {Group 1}:
  {Emoji} {Label}: {Period1}; {Period2}; ...
  {Emoji} {Label}: {Period1}; ...
Група {Group 2}:
  {Emoji} {Label}: {Period1}; ...

📅 {Date 2}:
Група {Group 1}:
  ...
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

## Current Implementation

The system is **fully template-based** as of now:

1. **`buildGroupSchedule()`** - Converts periods/statuses to `GroupSchedule`
2. **`renderGroup()`** - Returns `GroupSchedule` (not a string!)
3. **`renderMessage()`** - Uses `messageTemplate.Execute()` with `NotificationMessage`
4. **`MessageBuilder.Build()`** - Collects `GroupSchedule` objects and renders via template

### Adding Multi-Date Support

To support today + tomorrow notifications:

```go
// In notifications.go
todayTable, _ := s.shutdowns.GetShutdowns(dal.TodayDate(s.loc))
tomorrowTable, _ := s.shutdowns.GetShutdowns(dal.TomorrowDate(s.loc))

now := time.Now().In(s.loc)
todayBuilder := NewMessageBuilder(todayTable.Date, todayTable, now)
tomorrowBuilder := NewMessageBuilder(tomorrowTable.Date, tomorrowTable, now.Add(24*time.Hour))

// Collect groups from both days
allGroups := make([]DateSchedule, 0)
// ... build DateSchedule for each date

// Render using template
msg := NotificationMessage{Dates: allGroups}
messageTemplate.Execute(&buf, msg)
```

## Benefits

### Human Readability
- Date emoji (📅) clearly marks date sections
- Consistent indentation shows hierarchy
- Color-coded status (🟢🟡🔴) for quick scanning
- Semicolons separate time periods naturally

### Flexibility
- Add new dates without template changes
- Add new groups without template changes
- Easy to add new status types by extending StatusLine
- Template is self-documenting

### Maintainability
- Single source of truth for message format
- Type-safe data structure
- Easy to test each level independently
- Clear separation between data and presentation

## Implementation Notes

### Performance

- Template parsing happens once at startup (template.Must)
- Runtime execution is efficient
- String concatenation is comparable for small messages
- Template shines with complex nested structures

## Status Line Design

The StatusLine abstraction cleanly separates:
- **Visual**: Emoji for quick recognition
- **Text**: Human-readable label
- **Data**: Time periods

This makes it easy to:
- Change emojis without touching logic
- Translate labels to other languages
- Add new status types
- Sort/filter status lines

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

### Template

```
{{if .IsRestoration}}⚡ Гарні новини! Через 10 хвилин:{{else}}⚠️ Увага! Через 10 хвилин:{{end}}

{{range .Alerts}}
{{if eq (len .Groups) 1}}Група {{index .Groups 0}}:{{else}}Групи {{joinGroups .Groups}}:{{end}}
{{.Emoji}} {{.Label}} об {{.StartTime}}
{{end}}
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
⚠️ Увага! Через 10 хвилин:

Група 5:
🔴 Відключення електроенергії об 08:30
```

#### Multiple Groups, Same Time and Status

```
⚠️ Увага! Через 10 хвилин:

Групи 5, 7:
🔴 Відключення електроенергії об 08:30
```

#### Multiple Groups, Different Times

```
⚠️ Увага! Через 10 хвилин:

Група 5:
🔴 Відключення електроенергії об 08:30

Група 7:
🟡 Можливе відключення електроенергії об 09:00
```

#### Multiple Groups, Mixed Statuses Same Time

```
⚠️ Увага! Через 10 хвилин:

Групи 5, 7:
🔴 Відключення електроенергії об 08:30

Група 9:
🟡 Можливе відключення електроенергії об 08:30
```

#### Power Restoration

```
⚡ Гарні новини! Через 10 хвилин:

Групи 3, 5:
🟢 Відновлення електроенергії об 14:00
```

### Status Labels and Emojis

| Status | Emoji | Label |
|--------|-------|-------|
| ON | 🟢 | Відновлення електроенергії |
| OFF | 🔴 | Відключення електроенергії |
| MAYBE | 🟡 | Можливе відключення електроенергії |

### Implementation Notes

#### Grouping Logic

Alerts are grouped by `(Status, StartTime)` to merge multiple groups with the same event:

```go
type groupKey struct {
    Status    dal.Status
    StartTime string
}
```

This produces cleaner messages:
- Instead of: 3 separate messages for groups 5, 7, 9 at 08:30
- Shows: "Групи 5, 7, 9: 🔴 Відключення електроенергії об 08:30"

#### Sorting

1. **Groups within alerts**: Sorted numerically (1, 2, 11 not 1, 11, 2)
2. **Alerts**: Sorted by start time, then by status priority (OFF > MAYBE > ON)

This ensures consistent output and prioritizes more critical alerts.

#### Error Handling

If template execution fails, a fallback message is shown:
```
⚠️ Увага! Через 10 хвилин змінюється статус електроенергії
```

This prevents notification failures due to template errors.

### Comparison with Main Notification Template

| Feature | Main Notification | Upcoming Notification |
|---------|------------------|----------------------|
| **Scope** | Past + future periods | Only future (10 min ahead) |
| **Structure** | Multi-date, multi-group, multi-period | Single-event alerts |
| **Header** | "Графік стабілізаційних відключень:" | "⚠️ Увага!" or "⚡ Гарні новини!" |
| **Grouping** | By date → group → status | By status+time → groups |
| **Time format** | Period ranges (08:00 - 11:00) | Single time (об 08:00) |
| **Use case** | Schedule change notifications | Advance warnings |

### Maintenance Notes

**If you modify the template or rendering logic**, update:
1. This file (TEMPLATES.md) - examples and documentation
2. CLAUDE.md - "Alerts Service" section
3. ALERTS_DESIGN.md (if still present) - message format examples

## Future Enhancements

### Possible Extensions

1. **Empty state messages**:
   ```go
   {{if not .Periods}}  ℹ️ Немає відключень{{end}}
   ```

2. **Summary line**:
   ```go
   📊 Всього відключень: {{len .Dates}} днів, {{totalGroups .}} груп
   ```

3. **Time duration**:
   ```go
   🔴 Відключено: 18:00 - 20:00 (2 години);
   ```

4. **Conditional formatting**:
   ```go
   {{if gt (len .Off) 0}}⚠️ Увага: тривалі відключення{{end}}
   ```

5. **Localization support**:
   ```go
   type Locale struct {
       Header string
       Labels map[Status]string
   }
   ```

## Comparison: Before vs After

### Before (Pre-refactor)
```go
// Hardcoded template
var messageTemplate = `Графік стабілізаційних відключень на {{.Date}}:`
var groupMessageTemplate = `Група {{.GroupNum}}:`

func renderMessage(date string, msgs []string) (string, error)
func renderGroup(num string, periods, statuses) (string, error)
```
- ❌ Single date only
- ❌ Groups pre-rendered as strings
- ❌ Hard to extend
- ❌ String concatenation everywhere

### Current (Template-based)
```go
// Generic template
var messageTemplate = `Графік стабілізаційних відключень:
{{range .Dates}}
📅 {{.Date}}:
{{range .Groups}}...{{end}}
{{end}}`

func renderMessage(date string, groups []GroupSchedule) (string, error)
func renderGroup(num string, periods, statuses) (GroupSchedule, error)
```
- ✅ Multi-date ready (template supports it)
- ✅ Multi-group ready (already works)
- ✅ Fully template-based
- ✅ Type-safe data structures
- ✅ Easy to customize and test
