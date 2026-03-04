package main

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
)

func runRequest(t *testing.T, req Request) Response {
	var buf bytes.Buffer
	out = &buf

	handleRequest(req)

	var resp Response
	err := json.Unmarshal(buf.Bytes(), &resp)
	if err != nil {
		t.Fatalf("failed to parse response: %v\nOutput was: %s", err, buf.String())
	}
	return resp
}

func setupTestDB(t *testing.T) (*miniredis.Miniredis, ConnectionParams) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	hostPort := strings.Split(s.Addr(), ":")
	host := hostPort[0]
	port, _ := strconv.Atoi(hostPort[1])

	params := ConnectionParams{
		Driver:   "redis",
		Host:     &host,
		Port:     &port,
		Database: "0",
	}
	return s, params
}

func TestTestConnection(t *testing.T) {
	s, params := setupTestDB(t)
	defer s.Close()

	paramsJSON, _ := json.Marshal(map[string]interface{}{"params": params})
	req := Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "test_connection",
		Params:  paramsJSON,
	}

	resp := runRequest(t, req)
	
	result, ok := resp.Result.(map[string]interface{})
	if !ok || result["success"] != true {
		t.Errorf("Expected success=true, got %v", resp.Result)
	}
}

func TestGetTables(t *testing.T) {
	req := Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`2`),
		Method:  "get_tables",
		Params:  json.RawMessage(`{}`),
	}

	resp := runRequest(t, req)
	
	tables, ok := resp.Result.([]interface{})
	if !ok || len(tables) != 5 {
		t.Errorf("Expected 5 tables, got %v", resp.Result)
	}
}

func TestGetColumns(t *testing.T) {
	req := Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`3`),
		Method:  "get_columns",
		Params:  json.RawMessage(`{"table":"keys"}`),
	}

	resp := runRequest(t, req)
	
	columns, ok := resp.Result.([]interface{})
	if !ok || len(columns) != 3 {
		t.Errorf("Expected 3 columns for keys table, got %v", resp.Result)
	}
}

func TestExecuteQueryKeys(t *testing.T) {
	s, params := setupTestDB(t)
	defer s.Close()

	// Seed miniredis
	s.Set("mykey1", "value1")
	s.HSet("myhash", "f1", "v1")

	paramsJSON, _ := json.Marshal(map[string]interface{}{
		"params": params,
		"query": "SELECT * FROM keys",
		"page": 0,
		"page_size": 10,
	})
	
	req := Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`4`),
		Method:  "execute_query",
		Params:  paramsJSON,
	}

	resp := runRequest(t, req)
	
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected object result, got %v", resp.Result)
	}

	rows, ok := result["rows"].([]interface{})
	if !ok || len(rows) != 2 {
		t.Errorf("Expected 2 rows, got %v", result["rows"])
	}
}

func TestExecuteQueryHashes(t *testing.T) {
	s, params := setupTestDB(t)
	defer s.Close()

	// Seed miniredis
	s.HSet("myhash", "field1", "value1")
	s.HSet("myhash", "field2", "value2")

	paramsJSON, _ := json.Marshal(map[string]interface{}{
		"params": params,
		"query": "SELECT * FROM hashes WHERE key = 'myhash'",
		"page": 0,
		"page_size": 10,
	})
	
	req := Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`5`),
		Method:  "execute_query",
		Params:  paramsJSON,
	}

	resp := runRequest(t, req)
	
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected object result, got %v", resp.Result)
	}

	rows, ok := result["rows"].([]interface{})
	if !ok || len(rows) != 2 {
		t.Errorf("Expected 2 rows from hashes, got %v", result["rows"])
	}
}
