# Test Server

A simple HTTP server for testing the schedule scraper without hitting the real API.

## Building

```bash
make testserver
```

## Usage

```bash
./bin/testserver [options] <current-schedule.html> [next-schedule.html]
```

### Options

- `-port` - Port to listen on (default: 8080)

### Arguments

- `current-schedule.html` - Path to HTML file for current day schedule (required)
- `next-schedule.html` - Path to HTML file for next day schedule (optional)

## Examples

### Serve both current and next schedule

```bash
./bin/testserver data/http/main.html data/http/next.html
```

This will start the server on port 8080 with:
- Current schedule at: `http://localhost:8080/shutdowns/`
- Next schedule at: `http://localhost:8080/shutdowns/?next`

### Serve only current schedule

```bash
./bin/testserver data/http/main.html
```

This will start the server with only:
- Current schedule at: `http://localhost:8080/shutdowns/`

### Custom port

```bash
./bin/testserver -port 9000 data/http/main.html data/http/next.html
```

## Using with the bot

Set the `SCHEDULE_URL` environment variable to point to your test server:

```bash
DEV=true SCHEDULE_URL="http://localhost:8080/shutdowns/" TELEGRAM_TOKEN=your_token ./bin/sso-notifier
```

## Development workflow

1. Start the test server:
   ```bash
   ./bin/testserver data/http/main.html data/http/next.html
   ```

2. The server reads files on each request, so you can edit the HTML files while it's running

3. Start the bot pointing to the test server:
   ```bash
   DEV=true SCHEDULE_URL="http://localhost:8080/shutdowns/" TELEGRAM_TOKEN=your_token ./bin/sso-notifier
   ```

4. Make changes to `data/http/main.html` or `data/http/next.html` to test different scenarios

5. The bot will pick up your changes on the next refresh cycle
