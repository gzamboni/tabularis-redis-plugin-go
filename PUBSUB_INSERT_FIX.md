# PubSub INSERT Operations Fix

## Problems Fixed

### 1. Column Name Mapping Issue
The PubSub write operations (`INSERT INTO pubsub_subscriptions` and `INSERT INTO pubsub_messages`) were failing in Tabularis UI because the code was using **positional value mapping** instead of **column name mapping**.

### 2. SQL Comment Handling Issue  
Tabularis UI sends SQL queries with comments (e.g., `-- Publish a message`), which were not being stripped before parsing, causing "Unsupported query" errors.

## Root Causes

### Issue 1: Positional vs Named Columns
When Tabularis UI sends:
```sql
INSERT INTO pubsub_subscriptions (channel) VALUES ('test_channel')
```

The parser extracts:
- `parser.Columns = ["channel"]`
- `parser.Values = [["test_channel"]]`

But the original code expected values in fixed positions (values[0], values[1], etc.), causing failures when:
- Only some columns were specified
- Columns were in a different order
- UI-generated INSERT statements used explicit column names

### Issue 2: SQL Comments Not Stripped
Queries like:
```sql
-- Publish a message to a channel
INSERT INTO pubsub_messages (channel, payload) VALUES ('ch', 'msg')
```

Were being parsed with the comment included, causing parse failures.

## Solutions Implemented

### Fix 1: Column Name Mapping ([`internal/plugin/executor.go`](internal/plugin/executor.go:558-637))

**For `pubsub_subscriptions`:**
```go
// Map column names to indices
channelIdx, isPatternIdx, bufferSizeIdx, ttlIdx := -1, -1, -1, -1
if len(parser.Columns) > 0 {
    for i, col := range parser.Columns {
        switch col {
        case "channel":
            channelIdx = i
        case "is_pattern":
            isPatternIdx = i
        case "buffer_size":
            bufferSizeIdx = i
        case "ttl":
            ttlIdx = i
        }
    }
} else {
    // No columns specified, use positional order
    channelIdx = 0
    // ... etc
}

// Use indices to access values
if channelIdx >= 0 && channelIdx < len(values) {
    channel := values[channelIdx]
    // ...
}
```

**For `pubsub_messages`:**
Similar mapping for `channel` and `payload` columns.

### Fix 2: SQL Comment Stripping ([`internal/plugin/plugin.go`](internal/plugin/plugin.go:28-45))

Added `cleanQuery` function:
```go
func cleanQuery(query string) string {
    lines := strings.Split(query, "\n")
    var cleanedLines []string
    
    for _, line := range lines {
        // Remove single-line comments (-- comment)
        if idx := strings.Index(line, "--"); idx != -1 {
            line = line[:idx]
        }
        line = strings.TrimSpace(line)
        if line != "" {
            cleanedLines = append(cleanedLines, line)
        }
    }
    
    result := strings.Join(cleanedLines, " ")
    return strings.TrimSpace(result)
}
```

Updated `execute_query` handler to use `cleanQuery`:
```go
query := cleanQuery(p.Query)  // Instead of strings.TrimSpace(p.Query)
```

## Testing

### Test Script: [`test_pubsub_insert_fix.sh`](test_pubsub_insert_fix.sh)

All tests pass:
1. ✅ INSERT with single column: `INSERT INTO pubsub_subscriptions (channel) VALUES ('test')`
2. ✅ INSERT with multiple columns: `INSERT INTO pubsub_subscriptions (channel, is_pattern) VALUES ('pattern:*', 'true')`
3. ✅ INSERT into pubsub_messages: `INSERT INTO pubsub_messages (channel, payload) VALUES ('ch', 'msg')`
4. ✅ INSERT with reversed column order: `INSERT INTO pubsub_messages (payload, channel) VALUES ('msg', 'ch')`
5. ✅ Subscription persistence within same process

### Manual Test with Comments:
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"execute_query","params":{"params":{"driver":"redis","host":"localhost","port":6379,"database":"0"},"query":"-- Publish a message to a channel\nINSERT INTO pubsub_messages (channel, payload)\nVALUES ('"'"'notifications'"'"', '"'"'Hello World!'"'"')"}}' | ./tabularis-redis-plugin-go
```
Result: ✅ `{"jsonrpc":"2.0","id":1,"result":{"affected_rows":1,"columns":["status"],"rows":[["OK"]]}}`

## Impact

- ✅ **Fixed**: PubSub write operations now work correctly in Tabularis UI
- ✅ **Fixed**: SQL comments are properly stripped before parsing
- ✅ **Backward Compatible**: Still supports positional values when no column names are specified
- ✅ **Flexible**: Supports any column order in INSERT statements
- ✅ **No Breaking Changes**: All existing tests continue to pass

## Files Modified

1. [`internal/plugin/executor.go`](internal/plugin/executor.go) - Updated `executeInsert` function for `pubsub_subscriptions` and `pubsub_messages` cases with column mapping
2. [`internal/plugin/plugin.go`](internal/plugin/plugin.go) - Added `cleanQuery` function and updated `execute_query` handler to strip SQL comments

## Files Added

- [`test_pubsub_insert_fix.sh`](test_pubsub_insert_fix.sh) - Comprehensive test script for the fixes
- [`PUBSUB_INSERT_FIX.md`](PUBSUB_INSERT_FIX.md) - This documentation file

## Usage in Tabularis

After restarting Tabularis with the updated plugin, you can now use SQL comments and flexible column ordering:

```sql
-- Create a subscription
INSERT INTO pubsub_subscriptions (channel) VALUES ('notifications');

-- Create a pattern subscription with all options
INSERT INTO pubsub_subscriptions (channel, is_pattern, buffer_size, ttl) 
VALUES ('user:*', 'true', 2000, 7200);

-- Publish a message
INSERT INTO pubsub_messages (channel, payload) 
VALUES ('notifications', 'Hello, World!');

-- Publish with reversed column order (also works!)
INSERT INTO pubsub_messages (payload, channel) 
VALUES ('Test message', 'test_channel');
```

## Installation

1. Build: `go build -o tabularis-redis-plugin-go ./cmd/tabularis-redis-plugin-go`
2. Test: `./test_pubsub_insert_fix.sh`
3. Install: `cp tabularis-redis-plugin-go ~/Library/Application\ Support/com.debba.tabularis/plugins/redis/`
4. **Restart Tabularis**
5. Try INSERT operations in the SQL editor - they should now work correctly!
