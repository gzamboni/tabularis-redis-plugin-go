package plugin

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

func matchesConditions(row map[ // matchesConditions checks if a row matches all conditions
string]interface{}, conditions []QueryCondition) bool {
	for _, cond := range conditions {
		rowValue, exists := row[cond.Column]
		if !exists {
			return false
		}
		rowValueStr := fmt.Sprintf("%v", rowValue)
		switch cond.Operator {
		case "=":
			if rowValueStr != cond.Value {
				return false
			}
		case "!=", "<>":
			if rowValueStr == cond.Value {
				return false
			}
		case "IN":
			inValues := strings.Split(cond.Value, ",")
			found := false
			for _, inVal := range inValues {
				inVal = strings.TrimSpace(inVal)
				inVal = strings.Trim(inVal, "'\"")
				if rowValueStr == inVal {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		case "LIKE":
			pattern := strings.ReplaceAll(cond.Value, "%", ".*")
			pattern = strings.ReplaceAll(pattern, "_", ".")
			pattern = "^" + pattern + "$"
			matched, _ := regexp.MatchString(pattern, rowValueStr)
			if !matched {
				return false
			}
		case ">", "<", ">=", "<=":
			rowNum, rowErr := strconv.ParseFloat(rowValueStr, 64)
			condNum, condErr := strconv.ParseFloat(cond.Value, 64)
			if rowErr == nil && condErr == nil {
				switch cond.Operator {
				case ">":
					if !(rowNum > condNum) {
						return false
					}
				case "<":
					if !(rowNum < condNum) {
						return false
					}
				case ">=":
					if !(rowNum >= condNum) {
						return false
					}
				case "<=":
					if !(rowNum <= condNum) {
						return false
					}
				}
			} else {
				switch cond.Operator {
				case ">":
					if !(rowValueStr > cond.Value) {
						return false
					}
				case "<":
					if !(rowValueStr < cond.Value) {
						return false
					}
				case ">=":
					if !(rowValueStr >= cond.Value) {
						return false
					}
				case "<=":
					if !(rowValueStr <= cond.Value) {
						return false
					}
				}
			}
		}
	}
	return true
}

func applyLimitOffset(rows [][]interface // applyLimitOffset applies LIMIT and OFFSET to rows
{

}, limit, offset int) [][]interface{} {
	if offset >= len(rows) {
		return [][]interface{}{}
	}
	start := offset
	end := len(rows)
	if limit > 0 && start+limit < end {
		end = start + limit
	}
	return rows[start:end]
}

func applyOrderBy(rows [][]interface // applyOrderBy sorts rows based on ORDER BY clauses
{

}, columns []string, orderBy []OrderBy) [][]interface{} {
	if len(orderBy) == 0 || len(rows) == 0 {
		return rows
	}
	colIndex := make(map[string]int)
	for i, col := range columns {
		colIndex[col] = i
	}
	sortedRows := make([][]interface{}, len(rows))
	copy(sortedRows, rows)
	sort.Slice(sortedRows, func(i, j int) bool {
		for _, ob := range orderBy {
			idx, exists := colIndex[ob.Column]
			if !exists {
				continue
			}
			valI := sortedRows[i][idx]
			valJ := sortedRows[j][idx]
			cmp := compareValues(valI, valJ)
			if cmp != 0 {
				if ob.Direction == "DESC" {
					return cmp > 0
				}
				return cmp < 0
			}
		}
		return false
	})
	return sortedRows
}

func compareValues(a, b interface{}) int {
	aFloat, aIsNum := toFloat64(a)
	bFloat, bIsNum := toFloat64(b)
	if aIsNum && bIsNum {
		if aFloat < bFloat {
			return -1
		} else if aFloat > bFloat {
			return 1
		}
		return 0
	}
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)
	if aStr < bStr {
		return -1
	} else if aStr > bStr {
		return 1
	}
	return 0
} // compareValues compares two values and returns -1, 0, or 1

func toFloat64(val interface{}) (float64, bool) {
	switch v := val.(type // toFloat64 attempts to convert a value to float64
	) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

func executeScanKeys(p ConnectionParams, parser QueryParser, page, pageSize int) map[string]interface{} {
	client, _ := getClient(p)
	keys, _ := client.Keys(ctx, "*").Result()
	allRows := [][]interface{}{}
	for _, k := range keys {
		typ, _ := client.Type(ctx, k).Result()
		ttl, _ := client.TTL(ctx, k).Result()
		var valStr string
		switch typ {
		case "string":
			valStr, _ = client.Get(ctx, k).Result()
		case "hash":
			val, _ := client.HGetAll(ctx, k).Result()
			b, _ := json.Marshal(val)
			valStr = string(b)
		case "list":
			val, _ := client.LRange(ctx, k, 0, -1).Result()
			b, _ := json.Marshal(val)
			valStr = string(b)
		case "set":
			val, _ := client.SMembers(ctx, k).Result()
			b, _ := json.Marshal(val)
			valStr = string(b)
		case "zset":
			val, _ := client.ZRangeWithScores(ctx, k, 0, -1).Result()
			var zVals []map[string]interface{}
			for _, z := range val {
				zVals = append(zVals, map[string]interface{}{"value": z.Member, "score": z.Score})
			}
			b, _ := json.Marshal(zVals)
			valStr = string(b)
		default:
			valStr = fmt.Sprintf("<%s>", typ)
		}
		rowMap := map[string]interface{}{"key": k, "type": typ, "ttl": int64(ttl.Seconds()), "value": valStr}
		if matchesConditions(rowMap, parser.Conditions) {
			allRows = append(allRows, []interface{}{k, typ, int64(ttl.Seconds()), valStr})
		}
	}
	if len(parser.OrderBy) > 0 {
		allRows = applyOrderBy(allRows, []string{"key", "type", "ttl", "value"}, parser.OrderBy)
	}
	if parser.Limit > 0 || parser.Offset > 0 {
		allRows = applyLimitOffset(allRows, parser.Limit, parser.Offset)
	}
	total := len(allRows)
	if pageSize == 0 {
		pageSize = 50
	}
	if page == 0 {
		page = 1
	}
	startIdx := (page - 1) * pageSize
	endIdx := startIdx + pageSize
	if startIdx > total {
		startIdx = total
	}
	if endIdx > total {
		endIdx = total
	}
	pagedRows := allRows[startIdx:endIdx]
	hasMore := endIdx < total
	return map[string]interface{}{"columns": []string{"key", "type", "ttl", "value"}, "rows": pagedRows, "affected_rows": 0, "truncated": hasMore, "pagination": map[string]interface{}{"page": page, "page_size": pageSize, "total_rows": total, "has_more": hasMore}}
}

func extractKey(query string) string {
	upperQuery := strings.ToUpper(query)
	idx := strings.Index(upperQuery, "WHERE KEY =")
	if idx != -1 {
		valPart := query[idx+len("WHERE KEY ="):]
		spaceIdx := strings.Index(strings.TrimSpace(valPart), " ")
		if spaceIdx != -1 {
			valPart = strings.TrimSpace(valPart)[:spaceIdx]
		}
		return strings.TrimSpace(strings.Trim(strings.TrimSpace(valPart), " '\""))
	}
	return ""
}

func paginateRows(rows [][]interface{}, page, pageSize int) ([][]interface{}, bool, int) {
	total := len(rows)
	if pageSize == 0 {
		pageSize = 50
	}
	if page == 0 {
		page = 1
	}
	startIdx := (page - 1) * pageSize
	endIdx := startIdx + pageSize
	if startIdx > total {
		startIdx = total
	}
	if endIdx > total {
		endIdx = total
	}
	hasMore := endIdx < total
	return rows[startIdx:endIdx], hasMore, total
}

func executeScanHashes(p ConnectionParams, parser QueryParser, page, pageSize int) map[string]interface{} {
	client, _ := getClient(p)
	allRows := [][]interface{}{}
	var specificKey string
	for _, cond := range // Check if there's a specific key condition
	parser.Conditions {
		if cond.Column == "key" && cond.Operator == "=" {
			specificKey = cond.Value
			break
		}
	}
	if specificKey != "" {
		fields, _ := client.HGetAll(ctx, specificKey).Result()
		for f, v := range fields {
			rowMap := map[string]interface{}{"key": specificKey, "field": f, "value": v}
			if matchesConditions(rowMap, parser.Conditions) {
				allRows = append(allRows, []interface{}{specificKey, f, v})
			}
		}
	} else {
		keys, _ := client.Keys(ctx, "*").Result()
		for _, k := range keys {
			typ, _ := client.Type(ctx, k).Result()
			if typ == "hash" {
				fields, _ := client.HGetAll(ctx, k).Result()
				for f, v := range fields {
					rowMap := map[string]interface{}{"key": k, "field": f, "value": v}
					if matchesConditions(rowMap, parser.Conditions) {
						allRows = append(allRows, []interface{}{k, f, v})
					}
				}
			}
		}
	}
	if len(parser.OrderBy) > 0 {
		allRows = applyOrderBy(allRows, []string{"key", "field", "value"}, parser.OrderBy)
	}
	if parser.Limit > 0 || parser.Offset > 0 {
		allRows = applyLimitOffset(allRows, parser.Limit, parser.Offset)
	}
	pagedRows, hasMore, total := paginateRows(allRows, page, pageSize)
	return map[string]interface{}{"columns": []string{"key", "field", "value"}, "rows": pagedRows, "affected_rows": 0, "truncated": hasMore, "pagination": map[string]interface{}{"page": page, "page_size": pageSize, "total_rows": total, "has_more": hasMore}}
}

func executeScanLists(p ConnectionParams, parser QueryParser, page, pageSize int) map[string]interface{} {
	client, _ := getClient(p)
	allRows := [][]interface{}{}
	var specificKey string
	for _, cond := range // Check if there's a specific key condition
	parser.Conditions {
		if cond.Column == "key" && cond.Operator == "=" {
			specificKey = cond.Value
			break
		}
	}
	if specificKey != "" {
		values, _ := client.LRange(ctx, specificKey, 0, -1).Result()
		for i, v := range values {
			rowMap := map[string]interface{}{"key": specificKey, "index": i, "value": v}
			if matchesConditions(rowMap, parser.Conditions) {
				allRows = append(allRows, []interface{}{specificKey, i, v})
			}
		}
	} else {
		keys, _ := client.Keys(ctx, "*").Result()
		for _, k := range keys {
			typ, _ := client.Type(ctx, k).Result()
			if typ == "list" {
				values, _ := client.LRange(ctx, k, 0, -1).Result()
				for i, v := range values {
					rowMap := map[string]interface{}{"key": k, "index": i, "value": v}
					if matchesConditions(rowMap, parser.Conditions) {
						allRows = append(allRows, []interface{}{k, i, v})
					}
				}
			}
		}
	}
	if len(parser.OrderBy) > 0 {
		allRows = applyOrderBy(allRows, []string{"key", "index", "value"}, parser.OrderBy)
	}
	if parser.Limit > 0 || parser.Offset > 0 {
		allRows = applyLimitOffset(allRows, parser.Limit, parser.Offset)
	}
	pagedRows, hasMore, total := paginateRows(allRows, page, pageSize)
	return map[string]interface{}{"columns": []string{"key", "index", "value"}, "rows": pagedRows, "affected_rows": 0, "truncated": hasMore, "pagination": map[string]interface{}{"page": page, "page_size": pageSize, "total_rows": total, "has_more": hasMore}}
}

func executeScanSets(p ConnectionParams, parser QueryParser, page, pageSize int) map[string]interface{} {
	client, _ := getClient(p)
	allRows := [][]interface{}{}
	var specificKey string
	for _, cond := range // Check if there's a specific key condition
	parser.Conditions {
		if cond.Column == "key" && cond.Operator == "=" {
			specificKey = cond.Value
			break
		}
	}
	if specificKey != "" {
		members, _ := client.SMembers(ctx, specificKey).Result()
		for _, m := range members {
			rowMap := map[string]interface{}{"key": specificKey, "value": m}
			if matchesConditions(rowMap, parser.Conditions) {
				allRows = append(allRows, []interface{}{specificKey, m})
			}
		}
	} else {
		keys, _ := client.Keys(ctx, "*").Result()
		for _, k := range keys {
			typ, _ := client.Type(ctx, k).Result()
			if typ == "set" {
				members, _ := client.SMembers(ctx, k).Result()
				for _, m := range members {
					rowMap := map[string]interface{}{"key": k, "value": m}
					if matchesConditions(rowMap, parser.Conditions) {
						allRows = append(allRows, []interface{}{k, m})
					}
				}
			}
		}
	}
	if len(parser.OrderBy) > 0 {
		allRows = applyOrderBy(allRows, []string{"key", "value"}, parser.OrderBy)
	}
	if parser.Limit > 0 || parser.Offset > 0 {
		allRows = applyLimitOffset(allRows, parser.Limit, parser.Offset)
	}
	pagedRows, hasMore, total := paginateRows(allRows, page, pageSize)
	return map[string]interface{}{"columns": []string{"key", "value"}, "rows": pagedRows, "affected_rows": 0, "truncated": hasMore, "pagination": map[string]interface{}{"page": page, "page_size": pageSize, "total_rows": total, "has_more": hasMore}}
}

func executeScanZSets(p ConnectionParams, parser QueryParser, page, pageSize int) map[string]interface{} {
	client, _ := getClient(p)
	allRows := [][]interface{}{}
	var specificKey string
	for _, cond := range // Check if there's a specific key condition
	parser.Conditions {
		if cond.Column == "key" && cond.Operator == "=" {
			specificKey = cond.Value
			break
		}
	}
	if specificKey != "" {
		members, _ := client.ZRangeWithScores(ctx, specificKey, 0, -1).Result()
		for _, m := range members {
			rowMap := map[string]interface{}{"key": specificKey, "value": fmt.Sprintf("%v", m.Member), "score": m.Score}
			if matchesConditions(rowMap, parser.Conditions) {
				allRows = append(allRows, []interface{}{specificKey, m.Member, m.Score})
			}
		}
	} else {
		keys, _ := client.Keys(ctx, "*").Result()
		for _, k := range keys {
			typ, _ := client.Type(ctx, k).Result()
			if typ == "zset" {
				members, _ := client.ZRangeWithScores(ctx, k, 0, -1).Result()
				for _, m := range members {
					rowMap := map[string]interface{}{"key": k, "value": fmt.Sprintf("%v", m.Member), "score": m.Score}
					if matchesConditions(rowMap, parser.Conditions) {
						allRows = append(allRows, []interface{}{k, m.Member, m.Score})
					}
				}
			}
		}
	}
	if len(parser.OrderBy) > 0 {
		allRows = applyOrderBy(allRows, []string{"key", "value", "score"}, parser.OrderBy)
	}
	if parser.Limit > 0 || parser.Offset > 0 {
		allRows = applyLimitOffset(allRows, parser.Limit, parser.Offset)
	}
	pagedRows, hasMore, total := paginateRows(allRows, page, pageSize)
	return map[string]interface{}{"columns": []string{"key", "value", "score"}, "rows": pagedRows, "affected_rows": 0, "truncated": hasMore, "pagination": map[string]interface{}{"page": page, "page_size": pageSize, "total_rows": total, "has_more": hasMore}}
}

func executeInsert(p ConnectionParams, parser InsertParser) map[ // executeInsert handles INSERT operations for all virtual tables
string]interface{} {
	client, err := getClient(p)
	if err != nil {
		return map[string]interface{}{"columns": []string{"error"}, "rows": [][]interface{}{{err.Error()}}, "affected_rows": 0}
	}
	affectedRows := 0
	switch parser.Table {
	case "keys":
		keyIdx, valIdx, ttlIdx := -1, -1, -1
		if len(parser.Columns) > 0 {
			for i, col := range parser.Columns {
				if col == "key" {
					keyIdx = i
				} else if col == "value" {
					valIdx = i
				} else if col == "ttl" {
					ttlIdx = i
				}
			}
		} else {
			keyIdx, valIdx = 0, 1
			if len(parser.Values) > 0 && len(parser.Values[0]) >= 3 {
				ttlIdx = 2
			}
		}

		for _, values := range parser.Values {
			if keyIdx >= 0 && valIdx >= 0 && keyIdx < len(values) && valIdx < len(values) {
				key := values[keyIdx]
				value := values[valIdx]
				var ttl time.Duration = 0
				if ttlIdx >= 0 && ttlIdx < len(values) {
					if t, err := strconv.Atoi(values[ttlIdx]); err == nil && t > 0 {
						ttl = time.Duration(t) * time.Second
					}
				}
				err := client.Set(ctx, key, value, ttl).Err()
				if err == nil {
					affectedRows++
				}
			}
		}
	case "hashes":
		for _, values := range parser.Values {
			if len(values) >= 3 {
				key := values[0]
				field := values[1]
				value := values[2]
				err := client.HSet(ctx, key, field, value).Err()
				if err == nil {
					affectedRows++
				}
			}
		}
	case "lists":
		for _, values := range parser.Values {
			if len(values) >= 2 {
				key := values[0]
				value := values[1]
				err := client.RPush(ctx, key, value).Err()
				if err == nil {
					affectedRows++
				}
			}
		}
	case "sets":
		for _, values := range parser.Values {
			if len(values) >= 2 {
				key := values[0]
				member := values[1]
				err := client.SAdd(ctx, key, member).Err()
				if err == nil {
					affectedRows++
				}
			}
		}
	case "zsets":
		for _, values := range parser.Values {
			if len(values) >= 3 {
				key := values[0]
				member := values[1]
				score, err := strconv.ParseFloat(values[2], 64)
				if err == nil {
					err := client.ZAdd(ctx, key, &redis.Z{Score: score, Member: member}).Err()
					if err == nil {
						affectedRows++
					}
				}
			}
		}
	case "pubsub_subscriptions":
		// INSERT INTO pubsub_subscriptions (channel, is_pattern, buffer_size, ttl) VALUES (...)
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
			if len(parser.Values) > 0 && len(parser.Values[0]) >= 2 {
				isPatternIdx = 1
			}
			if len(parser.Values) > 0 && len(parser.Values[0]) >= 3 {
				bufferSizeIdx = 2
			}
			if len(parser.Values) > 0 && len(parser.Values[0]) >= 4 {
				ttlIdx = 3
			}
		}

		for _, values := range parser.Values {
			if channelIdx >= 0 && channelIdx < len(values) {
				channel := values[channelIdx]
				isPattern := false
				bufferSize := 1000
				ttl := 3600

				if isPatternIdx >= 0 && isPatternIdx < len(values) {
					isPattern = values[isPatternIdx] == "true" || values[isPatternIdx] == "1"
				}
				if bufferSizeIdx >= 0 && bufferSizeIdx < len(values) {
					if bs, err := strconv.Atoi(values[bufferSizeIdx]); err == nil {
						bufferSize = bs
					}
				}
				if ttlIdx >= 0 && ttlIdx < len(values) {
					if t, err := strconv.Atoi(values[ttlIdx]); err == nil {
						ttl = t
					}
				}

				// Create subscription
				_, err := globalSubscriptionManager.Subscribe(client, channel, isPattern, bufferSize, ttl)
				if err == nil {
					affectedRows++
				}
			}
		}
	case "pubsub_messages":
		// INSERT INTO pubsub_messages (channel, payload) VALUES (...) - publishes a message
		// Map column names to indices
		channelIdx, payloadIdx := -1, -1
		if len(parser.Columns) > 0 {
			for i, col := range parser.Columns {
				switch col {
				case "channel":
					channelIdx = i
				case "payload":
					payloadIdx = i
				}
			}
		} else {
			// No columns specified, use positional order
			channelIdx = 0
			if len(parser.Values) > 0 && len(parser.Values[0]) >= 2 {
				payloadIdx = 1
			}
		}

		for _, values := range parser.Values {
			if channelIdx >= 0 && channelIdx < len(values) && payloadIdx >= 0 && payloadIdx < len(values) {
				channel := values[channelIdx]
				message := values[payloadIdx]

				// Publish message
				err := client.Publish(ctx, channel, message).Err()
				if err == nil {
					affectedRows++
				}
			}
		}
	default:
		return map[string]interface{}{"columns": []string{"error"}, "rows": [][]interface{}{{fmt.Sprintf("Unsupported table for INSERT: %s", parser.Table)}}, "affected_rows": 0}
	}
	return map[string]interface{}{"columns": []string{"status"}, "rows": [][]interface{}{{"OK"}}, "affected_rows": affectedRows}
}

func executeUpdate(p ConnectionParams, parser UpdateParser) map[ // executeUpdate handles UPDATE operations for virtual tables
string]interface{} {
	client, err := getClient(p)
	if err != nil {
		return map[string]interface{}{"columns": []string{"error"}, "rows": [][]interface{}{{err.Error()}}, "affected_rows": 0}
	}
	affectedRows := 0
	switch parser.Table {
	case "keys":
		var keyToUpdate string
		for _, cond := range // UPDATE keys SET value = 'newvalue', ttl = 60 WHERE key = 'mykey'
		parser.Conditions {
			if cond.Column == "key" && cond.Operator == "=" {
				keyToUpdate = cond.Value
				break
			}
		}
		if keyToUpdate != "" {
			updated := false
			if val, ok := parser.SetClauses["value"]; ok {
				err := client.Set(ctx, keyToUpdate, val, redis.KeepTTL).Err()
				if err == nil {
					updated = true
				}
			}
			if ttlStr, ok := parser.SetClauses["ttl"]; ok {
				ttl, err := strconv.Atoi(ttlStr)
				if err == nil {
					if ttl == -1 {
						if err := client.Persist(ctx, keyToUpdate).Err(); err == nil {
							updated = true
						}
					} else {
						if err := client.Expire(ctx, keyToUpdate, time.Duration(ttl)*time.Second).Err(); err == nil {
							updated = true
						}
					}
				}
			}
			if updated {
				affectedRows++
			}
		}
	case "hashes":
		var keyToUpdate, fieldToUpdate string
		for _, cond := range // UPDATE hashes SET value = 'newvalue' WHERE key = 'myhash' AND field = 'field1'
		// Maps to: HSET key field newvalue
		parser.Conditions {
			if cond.Column == "key" && cond.Operator == "=" {
				keyToUpdate = cond.Value
			}
			if cond.Column == "field" && cond.Operator == "=" {
				fieldToUpdate = cond.Value
			}
		}
		if keyToUpdate != "" && fieldToUpdate != "" && parser.SetClauses["value"] != "" {
			err := client.HSet(ctx, keyToUpdate, fieldToUpdate, parser.SetClauses["value"]).Err()
			if err == nil {
				affectedRows++
			}
		}
	case "zsets":
		var keyToUpdate, memberToUpdate string
		for _, cond := range // UPDATE zsets SET score = 200 WHERE key = 'myzset' AND member = 'member1'
		// Maps to: ZADD key score member (overwrites existing score)
		parser.Conditions {
			if cond.Column == "key" && cond.Operator == "=" {
				keyToUpdate = cond.Value
			}
			if cond.Column == "member" && cond.Operator == "=" || cond.Column == "value" && cond.Operator == "=" {
				memberToUpdate = cond.Value
			}
		}
		if keyToUpdate != "" && memberToUpdate != "" && parser.SetClauses["score"] != "" {
			score, err := strconv.ParseFloat(parser.SetClauses["score"], 64)
			if err == nil {
				err := client.ZAdd(ctx, keyToUpdate, &redis.Z{Score: score, Member: memberToUpdate}).Err()
				if err == nil {
					affectedRows++
				}
			}
		}
	default:
		return map[string]interface{}{"columns": []string{"error"}, "rows": [][]interface{}{{fmt.Sprintf("UPDATE not supported for table: %s", parser.Table)}}, "affected_rows": 0}
	}
	return map[string]interface{}{"columns": []string{"status"}, "rows": [][]interface{}{{"OK"}}, "affected_rows": affectedRows}
}

func executeDelete(p ConnectionParams, parser DeleteParser) map[ // executeDelete handles DELETE operations for virtual tables
string]interface{} {
	client, err := getClient(p)
	if err != nil {
		return map[string]interface{}{"columns": []string{"error"}, "rows": [][]interface{}{{err.Error()}}, "affected_rows": 0}
	}
	affectedRows := 0
	switch parser.Table {
	case "keys":
		var keyToDelete string
		for _, cond := range // DELETE FROM keys WHERE key = 'mykey'
		// Maps to: DEL key
		parser.Conditions {
			if cond.Column == "key" && cond.Operator == "=" {
				keyToDelete = cond.Value
				break
			}
		}
		if keyToDelete != "" {
			result, err := client.Del(ctx, keyToDelete).Result()
			if err == nil {
				affectedRows = int(result)
			}
		}
	case "hashes":
		var keyToDelete, fieldToDelete string
		for _, cond := range // DELETE FROM hashes WHERE key = 'myhash' AND field = 'field1'
		// Maps to: HDEL key field
		parser.Conditions {
			if cond.Column == "key" && cond.Operator == "=" {
				keyToDelete = cond.Value
			}
			if cond.Column == "field" && cond.Operator == "=" {
				fieldToDelete = cond.Value
			}
		}
		if keyToDelete != "" && fieldToDelete != "" {
			result, err := client.HDel(ctx, keyToDelete, fieldToDelete).Result()
			if err == nil {
				affectedRows = int(result)
			}
		}
	case "lists":
		var keyToDelete, valueToDelete string
		for _, cond := range // DELETE FROM lists WHERE key = 'mylist' AND value = 'item1'
		// Maps to: LREM key 0 value (remove all occurrences)
		parser.Conditions {
			if cond.Column == "key" && cond.Operator == "=" {
				keyToDelete = cond.Value
			}
			if cond.Column == "value" && cond.Operator == "=" {
				valueToDelete = cond.Value
			}
		}
		if keyToDelete != "" && valueToDelete != "" {
			result, err := client.LRem(ctx, keyToDelete, 0, valueToDelete).Result()
			if err == nil {
				affectedRows = int(result)
			}
		}
	case "sets":
		var keyToDelete, memberToDelete string
		for _, cond := range // DELETE FROM sets WHERE key = 'myset' AND member = 'member1'
		// Maps to: SREM key member
		parser.Conditions {
			if cond.Column == "key" && cond.Operator == "=" {
				keyToDelete = cond.Value
			}
			if cond.Column == "member" && cond.Operator == "=" {
				memberToDelete = cond.Value
			}
		}
		if keyToDelete != "" && memberToDelete != "" {
			result, err := client.SRem(ctx, keyToDelete, memberToDelete).Result()
			if err == nil {
				affectedRows = int(result)
			}
		}
	case "zsets":
		var keyToDelete, memberToDelete string
		for _, cond := range // DELETE FROM zsets WHERE key = 'myzset' AND member = 'member1'
		// Maps to: ZREM key member
		parser.Conditions {
			if cond.Column == "key" && cond.Operator == "=" {
				keyToDelete = cond.Value
			}
			if cond.Column == "member" && cond.Operator == "=" || cond.Column == "value" && cond.Operator == "=" {
				memberToDelete = cond.Value
			}
		}
		if keyToDelete != "" && memberToDelete != "" {
			result, err := client.ZRem(ctx, keyToDelete, memberToDelete).Result()
			if err == nil {
				affectedRows = int(result)
			}
		}
	case "pubsub_subscriptions":
		// DELETE FROM pubsub_subscriptions WHERE id = 'sub_xxx'
		var subscriptionID string
		for _, cond := range parser.Conditions {
			if cond.Column == "id" && cond.Operator == "=" {
				subscriptionID = cond.Value
				break
			}
		}
		if subscriptionID != "" {
			_, err := globalSubscriptionManager.Unsubscribe(subscriptionID)
			if err == nil {
				affectedRows = 1
			}
		}
	default:
		return map[string]interface{}{"columns": []string{"error"}, "rows": [][]interface{}{{fmt.Sprintf("DELETE not supported for table: %s", parser.Table)}}, "affected_rows": 0}
	}
	return map[string]interface{}{"columns": []string{"status"}, "rows": [][]interface{}{{"OK"}}, "affected_rows": affectedRows}
}

// executeScanPubSubChannels retrieves information about available Redis Pub/Sub channels
// Columns: channel, subscribers, is_pattern, last_message_time
// Data source: PUBSUB CHANNELS, PUBSUB NUMSUB
func executeScanPubSubChannels(p ConnectionParams, parser QueryParser, page, pageSize int) map[string]interface{} {
	client, err := getClient(p)
	if err != nil {
		return map[string]interface{}{
			"columns":       []string{"error"},
			"rows":          [][]interface{}{{err.Error()}},
			"affected_rows": 0,
		}
	}
	defer client.Close()

	allRows := [][]interface{}{}

	// Get all active channels using PUBSUB CHANNELS
	channels, err := client.PubSubChannels(ctx, "*").Result()
	if err != nil {
		return map[string]interface{}{
			"columns":       []string{"error"},
			"rows":          [][]interface{}{{fmt.Sprintf("Failed to get channels: %v", err)}},
			"affected_rows": 0,
		}
	}

	// For each channel, get the number of subscribers using PUBSUB NUMSUB
	for _, channel := range channels {
		numSubMap, err := client.PubSubNumSub(ctx, channel).Result()
		if err != nil {
			continue
		}

		subscribers := int64(0)
		if count, ok := numSubMap[channel]; ok {
			subscribers = count
		}

		// Check if this is a pattern channel (we can't determine this from Redis directly,
		// so we'll mark all as false for now - pattern channels are tracked separately)
		isPattern := false

		// last_message_time is not available from Redis PUBSUB commands
		// We'll use 0 to indicate "unknown"
		lastMessageTime := int64(0)

		rowMap := map[string]interface{}{
			"channel":           channel,
			"subscribers":       subscribers,
			"is_pattern":        isPattern,
			"last_message_time": lastMessageTime,
		}

		if matchesConditions(rowMap, parser.Conditions) {
			allRows = append(allRows, []interface{}{channel, subscribers, isPattern, lastMessageTime})
		}
	}

	// Apply sorting
	if len(parser.OrderBy) > 0 {
		allRows = applyOrderBy(allRows, []string{"channel", "subscribers", "is_pattern", "last_message_time"}, parser.OrderBy)
	}

	// Apply LIMIT/OFFSET from query
	if parser.Limit > 0 || parser.Offset > 0 {
		allRows = applyLimitOffset(allRows, parser.Limit, parser.Offset)
	}

	// Apply pagination
	pagedRows, hasMore, total := paginateRows(allRows, page, pageSize)

	return map[string]interface{}{
		"columns":       []string{"channel", "subscribers", "is_pattern", "last_message_time"},
		"rows":          pagedRows,
		"affected_rows": 0,
		"truncated":     hasMore,
		"pagination": map[string]interface{}{
			"page":       page,
			"page_size":  pageSize,
			"total_rows": total,
			"has_more":   hasMore,
		},
	}
}

// executeScanPubSubMessages retrieves messages from active subscriptions
// Columns: subscription_id, message_id, channel, payload, published_at, received_at
// Data source: MessageBuffer of active subscriptions
func executeScanPubSubMessages(p ConnectionParams, parser QueryParser, page, pageSize int) map[string]interface{} {
	allRows := [][]interface{}{}

	// Get all active subscriptions from the global subscription manager
	subscriptions := globalSubscriptionManager.ListSubscriptions()

	// Check if there's a specific subscription_id filter
	var specificSubID string
	for _, cond := range parser.Conditions {
		if cond.Column == "subscription_id" && cond.Operator == "=" {
			specificSubID = cond.Value
			break
		}
	}

	// Iterate through subscriptions and collect messages
	for _, sub := range subscriptions {
		// If filtering by subscription_id, skip non-matching subscriptions
		if specificSubID != "" && sub.ID != specificSubID {
			continue
		}

		// Get all messages from the subscription's buffer (without auto-ack)
		messages, _ := sub.Buffer.Get(10000, false) // Get up to 10000 messages

		for _, msg := range messages {
			rowMap := map[string]interface{}{
				"subscription_id": sub.ID,
				"message_id":      msg.ID,
				"channel":         msg.Channel,
				"payload":         msg.Payload,
				"published_at":    msg.PublishedAt,
				"received_at":     msg.ReceivedAt,
			}

			if matchesConditions(rowMap, parser.Conditions) {
				allRows = append(allRows, []interface{}{
					sub.ID,
					msg.ID,
					msg.Channel,
					msg.Payload,
					msg.PublishedAt,
					msg.ReceivedAt,
				})
			}
		}
	}

	// Apply sorting
	if len(parser.OrderBy) > 0 {
		allRows = applyOrderBy(allRows, []string{"subscription_id", "message_id", "channel", "payload", "published_at", "received_at"}, parser.OrderBy)
	}

	// Apply LIMIT/OFFSET from query
	if parser.Limit > 0 || parser.Offset > 0 {
		allRows = applyLimitOffset(allRows, parser.Limit, parser.Offset)
	}

	// Apply pagination
	pagedRows, hasMore, total := paginateRows(allRows, page, pageSize)

	return map[string]interface{}{
		"columns":       []string{"subscription_id", "message_id", "channel", "payload", "published_at", "received_at"},
		"rows":          pagedRows,
		"affected_rows": 0,
		"truncated":     hasMore,
		"pagination": map[string]interface{}{
			"page":       page,
			"page_size":  pageSize,
			"total_rows": total,
			"has_more":   hasMore,
		},
	}
}

// executeScanPubSubSubscriptions retrieves information about active subscriptions
// Columns: id, channel, is_pattern, created_at, ttl, buffer_size, buffer_used, messages_received, messages_dropped
// Data source: SubscriptionManager's active subscriptions
func executeScanPubSubSubscriptions(p ConnectionParams, parser QueryParser, page, pageSize int) map[string]interface{} {
	allRows := [][]interface{}{}

	// Clean up expired subscriptions first
	globalSubscriptionManager.CleanupExpiredSubscriptions()

	// Get all active subscriptions
	subscriptions := globalSubscriptionManager.ListSubscriptions()

	currentTime := time.Now().Unix()

	for _, sub := range subscriptions {
		// Calculate TTL (time remaining until expiration)
		ttl := sub.ExpiresAt - currentTime
		if ttl < 0 {
			ttl = 0
		}

		// Get buffer statistics
		bufferUsed := int64(sub.Buffer.Size())
		bufferSize := int64(sub.Buffer.capacity)

		// Calculate messages received (nextID - 1 since IDs start at 1)
		messagesReceived := sub.Buffer.nextID - 1

		// Calculate messages dropped (total received - current buffer size)
		// This is an approximation since we don't track drops explicitly
		messagesDropped := int64(0)
		if messagesReceived > bufferSize {
			messagesDropped = messagesReceived - bufferSize
		}

		rowMap := map[string]interface{}{
			"id":                sub.ID,
			"channel":           sub.Channel,
			"is_pattern":        sub.IsPattern,
			"created_at":        sub.CreatedAt,
			"ttl":               ttl,
			"buffer_size":       bufferSize,
			"buffer_used":       bufferUsed,
			"messages_received": messagesReceived,
			"messages_dropped":  messagesDropped,
		}

		if matchesConditions(rowMap, parser.Conditions) {
			allRows = append(allRows, []interface{}{
				sub.ID,
				sub.Channel,
				sub.IsPattern,
				sub.CreatedAt,
				ttl,
				bufferSize,
				bufferUsed,
				messagesReceived,
				messagesDropped,
			})
		}
	}

	// Apply sorting
	if len(parser.OrderBy) > 0 {
		allRows = applyOrderBy(allRows, []string{"id", "channel", "is_pattern", "created_at", "ttl", "buffer_size", "buffer_used", "messages_received", "messages_dropped"}, parser.OrderBy)
	}

	// Apply LIMIT/OFFSET from query
	if parser.Limit > 0 || parser.Offset > 0 {
		allRows = applyLimitOffset(allRows, parser.Limit, parser.Offset)
	}

	// Apply pagination
	pagedRows, hasMore, total := paginateRows(allRows, page, pageSize)

	return map[string]interface{}{
		"columns":       []string{"id", "channel", "is_pattern", "created_at", "ttl", "buffer_size", "buffer_used", "messages_received", "messages_dropped"},
		"rows":          pagedRows,
		"affected_rows": 0,
		"truncated":     hasMore,
		"pagination": map[string]interface{}{
			"page":       page,
			"page_size":  pageSize,
			"total_rows": total,
			"has_more":   hasMore,
		},
	}
}
