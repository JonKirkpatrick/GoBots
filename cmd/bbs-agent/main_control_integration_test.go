package main

import (
	"bufio"
	"encoding/json"
	"net"
	"path/filepath"
	"testing"
	"time"
)

func TestControlSocketIntegration_LocalOnly(t *testing.T) {
	t.Helper()

	sockPath := filepath.Join(t.TempDir(), "bbs-agent-control.sock")

	ag, err := newAgent(runtimeConfig{server: "", name: "agent_bot"})
	if err != nil {
		t.Fatalf("newAgent returned error: %v", err)
	}
	ag.name = "agent_bot"
	defer ag.shutdown("test_done")

	if err := ag.startControlListener("unix://" + sockPath); err != nil {
		t.Fatalf("startControlListener returned error: %v", err)
	}

	conn, err := net.DialTimeout("unix", sockPath, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to connect to control socket: %v", err)
	}
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 1024), maxScannerToken)

	first := readControlMessage(t, scanner)
	if first.Type != "control_hello" {
		t.Fatalf("first message type = %q, want %q", first.Type, "control_hello")
	}

	writeControlMessage(t, conn, contractMessage{V: contractVersion, Type: "ping", ID: "req-ping", Payload: map[string]interface{}{}})
	pingResp := readControlMessage(t, scanner)
	if pingResp.Type != "pong" {
		t.Fatalf("ping response type = %q, want %q", pingResp.Type, "pong")
	}
	if pingResp.ID != "req-ping" {
		t.Fatalf("ping response id = %q, want %q", pingResp.ID, "req-ping")
	}

	writeControlMessage(t, conn, contractMessage{V: contractVersion, Type: "status", ID: "req-status", Payload: map[string]interface{}{}})
	statusResp := readControlMessage(t, scanner)
	if statusResp.Type != "status" {
		t.Fatalf("status response type = %q, want %q", statusResp.Type, "status")
	}
	if statusResp.ID != "req-status" {
		t.Fatalf("status response id = %q, want %q", statusResp.ID, "req-status")
	}

	statusPayload, ok := statusResp.Payload.(map[string]interface{})
	if !ok {
		t.Fatalf("status payload has unexpected type: %T", statusResp.Payload)
	}
	if statusPayload["server_connected"] != false {
		t.Fatalf("status payload server_connected = %#v, want false", statusPayload["server_connected"])
	}

	writeControlMessage(t, conn, contractMessage{V: contractVersion, Type: "server_access", ID: "req-access", Payload: map[string]interface{}{}})
	accessResp := readControlMessage(t, scanner)
	if accessResp.Type != "server_access" {
		t.Fatalf("server_access response type = %q, want %q", accessResp.Type, "server_access")
	}
	if accessResp.ID != "req-access" {
		t.Fatalf("server_access response id = %q, want %q", accessResp.ID, "req-access")
	}

	accessPayload, ok := accessResp.Payload.(map[string]interface{})
	if !ok {
		t.Fatalf("server_access payload has unexpected type: %T", accessResp.Payload)
	}
	if accessPayload["server_connected"] != false {
		t.Fatalf("server_access server_connected = %#v, want false", accessPayload["server_connected"])
	}
	if got := asString(accessPayload["owner_token"]); got != "" {
		t.Fatalf("server_access owner_token = %q, want empty", got)
	}

	writeControlMessage(t, conn, contractMessage{V: contractVersion, Type: "arm", ID: "req-arm", Payload: map[string]interface{}{"reason": "client_boot"}})
	armResp := readControlMessage(t, scanner)
	if armResp.Type != "arm_ack" {
		t.Fatalf("arm response type = %q, want %q", armResp.Type, "arm_ack")
	}
	if armResp.ID != "req-arm" {
		t.Fatalf("arm response id = %q, want %q", armResp.ID, "req-arm")
	}

	writeControlMessage(t, conn, contractMessage{V: contractVersion, Type: "lifecycle", ID: "req-lifecycle", Payload: map[string]interface{}{}})
	lifecycleResp := readControlMessage(t, scanner)
	if lifecycleResp.Type != "lifecycle" {
		t.Fatalf("lifecycle response type = %q, want %q", lifecycleResp.Type, "lifecycle")
	}
	if lifecycleResp.ID != "req-lifecycle" {
		t.Fatalf("lifecycle response id = %q, want %q", lifecycleResp.ID, "req-lifecycle")
	}
	lifecyclePayload, ok := lifecycleResp.Payload.(map[string]interface{})
	if !ok {
		t.Fatalf("lifecycle payload has unexpected type: %T", lifecycleResp.Payload)
	}
	if lifecyclePayload["armed"] != true {
		t.Fatalf("lifecycle armed = %#v, want true", lifecyclePayload["armed"])
	}

	writeControlMessage(t, conn, contractMessage{V: contractVersion, Type: "server_command", ID: "req-forbidden", Payload: map[string]interface{}{"command": "JOIN 1"}})
	errResp := readControlMessage(t, scanner)
	if errResp.Type != "control_error" {
		t.Fatalf("server_command response type = %q, want %q", errResp.Type, "control_error")
	}
	if errResp.ID != "req-forbidden" {
		t.Fatalf("server_command response id = %q, want %q", errResp.ID, "req-forbidden")
	}

	errPayload, ok := errResp.Payload.(map[string]interface{})
	if !ok {
		t.Fatalf("error payload has unexpected type: %T", errResp.Payload)
	}
	if asString(errPayload["error"]) != "forbidden_type" {
		t.Fatalf("error payload error = %q, want %q", asString(errPayload["error"]), "forbidden_type")
	}

	writeControlMessage(t, conn, contractMessage{V: contractVersion, Type: "disarm", ID: "req-disarm", Payload: map[string]interface{}{"reason": "client_shutdown"}})
	disarmResp := readControlMessage(t, scanner)
	if disarmResp.Type != "disarm_ack" {
		t.Fatalf("disarm response type = %q, want %q", disarmResp.Type, "disarm_ack")
	}
	if disarmResp.ID != "req-disarm" {
		t.Fatalf("disarm response id = %q, want %q", disarmResp.ID, "req-disarm")
	}
}

func TestControlSocketQuit_CancelsAndClosesConnections(t *testing.T) {
	t.Helper()

	sockPath := filepath.Join(t.TempDir(), "bbs-agent-control-quit.sock")

	ag, err := newAgent(runtimeConfig{server: "", name: "agent_bot"})
	if err != nil {
		t.Fatalf("newAgent returned error: %v", err)
	}
	ag.name = "agent_bot"
	defer ag.shutdown("test_done")

	botConn, botPeer := net.Pipe()
	defer botPeer.Close()
	serverConn, serverPeer := net.Pipe()
	defer serverPeer.Close()
	ag.botConn = botConn
	ag.conn = serverConn

	if err := ag.startControlListener("unix://" + sockPath); err != nil {
		t.Fatalf("startControlListener returned error: %v", err)
	}

	conn, err := net.DialTimeout("unix", sockPath, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to connect to control socket: %v", err)
	}
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 1024), maxScannerToken)
	_ = readControlMessage(t, scanner)

	writeControlMessage(t, conn, contractMessage{V: contractVersion, Type: "quit", ID: "req-quit", Payload: map[string]interface{}{"reason": "client_exit"}})
	quitResp := readControlMessage(t, scanner)
	if quitResp.Type != "quit_ack" {
		t.Fatalf("quit response type = %q, want %q", quitResp.Type, "quit_ack")
	}
	if quitResp.ID != "req-quit" {
		t.Fatalf("quit response id = %q, want %q", quitResp.ID, "req-quit")
	}

	select {
	case <-ag.ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("agent context was not canceled after quit")
	}

	assertConnClosed(t, botPeer, "bot peer")
	assertConnClosed(t, serverPeer, "server peer")
}

func TestControlSocketLifecycleSequence_ArmLifecycleQuit(t *testing.T) {
	t.Helper()

	sockPath := filepath.Join(t.TempDir(), "bbs-agent-control-sequence.sock")

	ag, err := newAgent(runtimeConfig{server: "", name: "agent_bot"})
	if err != nil {
		t.Fatalf("newAgent returned error: %v", err)
	}
	ag.name = "agent_bot"
	defer ag.shutdown("test_done")

	if err := ag.startControlListener("unix://" + sockPath); err != nil {
		t.Fatalf("startControlListener returned error: %v", err)
	}

	conn, err := net.DialTimeout("unix", sockPath, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to connect to control socket: %v", err)
	}
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 1024), maxScannerToken)
	_ = readControlMessage(t, scanner)

	writeControlMessage(t, conn, contractMessage{V: contractVersion, Type: "arm", ID: "seq-arm", Payload: map[string]interface{}{"reason": "sequence_boot"}})
	armResp := readControlMessage(t, scanner)
	if armResp.Type != "arm_ack" {
		t.Fatalf("arm response type = %q, want %q", armResp.Type, "arm_ack")
	}
	if armResp.ID != "seq-arm" {
		t.Fatalf("arm response id = %q, want %q", armResp.ID, "seq-arm")
	}

	writeControlMessage(t, conn, contractMessage{V: contractVersion, Type: "lifecycle", ID: "seq-life", Payload: map[string]interface{}{}})
	lifecycleResp := readControlMessage(t, scanner)
	if lifecycleResp.Type != "lifecycle" {
		t.Fatalf("lifecycle response type = %q, want %q", lifecycleResp.Type, "lifecycle")
	}
	if lifecycleResp.ID != "seq-life" {
		t.Fatalf("lifecycle response id = %q, want %q", lifecycleResp.ID, "seq-life")
	}

	lifecyclePayload, ok := lifecycleResp.Payload.(map[string]interface{})
	if !ok {
		t.Fatalf("lifecycle payload has unexpected type: %T", lifecycleResp.Payload)
	}
	if lifecyclePayload["armed"] != true {
		t.Fatalf("lifecycle armed = %#v, want true", lifecyclePayload["armed"])
	}
	if got := asString(lifecyclePayload["reason"]); got != "sequence_boot" {
		t.Fatalf("lifecycle reason = %q, want %q", got, "sequence_boot")
	}

	writeControlMessage(t, conn, contractMessage{V: contractVersion, Type: "quit", ID: "seq-quit", Payload: map[string]interface{}{"reason": "sequence_shutdown"}})
	quitResp := readControlMessage(t, scanner)
	if quitResp.Type != "quit_ack" {
		t.Fatalf("quit response type = %q, want %q", quitResp.Type, "quit_ack")
	}
	if quitResp.ID != "seq-quit" {
		t.Fatalf("quit response id = %q, want %q", quitResp.ID, "seq-quit")
	}

	select {
	case <-ag.ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("agent context was not canceled after lifecycle quit sequence")
	}
}

func assertConnClosed(t *testing.T, conn net.Conn, label string) {
	t.Helper()

	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	buf := make([]byte, 1)
	_, err := conn.Read(buf)
	if err == nil {
		t.Fatalf("%s unexpectedly readable after quit", label)
	}
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		t.Fatalf("%s appears to still be open after quit (read timeout)", label)
	}
}

func writeControlMessage(t *testing.T, conn net.Conn, msg contractMessage) {
	t.Helper()
	payload, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if err := conn.SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetWriteDeadline failed: %v", err)
	}
	if _, err := conn.Write(append(payload, '\n')); err != nil {
		t.Fatalf("control write failed: %v", err)
	}
}

func readControlMessage(t *testing.T, scanner *bufio.Scanner) contractMessage {
	t.Helper()
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			t.Fatalf("control scan failed: %v", err)
		}
		t.Fatal("control socket closed before response")
	}
	line := scanner.Bytes()
	var msg contractMessage
	if err := json.Unmarshal(line, &msg); err != nil {
		t.Fatalf("invalid control response JSON %q: %v", string(line), err)
	}
	return msg
}
