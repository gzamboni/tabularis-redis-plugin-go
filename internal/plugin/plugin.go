package plugin

import (
	"bufio"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/go-redis/redis/v8"
)

var ctx = context.Background()

var out io.Writer = os.Stdout

// Global client pool to reuse Redis connections
var (
	clientPool      = make(map[string]*redis.Client)
	clientPoolMutex sync.RWMutex
)

// cleanQuery removes SQL comments and trims whitespace
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

func Run() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			sendError(nil, -32700, "Parse error")
			continue
		}
		fmt.Fprintf(os.Stderr, "DEBUG: Method called: %s\n", req.Method)
		handleRequest(req)
	}
}

func sendResponse(id json.RawMessage, result interface{}, err error) {
	if err != nil {
		sendError(id, -32603, err.Error())
		return
	}
	resp := Response{JSONRPC: "2.0", ID: id, Result: result}
	data, _ := json.Marshal(resp)
	fmt.Fprintln(out, string(data))
}

func sendError(id json.RawMessage, code int, message string) {
	resp := Response{JSONRPC: "2.0", ID: id, Error: RPCError{Code: code, Message: message}}
	data, _ := json.Marshal(resp)
	fmt.Fprintln(out, string(data))
}

// getConnectionKey generates a unique key for connection parameters
func getConnectionKey(p ConnectionParams) string {
	addr := "localhost:6379"
	if p.Host != nil && p.Port != nil {
		addr = fmt.Sprintf("%s:%d", *p.Host, *p.Port)
	} else if p.Host != nil {
		addr = fmt.Sprintf("%s:6379", *p.Host)
	}
	db := p.Database
	username := ""
	if p.Username != nil {
		username = *p.Username
	}
	password := ""
	if p.Password != nil {
		password = *p.Password
	}
	// Create a unique key based on connection parameters
	key := fmt.Sprintf("%s:%s:%s:%s", addr, db, username, password)
	return fmt.Sprintf("%x", md5.Sum([]byte(key)))
}

func getClient(p ConnectionParams) (*redis.Client, error) {
	// Generate connection key
	connKey := getConnectionKey(p)

	// Check if client already exists in pool
	clientPoolMutex.RLock()
	if client, exists := clientPool[connKey]; exists {
		clientPoolMutex.RUnlock()
		// Test if connection is still alive
		if err := client.Ping(ctx).Err(); err == nil {
			return client, nil
		}
		// Connection is dead, remove it from pool
		clientPoolMutex.Lock()
		delete(clientPool, connKey)
		clientPoolMutex.Unlock()
	} else {
		clientPoolMutex.RUnlock()
	}

	// Create new client
	addr := "localhost:6379"
	if p.Host != nil && p.Port != nil {
		addr = fmt.Sprintf("%s:%d", *p.Host, *p.Port)
	} else if p.Host != nil {
		addr = fmt.Sprintf("%s:6379", *p.Host)
	}
	db, _ := strconv.Atoi(p.Database)
	opts := &redis.Options{Addr: addr, DB: db}
	if p.Username != nil && *p.Username != "" {
		if !(*p.Username == "default" && (p.Password == nil || *p.Password == "")) {
			opts.Username = *p.Username
		}
	}
	if p.Password != nil && *p.Password != "" {
		opts.Password = *p.Password
	}

	client := redis.NewClient(opts)

	// Store in pool
	clientPoolMutex.Lock()
	clientPool[connKey] = client
	clientPoolMutex.Unlock()

	return client, nil
}

func getTableColumns(table string) []map[string]interface{} {
	switch table {
	case "keys":
		return []map[string]interface{}{{"name": "key", "data_type": "STRING", "is_pk": true, "is_nullable": false, "is_auto_increment": false}, {"name": "type", "data_type": "STRING", "is_pk": false, "is_nullable": false, "is_auto_increment": false}, {"name": "ttl", "data_type": "INTEGER", "is_pk": false, "is_nullable": false, "is_auto_increment": false}, {"name": "value", "data_type": "STRING", "is_pk": false, "is_nullable": true, "is_auto_increment": false}}
	case "hashes":
		return []map[string]interface{}{{"name": "key", "data_type": "STRING", "is_pk": true, "is_nullable": false, "is_auto_increment": false}, {"name": "field", "data_type": "STRING", "is_pk": true, "is_nullable": false, "is_auto_increment": false}, {"name": "value", "data_type": "STRING", "is_pk": false, "is_nullable": false, "is_auto_increment": false}}
	case "lists":
		return []map[string]interface{}{{"name": "key", "data_type": "STRING", "is_pk": true, "is_nullable": false, "is_auto_increment": false}, {"name": "index", "data_type": "INTEGER", "is_pk": true, "is_nullable": false, "is_auto_increment": false}, {"name": "value", "data_type": "STRING", "is_pk": false, "is_nullable": false, "is_auto_increment": false}}
	case "sets":
		return []map[string]interface{}{{"name": "key", "data_type": "STRING", "is_pk": true, "is_nullable": false, "is_auto_increment": false}, {"name": "value", "data_type": "STRING", "is_pk": true, "is_nullable": false, "is_auto_increment": false}}
	case "zsets":
		return []map[string]interface{}{{"name": "key", "data_type": "STRING", "is_pk": true, "is_nullable": false, "is_auto_increment": false}, {"name": "value", "data_type": "STRING", "is_pk": true, "is_nullable": false, "is_auto_increment": false}, {"name": "score", "data_type": "NUMERIC", "is_pk": false, "is_nullable": false, "is_auto_increment": false}}
	case "pubsub_channels":
		return []map[string]interface{}{{"name": "channel", "data_type": "STRING", "is_pk": true, "is_nullable": false, "is_auto_increment": false}, {"name": "subscribers", "data_type": "INTEGER", "is_pk": false, "is_nullable": false, "is_auto_increment": false}, {"name": "is_pattern", "data_type": "BOOLEAN", "is_pk": false, "is_nullable": false, "is_auto_increment": false}, {"name": "last_message_time", "data_type": "INTEGER", "is_pk": false, "is_nullable": false, "is_auto_increment": false}}
	case "pubsub_messages":
		return []map[string]interface{}{{"name": "subscription_id", "data_type": "STRING", "is_pk": true, "is_nullable": false, "is_auto_increment": false}, {"name": "message_id", "data_type": "INTEGER", "is_pk": true, "is_nullable": false, "is_auto_increment": false}, {"name": "channel", "data_type": "STRING", "is_pk": false, "is_nullable": false, "is_auto_increment": false}, {"name": "payload", "data_type": "STRING", "is_pk": false, "is_nullable": false, "is_auto_increment": false}, {"name": "published_at", "data_type": "INTEGER", "is_pk": false, "is_nullable": false, "is_auto_increment": false}, {"name": "received_at", "data_type": "INTEGER", "is_pk": false, "is_nullable": false, "is_auto_increment": false}}
	case "pubsub_subscriptions":
		return []map[string]interface{}{{"name": "id", "data_type": "STRING", "is_pk": true, "is_nullable": false, "is_auto_increment": false}, {"name": "channel", "data_type": "STRING", "is_pk": false, "is_nullable": false, "is_auto_increment": false}, {"name": "is_pattern", "data_type": "BOOLEAN", "is_pk": false, "is_nullable": false, "is_auto_increment": false}, {"name": "created_at", "data_type": "INTEGER", "is_pk": false, "is_nullable": false, "is_auto_increment": false}, {"name": "ttl", "data_type": "INTEGER", "is_pk": false, "is_nullable": false, "is_auto_increment": false}, {"name": "buffer_size", "data_type": "INTEGER", "is_pk": false, "is_nullable": false, "is_auto_increment": false}, {"name": "buffer_used", "data_type": "INTEGER", "is_pk": false, "is_nullable": false, "is_auto_increment": false}, {"name": "messages_received", "data_type": "INTEGER", "is_pk": false, "is_nullable": false, "is_auto_increment": false}, {"name": "messages_dropped", "data_type": "INTEGER", "is_pk": false, "is_nullable": false, "is_auto_increment": false}}
	}
	return []map[string]interface{}{}
}

func handleRequest(req Request) {
	switch req.Method {
	case "test_connection":
		var p struct {
			Params ConnectionParams `json:"params"`
		}
		json.Unmarshal(req.Params, &p)
		client, _ := getClient(p.Params)
		err := client.Ping(ctx).Err()
		if err != nil {
			sendResponse(req.ID, map[string]bool{"success": false}, nil)
		} else {
			sendResponse(req.ID, map[string]bool{"success": true}, nil)
		}
	case "get_databases":
		dbs := []string{}
		for i := 0; i < 16; i++ {
			dbs = append(dbs, strconv.Itoa(i))
		}
		sendResponse(req.ID, dbs, nil)
	case "get_schemas", "get_views", "get_routines", "get_routine_parameters", "get_foreign_keys", "get_indexes", "get_all_columns_batch", "get_all_foreign_keys_batch":
		sendResponse(req.ID, []interface{}{}, nil)
	case "get_tables":
		tables := []map[string]interface{}{{"name": "keys", "schema": nil, "comment": "All Redis keys"}, {"name": "hashes", "schema": nil, "comment": "All fields in hashes"}, {"name": "lists", "schema": nil, "comment": "All elements in lists"}, {"name": "sets", "schema": nil, "comment": "All members in sets"}, {"name": "zsets", "schema": nil, "comment": "All members in sorted sets"}, {"name": "pubsub_channels", "schema": nil, "comment": "Available Redis Pub/Sub channels"}, {"name": "pubsub_messages", "schema": nil, "comment": "Messages from active subscriptions"}, {"name": "pubsub_subscriptions", "schema": nil, "comment": "Active Pub/Sub subscriptions"}}
		sendResponse(req.ID, tables, nil)
	case "get_columns":
		var p struct {
			Table string `json:"table"`
		}
		json.Unmarshal(req.Params, &p)
		sendResponse(req.ID, getTableColumns(p.Table), nil)
	case "get_schema_snapshot":
		tables := []string{"keys", "hashes", "lists", "sets", "zsets", "pubsub_channels", "pubsub_messages", "pubsub_subscriptions"}
		snapshotTables := []map[string]interface{}{}
		snapshotColumns := map[string]interface{}{}
		for _, t := range tables {
			snapshotTables = append(snapshotTables, map[string]interface{}{"name": t, "schema": nil})
			snapshotColumns[t] = getTableColumns(t)
		}
		snapshot := map[string]interface{}{"tables": snapshotTables, "columns": snapshotColumns, "foreign_keys": map[string]interface{}{}}
		sendResponse(req.ID, snapshot, nil)
	case "execute_query":
		var p struct {
			Params   ConnectionParams `json:"params"`
			Query    string           `json:"query"`
			Page     int              `json:"page"`
			PageSize int              `json:"page_size"`
		}
		json.Unmarshal(req.Params, &p)
		query := cleanQuery(p.Query)
		queryUpper := strings.ToUpper(query)
		var result map[string]interface{}
		if strings.HasPrefix(queryUpper, "INSERT") {
			parser := parseInsert(query)
			result = executeInsert(p.Params, parser)
		} else if strings.HasPrefix(queryUpper, "UPDATE") {
			parser := parseUpdate(query)
			result = executeUpdate(p.Params, parser)
		} else if strings.HasPrefix(queryUpper, "DELETE") {
			parser := parseDelete(query)
			result = executeDelete(p.Params, parser)
		} else {
			parser := parseQuery(query)
			switch parser.Table {
			case "keys":
				result = executeScanKeys(p.Params, parser, p.Page, p.PageSize)
			case "hashes":
				result = executeScanHashes(p.Params, parser, p.Page, p.PageSize)
			case "lists":
				result = executeScanLists(p.Params, parser, p.Page, p.PageSize)
			case "sets":
				result = executeScanSets(p.Params, parser, p.Page, p.PageSize)
			case "zsets":
				result = executeScanZSets(p.Params, parser, p.Page, p.PageSize)
			case "pubsub_channels":
				result = executeScanPubSubChannels(p.Params, parser, p.Page, p.PageSize)
			case "pubsub_messages":
				result = executeScanPubSubMessages(p.Params, parser, p.Page, p.PageSize)
			case "pubsub_subscriptions":
				result = executeScanPubSubSubscriptions(p.Params, parser, p.Page, p.PageSize)
			default:
				result = map[string]interface{}{"columns": []string{"error"}, "rows": [][]interface{}{{fmt.Sprintf("Unsupported query: %s. Try 'SELECT * FROM keys' or 'SELECT * FROM hashes WHERE key = \"mykey\"'", p.Query)}}, "affected_rows": 0, "truncated": false, "has_more": false, "pagination": nil}
			}
		}
		sendResponse(req.ID, result, nil)
	case "update_record":
		var p struct {
			Params  ConnectionParams `json:"params"`
			Table   string           `json:"table"`
			PkCol   string           `json:"pk_col"`
			PkVal   interface{}      `json:"pk_val"`
			ColName string           `json:"col_name"`
			NewVal  interface{}      `json:"new_val"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			sendError(req.ID, -32602, err.Error())
			return
		}
		pkValStr := fmt.Sprintf("%v", p.PkVal)
		newValStr := fmt.Sprintf("%v", p.NewVal)
		parser := UpdateParser{Table: p.Table, SetClauses: map[string]string{p.ColName: newValStr}, Conditions: []QueryCondition{{Column: p.PkCol, Operator: "=", Value: pkValStr}}}
		res := executeUpdate(p.Params, parser)
		if len(res["columns"].([]string)) > 0 && res["columns"].([]string)[0] == "error" {
			sendError(req.ID, -32603, fmt.Sprintf("%v", res["rows"].([][]interface{})[0][0]))
		} else {
			sendResponse(req.ID, res["affected_rows"], nil)
		}
	case "delete_record":
		var p struct {
			Params ConnectionParams `json:"params"`
			Table  string           `json:"table"`
			PkCol  string           `json:"pk_col"`
			PkVal  interface{}      `json:"pk_val"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			sendError(req.ID, -32602, err.Error())
			return
		}
		pkValStr := fmt.Sprintf("%v", p.PkVal)
		parser := DeleteParser{Table: p.Table, Conditions: []QueryCondition{{Column: p.PkCol, Operator: "=", Value: pkValStr}}}
		res := executeDelete(p.Params, parser)
		if len(res["columns"].([]string)) > 0 && res["columns"].([]string)[0] == "error" {
			sendError(req.ID, -32603, fmt.Sprintf("%v", res["rows"].([][]interface{})[0][0]))
		} else {
			sendResponse(req.ID, res["affected_rows"], nil)
		}
	case "insert_record":
		var p struct {
			Params ConnectionParams       `json:"params"`
			Table  string                 `json:"table"`
			Data   map[string]interface{} `json:"data"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			sendError(req.ID, -32602, err.Error())
			return
		}
		var values []string
		switch p.Table {
		case "keys", "lists", "sets":
			values = []string{fmt.Sprintf("%v", p.Data["key"]), fmt.Sprintf("%v", p.Data["value"])}
		case "hashes":
			values = []string{fmt.Sprintf("%v", p.Data["key"]), fmt.Sprintf("%v", p.Data["field"]), fmt.Sprintf("%v", p.Data["value"])}
		case "zsets":
			values = []string{fmt.Sprintf("%v", p.Data["key"]), fmt.Sprintf("%v", p.Data["value"]), fmt.Sprintf("%v", p.Data["score"])}
		}
		parser := InsertParser{Table: p.Table, Columns: []string{}, Values: [][]string{values}}
		res := executeInsert(p.Params, parser)
		if len(res["columns"].([]string)) > 0 && res["columns"].([]string)[0] == "error" {
			sendError(req.ID, -32603, fmt.Sprintf("%v", res["rows"].([][]interface{})[0][0]))
		} else {
			sendResponse(req.ID, res["affected_rows"], nil)
		}
	case "pubsub_subscribe":
		var p PubSubSubscribeRequest
		if err := json.Unmarshal(req.Params, &p); err != nil {
			sendError(req.ID, -32602, "Invalid params: "+err.Error())
			return
		}
		result, err := HandlePubSubSubscribe(p)
		sendResponse(req.ID, result, err)
	case "pubsub_unsubscribe":
		var p PubSubUnsubscribeRequest
		if err := json.Unmarshal(req.Params, &p); err != nil {
			sendError(req.ID, -32602, "Invalid params: "+err.Error())
			return
		}
		result, err := HandlePubSubUnsubscribe(p)
		sendResponse(req.ID, result, err)
	case "pubsub_publish":
		var p PubSubPublishRequest
		if err := json.Unmarshal(req.Params, &p); err != nil {
			sendError(req.ID, -32602, "Invalid params: "+err.Error())
			return
		}
		result, err := HandlePubSubPublish(p)
		sendResponse(req.ID, result, err)
	case "pubsub_poll_messages":
		var p PubSubPollMessagesRequest
		if err := json.Unmarshal(req.Params, &p); err != nil {
			sendError(req.ID, -32602, "Invalid params: "+err.Error())
			return
		}
		result, err := HandlePubSubPollMessages(p)
		sendResponse(req.ID, result, err)
	case "pubsub_acknowledge_messages":
		var p PubSubAcknowledgeMessagesRequest
		if err := json.Unmarshal(req.Params, &p); err != nil {
			sendError(req.ID, -32602, "Invalid params: "+err.Error())
			return
		}
		result, err := HandlePubSubAcknowledgeMessages(p)
		sendResponse(req.ID, result, err)
	default:
		sendError(req.ID, -32601, "Method not found")
	}
}
