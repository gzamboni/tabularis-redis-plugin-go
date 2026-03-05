# Redis Tabularis Plugin

A Redis driver plugin for [Tabularis](https://github.com/debba/tabularis), the lightweight database management tool.

This plugin enables Tabularis to connect to Redis databases and explore data in a tabular format, providing key scanning and virtual table representations for complex data types through a JSON-RPC 2.0 over stdio interface.

## Table of Contents

- [Features](#features)
- [Supported Redis Data Types](#supported-redis-data-types)
- [Installation](#installation)
  - [Automatic (via Tabularis)](#automatic-via-tabularis)
  - [Manual Installation](#manual-installation)
- [How It Works](#how-it-works)
- [Virtual Tables](#virtual-tables)
- [Supported Operations](#supported-operations)
- [Building from Source](#building-from-source)
- [Development](#development)
  - [Testing](#testing)
  - [Tech Stack](#tech-stack)
- [CI/CD](#cicd)
- [Contributing](#contributing)
- [License](#license)

## Features

- **Key Scanning** — View all keys with their types and Time-To-Live (TTL)
- **Virtual Tables** — Explore Hashes, Lists, Sets, and Sorted Sets as virtual relational tables
- **JSON-RPC 2.0** — Implements the standard Tabularis plugin protocol over stdio
- **Cross-platform** — Supports Linux (amd64, arm64), macOS (amd64, arm64), and Windows (amd64, arm64)
- **Read-only Operations** — Safe exploration of Redis data without modification risk

## Supported Redis Data Types

The plugin maps Redis data structures to virtual tables for SQL-like querying:

| Redis Type | Description | Virtual Table |
| :--- | :--- | :--- |
| **String** | Basic key-value pairs | `keys` |
| **Hash** | Field-value maps | `hashes` |
| **List** | Ordered collections of strings | `lists` |
| **Set** | Unordered collections of unique strings | `sets` |
| **Sorted Set (ZSet)** | Collections of unique strings ordered by score | `zsets` |

## Installation

### Automatic (via Tabularis)

If your version of Tabularis supports plugin management, the Redis plugin can be installed directly from the application's plugin manager.

### Manual Installation

1. **Download** the latest release for your platform from the [Releases page](https://github.com/gzamboni/tabularis-redis-plugin-go/releases)
2. **Extract** the archive (contains the executable, `manifest.json`, `README.md`, and `LICENSE`)
3. **Copy** the contents to the Tabularis plugins directory:

| OS | Plugins Directory |
| :--- | :--- |
| **Linux** | `~/.config/tabularis/plugins/redis/` |
| **macOS** | `~/Library/Application Support/com.debba.tabularis/plugins/redis/` |
| **Windows** | `%APPDATA%\tabularis\plugins\redis\` |

4. **Restart** Tabularis

The plugin will appear as a connection option in Tabularis.

## How It Works

The plugin is a standalone Go binary that communicates with Tabularis through JSON-RPC 2.0 over stdio:

1. **Spawn** — Tabularis spawns the plugin as a child process
2. **Request** — Tabularis sends newline-delimited JSON-RPC messages to the plugin's stdin
3. **Process** — The plugin processes requests and queries Redis
4. **Response** — Results are written to stdout in JSON-RPC format

This architecture ensures complete isolation between Tabularis and the plugin, with no shared memory or state.

## Virtual Tables

Since Redis is a key-value store, this plugin exposes virtual tables to enable SQL-like querying:

### `keys` Table
Lists all keys in the selected database.

| Column | Type | Description |
| :--- | :--- | :--- |
| `key` | STRING | The Redis key name |
| `type` | STRING | Data type (string, hash, list, set, zset) |
| `ttl` | INTEGER | Time-to-live in seconds (-1 = no expiry, -2 = expired) |
| `value` | STRING | Value for string types, empty for others |

**Example Queries:**
```sql
-- Get all hash keys
SELECT * FROM keys WHERE type = 'hash'

-- Find keys matching a pattern
SELECT * FROM keys WHERE key LIKE 'user:%'

-- Filter by TTL
SELECT * FROM keys WHERE ttl > 3600
```

### `hashes` Table
Explores hash fields and values.

| Column | Type | Description |
| :--- | :--- | :--- |
| `key` | STRING | The hash key name |
| `field` | STRING | Field name within the hash |
| `value` | STRING | Field value |

**Example Queries:**
```sql
-- Get all fields for a specific hash
SELECT * FROM hashes WHERE key = 'user:1001'

-- Filter by field name
SELECT * FROM hashes WHERE key = 'user:1001' AND field = 'email'

-- Find hashes with specific field values
SELECT * FROM hashes WHERE field = 'status' AND value = 'active'
```

### `lists` Table
Explores list elements with their indices.

| Column | Type | Description |
| :--- | :--- | :--- |
| `key` | STRING | The list key name |
| `index` | INTEGER | Element position (0-based) |
| `value` | STRING | Element value |

**Example Queries:**
```sql
-- Get all elements from a list
SELECT * FROM lists WHERE key = 'queue:tasks'

-- Filter by value
SELECT * FROM lists WHERE key = 'mylist' AND value = 'apple'

-- Get first 10 elements
SELECT * FROM lists WHERE key = 'queue:tasks' LIMIT 10
```

### `sets` Table
Explores set members.

| Column | Type | Description |
| :--- | :--- | :--- |
| `key` | STRING | The set key name |
| `value` | STRING | Member value |

**Example Queries:**
```sql
-- Get all members from a set
SELECT * FROM sets WHERE key = 'tags'

-- Check if a value exists
SELECT * FROM sets WHERE key = 'tags' AND value = 'redis'

-- Find sets with specific members
SELECT * FROM sets WHERE value LIKE 'user:%'
```

### `zsets` Table
Explores sorted set members with scores.

| Column | Type | Description |
| :--- | :--- | :--- |
| `key` | STRING | The sorted set key name |
| `value` | STRING | Member value |
| `score` | NUMERIC | Member score |

**Example Queries:**
```sql
-- Get all members from a sorted set
SELECT * FROM zsets WHERE key = 'leaderboard'

-- Filter by score range
SELECT * FROM zsets WHERE key = 'leaderboard' AND score > 100

-- Get top 10 scores
SELECT * FROM zsets WHERE key = 'leaderboard' LIMIT 10

-- Complex filtering
SELECT * FROM zsets WHERE key = 'leaderboard' AND score >= 50 AND score <= 200
```

## Query Features

The plugin supports advanced SQL-like query features:

### WHERE Conditions

**Supported Operators:**
- `=` — Equality
- `!=` or `<>` — Inequality
- `>` — Greater than
- `<` — Less than
- `>=` — Greater than or equal
- `<=` — Less than or equal
- `LIKE` — Pattern matching (use `%` as wildcard, `_` for single character)
- `IN` — Match against multiple values

**Examples:**
```sql
-- Equality
SELECT * FROM keys WHERE key = 'user:1'

-- Comparison
SELECT * FROM zsets WHERE score > 100

-- Pattern matching
SELECT * FROM keys WHERE key LIKE 'user:%'

-- IN operator (match multiple values)
SELECT * FROM keys WHERE type IN ('hash', 'set')
SELECT * FROM hashes WHERE field IN ('name', 'email', 'age')

-- Multiple conditions with AND
SELECT * FROM hashes WHERE key = 'user:1' AND field = 'email'
SELECT * FROM zsets WHERE key = 'scores' AND score > 50 AND score < 100

-- Combining IN with other conditions
SELECT * FROM keys WHERE type IN ('hash', 'string') AND key LIKE 'user:%'
```

### ORDER BY

Sort results by one or more columns:
```sql
-- Sort by single column ascending (default)
SELECT * FROM keys ORDER BY key

-- Sort descending
SELECT * FROM zsets WHERE key = 'leaderboard' ORDER BY score DESC

-- Sort by multiple columns
SELECT * FROM keys ORDER BY type ASC, key DESC

-- Combine with WHERE and LIMIT
SELECT * FROM zsets WHERE score > 50 ORDER BY score DESC LIMIT 10
```

**Supported:**
- Single or multiple columns
- `ASC` (ascending, default) or `DESC` (descending)
- Works with all data types (numeric and string)

### LIMIT and OFFSET

Control result set size:
```sql
-- Get first 10 results
SELECT * FROM keys LIMIT 10

-- Skip first 5, get next 10
SELECT * FROM keys LIMIT 10 OFFSET 5

-- Pagination example
SELECT * FROM hashes WHERE key LIKE 'user:%' LIMIT 20 OFFSET 40
```

### Pattern Matching

Use `LIKE` for pattern matching:
```sql
-- Keys starting with 'user:'
SELECT * FROM keys WHERE key LIKE 'user:%'

-- Keys ending with ':cache'
SELECT * FROM keys WHERE key LIKE '%:cache'

-- Keys containing 'session'
SELECT * FROM keys WHERE key LIKE '%session%'

-- Single character wildcard
SELECT * FROM keys WHERE key LIKE 'user:_'
```

### Column Filtering

Filter on any column in the virtual table:
```sql
-- Filter hashes by field name
SELECT * FROM hashes WHERE field = 'email'

-- Filter lists by value
SELECT * FROM lists WHERE value = 'pending'

-- Filter sorted sets by score
SELECT * FROM zsets WHERE score > 1000
```

## Write Operations

The plugin supports INSERT, UPDATE, and DELETE operations that translate SQL statements to appropriate Redis commands:

### INSERT Operations

Add new data to Redis using SQL INSERT statements:

```sql
-- Insert a string key (with optional TTL in seconds)
INSERT INTO keys (key, value) VALUES ('user:1', 'John Doe')
-- Redis: SET user:1 "John Doe"
INSERT INTO keys (key, value, ttl) VALUES ('session:1', 'active', 3600)
-- Redis: SET session:1 "active" EX 3600

-- Insert hash fields
INSERT INTO hashes (key, field, value) VALUES ('user:1', 'name', 'John'), ('user:1', 'email', 'john@example.com')
-- Redis: HSET user:1 name "John", HSET user:1 email "john@example.com"

-- Insert list items (appends to end)
INSERT INTO lists (key, value) VALUES ('queue', 'task1'), ('queue', 'task2')
-- Redis: RPUSH queue "task1", RPUSH queue "task2"

-- Insert set members
INSERT INTO sets (key, member) VALUES ('tags', 'redis'), ('tags', 'database')
-- Redis: SADD tags "redis", SADD tags "database"

-- Insert sorted set members with scores
INSERT INTO zsets (key, member, score) VALUES ('leaderboard', 'player1', 100), ('leaderboard', 'player2', 200)
-- Redis: ZADD leaderboard 100 "player1", ZADD leaderboard 200 "player2"
```

### UPDATE Operations

Modify existing data:

```sql
-- Update a string key value
UPDATE keys SET value = 'Jane Doe' WHERE key = 'user:1'
-- Redis: SET user:1 "Jane Doe"

-- Update a hash field
UPDATE hashes SET value = 'jane@example.com' WHERE key = 'user:1' AND field = 'email'
-- Redis: HSET user:1 email "jane@example.com"

-- Update a sorted set member score
UPDATE zsets SET score = 150 WHERE key = 'leaderboard' AND member = 'player1'
-- Redis: ZADD leaderboard 150 "player1"
```

**Note:** UPDATE for lists and sets is not supported as these data structures don't have a natural update semantic. Use DELETE + INSERT instead.

### DELETE Operations

Remove data from Redis:

```sql
-- Delete a key entirely
DELETE FROM keys WHERE key = 'user:1'
-- Redis: DEL user:1

-- Delete a hash field
DELETE FROM hashes WHERE key = 'user:1' AND field = 'email'
-- Redis: HDEL user:1 email

-- Delete list items (removes all occurrences)
DELETE FROM lists WHERE key = 'queue' AND value = 'task1'
-- Redis: LREM queue 0 "task1"

-- Delete a set member
DELETE FROM sets WHERE key = 'tags' AND member = 'redis'
-- Redis: SREM tags "redis"

-- Delete a sorted set member
DELETE FROM zsets WHERE key = 'leaderboard' AND member = 'player1'
-- Redis: ZREM leaderboard "player1"
```

## Supported Operations

The plugin implements the following JSON-RPC methods:

| Method | Description | Parameters |
| :--- | :--- | :--- |
| `test_connection` | Verify database connectivity | Connection params |
| `get_databases` | List logical databases (0-15) | Connection params |
| `get_tables` | List virtual tables | Connection params + database |
| `get_columns` | Get column metadata for a table | Connection params + database + table |
| `execute_query` | Execute SELECT, INSERT, UPDATE, DELETE queries | Connection params + database + SQL query |

> **Note:** Methods like `get_schemas`, `get_views`, `get_routines`, etc., return empty responses as they don't apply to Redis's data model.

## Building from Source

### Prerequisites

- Go 1.19 or higher
- Git

### Build

```bash
# Clone the repository
git clone https://github.com/gzamboni/tabularis-redis-plugin-go.git
cd tabularis-redis-plugin-go

# Build the plugin
go build -o tabularis-redis-plugin-go ./cmd/tabularis-redis-plugin-go
```

The executable will be generated in the current directory.

### Cross-Platform Build

To build for multiple platforms:

```bash
# Linux AMD64
GOOS=linux GOARCH=amd64 go build -o tabularis-redis-plugin-go-linux-amd64 ./cmd/tabularis-redis-plugin-go

# macOS ARM64 (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o tabularis-redis-plugin-go-darwin-arm64 ./cmd/tabularis-redis-plugin-go

# Windows AMD64
GOOS=windows GOARCH=amd64 go build -o tabularis-redis-plugin-go-windows-amd64.exe ./cmd/tabularis-redis-plugin-go
```

## Development

The plugin communicates with Tabularis via JSON-RPC 2.0 over `stdin` and `stdout`. For detailed AI agent development guidance, see [`AGENTS.md`](AGENTS.md).

### Testing

#### Manual Testing

Test the plugin by piping JSON-RPC requests directly:

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"test_connection","params":{"params":{"driver":"redis","host":"localhost","port":6379,"database":"0"}}}' | ./tabularis-redis-plugin-go
```

Expected response:
```json
{"jsonrpc":"2.0","id":1,"result":{"success":true}}
```

#### Unit Tests

Run unit tests using the in-memory `miniredis` instance:

```bash
go test -v ./...
```

#### End-to-End Tests

Run E2E tests against a real Redis instance (requires Docker):

```bash
chmod +x run_e2e.sh
./run_e2e.sh
```

This script:
1. Starts a Redis container
2. Seeds test data
3. Executes queries through the plugin
4. Validates responses
5. Cleans up the container

### Tech Stack

- **Language:** Go 1.19+
- **Redis Client:** [`github.com/go-redis/redis/v8`](https://github.com/go-redis/redis)
- **Testing:** [`github.com/alicebob/miniredis/v2`](https://github.com/alicebob/miniredis) (in-memory Redis)
- **Protocol:** JSON-RPC 2.0 over stdio
- **Build Tool:** [GoReleaser](https://goreleaser.com/)

## CI/CD

The project uses GitHub Actions for continuous integration and deployment:

### CI Workflow
**Trigger:** Every push and pull request to `main`

**Steps:**
1. Build the plugin
2. Run unit tests
3. Run E2E tests with Docker

### Release Workflow
**Trigger:** Tag pushes matching `v*` (e.g., `v0.1.0`)

**Steps:**
1. Cross-compile for Linux, macOS, and Windows (amd64 and arm64)
2. Package each binary with `manifest.json`, `README.md`, and `LICENSE`
3. Create ZIP archives
4. Publish GitHub Release with all artifacts

**Creating a Release:**
```bash
git tag v0.2.0
git push origin v0.2.0
```

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

Ensure all tests pass before submitting:
```bash
go test -v ./...
./run_e2e.sh
```

## License

This project is licensed under the MIT License. See the [`LICENSE`](LICENSE) file for details.

---

**Repository:** [github.com/gzamboni/tabularis-redis-plugin-go](https://github.com/gzamboni/tabularis-redis-plugin-go)
**Tabularis:** [github.com/debba/tabularis](https://github.com/debba/tabularis)
