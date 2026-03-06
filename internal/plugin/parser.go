package plugin

import (
	"regexp"
	"strconv"
	"strings"
)

func parseQuery(query string) QueryParser {
	parser := QueryParser{Limit: -1, Offset: 0}
	upperQuery := strings.ToUpper(query)
	normalizedQuery := strings.ReplaceAll(upperQuery, "\"", "")
	if strings.Contains(normalizedQuery, "FROM KEYS") {
		parser.Table = "keys"
	} else if strings.Contains(normalizedQuery, "FROM HASHES") {
		parser.Table = "hashes"
	} else if strings.Contains(normalizedQuery, "FROM LISTS") {
		parser.Table = "lists"
	} else if strings.Contains(normalizedQuery, "FROM SETS") {
		parser.Table = "sets"
	} else if strings.Contains(normalizedQuery, "FROM ZSETS") {
		parser.Table = "zsets"
	} else if strings.Contains(normalizedQuery, "FROM PUBSUB_CHANNELS") {
		parser.Table = "pubsub_channels"
	} else if strings.Contains(normalizedQuery, "FROM PUBSUB_MESSAGES") {
		parser.Table = "pubsub_messages"
	} else if strings.Contains(normalizedQuery, "FROM PUBSUB_SUBSCRIPTIONS") {
		parser.Table = "pubsub_subscriptions"
	}
	whereIdx := strings.Index(upperQuery, "WHERE")
	if whereIdx != -1 {
		wherePart := query[whereIdx+5:]
		endIdx := len(wherePart)
		for _, keyword := range // parseQuery extracts table, conditions, limit, and offset from SQL query
		[]string{"LIMIT", "ORDER BY", "GROUP BY"} {
			if idx := strings.Index(strings.ToUpper(wherePart), keyword); idx != -1 && idx < endIdx {
				endIdx = idx
			}
		}
		wherePart = strings.TrimSpace(wherePart[:endIdx])
		condParts := strings.Split(wherePart, " AND ")
		for _, condPart := range condParts {
			condPart = strings.TrimSpace(condPart)
			if condPart == "" {
				continue
			}
			cond := parseCondition(condPart)
			if cond.Column != "" {
				parser.Conditions = append(parser.Conditions, cond)
			}
		}
	}
	orderByRegex := regexp.MustCompile(`(?i)ORDER\s+BY\s+(.+?)(?:\s+LIMIT|\s+OFFSET|$)`)
	if matches := orderByRegex.FindStringSubmatch(query); len(matches) > 1 {
		orderByPart := strings.TrimSpace(matches[1])
		orderCols := strings.Split(orderByPart, ",")
		for _, orderCol := range orderCols {
			orderCol = strings.TrimSpace(orderCol)
			if orderCol == "" {
				continue
			}
			parts := strings.Fields(orderCol)
			if len(parts) > 0 {
				ob := OrderBy{Column: strings.ToLower(parts[0]), Direction: "ASC"}
				if len(parts) > 1 {
					dir := strings.ToUpper(parts[1])
					if dir == "DESC" || dir == "ASC" {
						ob.Direction = dir
					}
				}
				parser.OrderBy = append(parser.OrderBy, ob)
			}
		}
	}
	limitRegex := regexp.MustCompile(`(?i)LIMIT\s+(\d+)`)
	if matches := limitRegex.FindStringSubmatch(query); len(matches) > 1 {
		parser.Limit, _ = strconv.Atoi(matches[1])
	}
	offsetRegex := regexp.MustCompile(`(?i)OFFSET\s+(\d+)`)
	if matches := offsetRegex.FindStringSubmatch(query); len(matches) > 1 {
		parser.Offset, _ = strconv.Atoi(matches[1])
	}
	return parser
}

func parseInsert(query string) InsertParser {
	parser := InsertParser{Columns: []string{ // parseInsert parses INSERT INTO statements
			// Supports: INSERT INTO table (col1, col2) VALUES (val1, val2), (val3, val4)
	}, Values: [][]string{}}
	tableRegex := regexp.MustCompile(`(?i)INSERT\s+INTO\s+(\w+)`)
	if matches := tableRegex.FindStringSubmatch(query); len(matches) > 1 {
		parser.Table = strings.ToLower(matches[1])
	}
	colRegex := regexp.MustCompile(`(?i)\(([^)]+)\)\s+VALUES`)
	if matches := colRegex.FindStringSubmatch(query); len(matches) > 1 {
		colStr := matches[1]
		cols := strings.Split(colStr, ",")
		for _, col := range cols {
			parser.Columns = append(parser.Columns, strings.TrimSpace(strings.ToLower(col)))
		}
	}
	valuesRegex := regexp.MustCompile(`(?i)VALUES\s+(.+)$`)
	if matches := valuesRegex.FindStringSubmatch(query); len(matches) > 1 {
		valuesStr := matches[1]
		valueSets := regexp.MustCompile(`\)\s*,\s*\(`).Split(valuesStr, -1)
		for _, valueSet := range valueSets {
			valueSet = strings.TrimPrefix(strings.TrimSpace(valueSet), "(")
			valueSet = strings.TrimSuffix(strings.TrimSpace(valueSet), ")")
			values := []string{}
			inQuote := false
			current := ""
			quoteChar := rune(0)
			for _, ch := range valueSet {
				if (ch == '\'' || ch == '"') && quoteChar == 0 {
					quoteChar = ch
					inQuote = true
				} else if ch == quoteChar {
					inQuote = false
					quoteChar = 0
				} else if ch == ',' && !inQuote {
					values = append(values, strings.TrimSpace(strings.Trim(current, "'\"")))
					current = ""
					continue
				} else {
					current += string(ch)
				}
			}
			if current != "" {
				values = append(values, strings.TrimSpace(strings.Trim(current, "'\"")))
			}
			if len(values) > 0 {
				parser.Values = append(parser.Values, values)
			}
		}
	}
	return parser
}

func parseUpdate(query string) UpdateParser {
	parser := UpdateParser{SetClauses: make(map[ // parseUpdate parses UPDATE statements
	// Supports: UPDATE table SET col1 = val1, col2 = val2 WHERE condition
	string]string), Conditions: []QueryCondition{}}
	tableRegex := regexp.MustCompile(`(?i)UPDATE\s+(\w+)`)
	if matches := tableRegex.FindStringSubmatch(query); len(matches) > 1 {
		parser.Table = strings.ToLower(matches[1])
	}
	setRegex := regexp.MustCompile(`(?i)SET\s+(.+?)(?:\s+WHERE|$)`)
	if matches := setRegex.FindStringSubmatch(query); len(matches) > 1 {
		setStr := matches[1]
		setClauses := splitByCommaOutsideQuotes(setStr)
		for _, clause := range setClauses {
			parts := strings.SplitN(clause, "=", 2)
			if len(parts) == 2 {
				col := strings.TrimSpace(strings.ToLower(parts[0]))
				val := strings.TrimSpace(strings.Trim(parts[1], "'\""))
				parser.SetClauses[col] = val
			}
		}
	}
	whereRegex := regexp.MustCompile(`(?i)WHERE\s+(.+)$`)
	if matches := whereRegex.FindStringSubmatch(query); len(matches) > 1 {
		whereStr := matches[1]
		condStrs := strings.Split(whereStr, " AND ")
		for _, condStr := range condStrs {
			condStr = strings.TrimSpace(condStr)
			if condStr != "" {
				parser.Conditions = append(parser.Conditions, parseCondition(condStr))
			}
		}
	}
	return parser
}

func parseDelete(query string) DeleteParser {
	parser := DeleteParser{Conditions: []QueryCondition{ // parseDelete parses DELETE statements
		// Supports: DELETE FROM table WHERE condition
	}}
	tableRegex := regexp.MustCompile(`(?i)DELETE\s+FROM\s+(\w+)`)
	if matches := tableRegex.FindStringSubmatch(query); len(matches) > 1 {
		parser.Table = strings.ToLower(matches[1])
	}
	whereRegex := regexp.MustCompile(`(?i)WHERE\s+(.+)$`)
	if matches := whereRegex.FindStringSubmatch(query); len(matches) > 1 {
		whereStr := matches[1]
		condStrs := strings.Split(whereStr, " AND ")
		for _, condStr := range condStrs {
			condStr = strings.TrimSpace(condStr)
			if condStr != "" {
				parser.Conditions = append(parser.Conditions, parseCondition(condStr))
			}
		}
	}
	return parser
}

func splitByCommaOutsideQuotes(s string) []string { // splitByCommaOutsideQuotes splits a string by comma, but not inside quotes

	result := []string{}
	current := ""
	inQuote := false
	quoteChar := rune(0)
	for _, ch := range s {
		if (ch == '\'' || ch == '"') && quoteChar == 0 {
			quoteChar = ch
			inQuote = true
			current += string(ch)
		} else if ch == quoteChar {
			inQuote = false
			quoteChar = 0
			current += string(ch)
		} else if ch == ',' && !inQuote {
			result = append(result, strings.TrimSpace(current))
			current = ""
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		result = append(result, strings.TrimSpace(current))
	}
	return result
}

func parseCondition(condStr string) QueryCondition {
	cond := QueryCondition{}
	upperCondStr := strings.ToUpper(condStr)
	inIdx := strings.Index(upperCondStr, " IN ")
	if inIdx != -1 {
		cond.Column = strings.TrimSpace(strings.ToLower(condStr[: // parseCondition parses a single condition like "key = 'value'" or "score > 10" or "type IN ('hash', 'set')"
		inIdx]))
		cond.Operator = "IN"
		valuesPart := strings.TrimSpace(condStr[inIdx+4:])
		valuesPart = strings.TrimPrefix(valuesPart, "(")
		valuesPart = strings.TrimSuffix(valuesPart, ")")
		cond.Value = valuesPart
		return cond
	}
	operators := []string{">=", "<=", "!=", "<>", " LIKE ", "=", ">", "<"}
	for _, op := range operators {
		var idx int
		if strings.Contains(op, " ") {
			idx = strings.Index(upperCondStr, op)
		} else {
			idx = strings.Index(condStr, op)
		}
		if idx != -1 {
			cond.Column = strings.TrimSpace(strings.ToLower(condStr[:idx]))
			cond.Operator = strings.TrimSpace(strings.ToUpper(op))
			valueStart := idx + len(op)
			if valueStart < len(condStr) {
				value := strings.TrimSpace(condStr[valueStart:])
				value = strings.Trim(value, "'\"")
				cond.Value = value
			}
			break
		}
	}
	return cond
}
