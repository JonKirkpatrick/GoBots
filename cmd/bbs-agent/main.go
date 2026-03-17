package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	contractVersion = "0.2"
	agentVersion    = "0.2.0"
)

type repeatedStringFlag []string

func (r *repeatedStringFlag) String() string {
	return strings.Join(*r, ",")
}

func (r *repeatedStringFlag) Set(value string) error {
	*r = append(*r, value)
	return nil
}

type credentials struct {
	BotID     string
	BotSecret string
}

type contractMessage struct {
	V       string      `json:"v"`
	Type    string      `json:"type"`
	ID      string      `json:"id,omitempty"`
	Payload interface{} `json:"payload"`
}

type serverMessage struct {
	Status  string      `json:"status"`
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type workerEnvelope struct {
	V       string          `json:"v"`
	Type    string          `json:"type"`
	ID      string          `json:"id,omitempty"`
	Payload json.RawMessage `json:"payload"`
}

type agent struct {
	ctx    context.Context
	cancel context.CancelFunc

	name         string
	server       string
	ownerToken   string
	capabilities string

	workerCmd    *exec.Cmd
	workerStdin  io.WriteCloser
	workerStdout io.ReadCloser
	workerStderr io.ReadCloser
	workerMu     sync.Mutex

	conn     net.Conn
	serverMu sync.Mutex

	registerCh      chan serverMessage
	serverErrCh     chan error
	workerReadErrCh chan error

	sessionID int

	joinedArenaID  int
	joinedPlayerID int
	joinedGame     string
	joinedTimeMS   int
	joinedMoveMS   int
	turnStep       int

	lastStatePayload map[string]interface{}
	pendingResponse  map[string]interface{}
}

func main() {
	os.Exit(run())
}

func run() int {
	cfg, err := parseFlags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[agent] %v\n", err)
		return 1
	}

	creds, err := loadCredentials(cfg.credentialsFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[agent] failed to load credentials: %v\n", err)
		return 1
	}

	ag, err := newAgent(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[agent] failed to initialize: %v\n", err)
		return 1
	}
	defer ag.shutdown("agent_exit")

	if err := ag.startWorker(); err != nil {
		fmt.Fprintf(os.Stderr, "[agent] failed to start worker: %v\n", err)
		return 1
	}

	if err := ag.connectServer(); err != nil {
		fmt.Fprintf(os.Stderr, "[agent] server connect failed: %v\n", err)
		return 1
	}

	registerCommand := buildRegisterCommand(cfg.name, creds, cfg.capabilities, cfg.ownerToken)
	fmt.Fprintf(os.Stderr, "[agent] sending REGISTER to %s\n", cfg.server)
	if err := ag.sendServerCommand(registerCommand); err != nil {
		fmt.Fprintf(os.Stderr, "[agent] failed to send register command: %v\n", err)
		return 1
	}

	registerMsg, err := waitForRegister(ag.registerCh, cfg.registerTimeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[agent] register failed: %v\n", err)
		return 1
	}

	if strings.ToLower(strings.TrimSpace(registerMsg.Status)) != "ok" {
		fmt.Fprintf(os.Stderr, "[agent] register rejected: %v\n", registerMsg.Payload)
		return 1
	}

	registerPayload, _ := registerMsg.Payload.(map[string]interface{})
	ag.sessionID = asInt(registerPayload["session_id"])
	if registerPayload != nil {
		botID := asString(registerPayload["bot_id"])
		botSecret := asString(registerPayload["bot_secret"])
		if botID != "" && botSecret != "" {
			if err := saveCredentials(cfg.credentialsFile, credentials{BotID: botID, BotSecret: botSecret}); err != nil {
				fmt.Fprintf(os.Stderr, "[agent] warning: failed to save credentials: %v\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "[agent] saved credentials to %s\n", cfg.credentialsFile)
			}
		}
	}

	fmt.Fprintln(os.Stderr, "[agent] registered; waiting for JOIN to send worker welcome")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case sig := <-sigCh:
		fmt.Fprintf(os.Stderr, "[agent] signal received: %s\n", sig)
	case err := <-ag.serverErrCh:
		if err != nil && !errors.Is(err, io.EOF) {
			fmt.Fprintf(os.Stderr, "[agent] server reader stopped: %v\n", err)
		}
	case err := <-ag.workerReadErrCh:
		if err != nil && !errors.Is(err, io.EOF) {
			fmt.Fprintf(os.Stderr, "[agent] worker reader stopped: %v\n", err)
		}
	}

	return 0
}

type runtimeConfig struct {
	server          string
	name            string
	ownerToken      string
	capabilities    string
	credentialsFile string
	workerBin       string
	workerArgs      []string
	registerTimeout time.Duration
}

func parseFlags() (runtimeConfig, error) {
	var cfg runtimeConfig
	var workerArgs repeatedStringFlag

	flag.StringVar(&cfg.server, "server", "", "BBS server endpoint in host:port format")
	flag.StringVar(&cfg.name, "name", "agent_bot", "bot display name used during REGISTER")
	flag.StringVar(&cfg.ownerToken, "owner-token", "", "optional owner token from dashboard")
	flag.StringVar(&cfg.capabilities, "capabilities", "connect4", "comma-separated capability list")
	flag.StringVar(&cfg.credentialsFile, "credentials-file", "", "path to bot credentials file (key=value format)")
	flag.StringVar(&cfg.workerBin, "worker", "", "worker executable path (required)")
	flag.Var(&workerArgs, "worker-arg", "argument to pass to worker process (repeat flag for multiple args)")
	flag.DurationVar(&cfg.registerTimeout, "register-timeout", 12*time.Second, "server register response timeout")

	flag.Parse()

	if _, _, err := parseServerAddress(cfg.server); err != nil {
		return cfg, err
	}
	if strings.TrimSpace(cfg.workerBin) == "" {
		return cfg, errors.New("--worker is required")
	}
	if strings.Contains(cfg.name, " ") {
		return cfg, errors.New("--name cannot contain spaces")
	}
	if cfg.credentialsFile == "" {
		cfg.credentialsFile = defaultCredentialsFilePath(cfg.name)
	}
	cfg.credentialsFile = strings.TrimSpace(cfg.credentialsFile)
	cfg.workerArgs = append([]string(nil), workerArgs...)

	return cfg, nil
}

func newAgent(cfg runtimeConfig) (*agent, error) {
	ctx, cancel := context.WithCancel(context.Background())

	cmd := exec.CommandContext(ctx, cfg.workerBin, cfg.workerArgs...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, err
	}

	return &agent{
		ctx:             ctx,
		cancel:          cancel,
		name:            cfg.name,
		server:          cfg.server,
		ownerToken:      strings.TrimSpace(cfg.ownerToken),
		capabilities:    strings.TrimSpace(cfg.capabilities),
		workerCmd:       cmd,
		workerStdin:     stdin,
		workerStdout:    stdout,
		workerStderr:    stderr,
		registerCh:      make(chan serverMessage, 1),
		serverErrCh:     make(chan error, 1),
		workerReadErrCh: make(chan error, 1),
	}, nil
}

func (a *agent) startWorker() error {
	if err := a.workerCmd.Start(); err != nil {
		return err
	}

	go a.readWorkerStdout()
	go a.streamWorkerStderr()

	go func() {
		err := a.workerCmd.Wait()
		select {
		case a.workerReadErrCh <- err:
		default:
		}
	}()

	return nil
}

func (a *agent) connectServer() error {
	conn, err := net.Dial("tcp", a.server)
	if err != nil {
		return err
	}
	a.conn = conn

	go a.readServer()
	return nil
}

func waitForRegister(ch <-chan serverMessage, timeout time.Duration) (serverMessage, error) {
	select {
	case msg := <-ch:
		return msg, nil
	case <-time.After(timeout):
		return serverMessage{}, fmt.Errorf("timeout waiting for register response after %s", timeout)
	}
}

func (a *agent) readWorkerStdout() {
	scanner := bufio.NewScanner(a.workerStdout)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		a.handleWorkerLine(line)
	}

	err := scanner.Err()
	select {
	case a.workerReadErrCh <- err:
	default:
	}
}

func (a *agent) streamWorkerStderr() {
	scanner := bufio.NewScanner(a.workerStderr)
	for scanner.Scan() {
		fmt.Fprintf(os.Stderr, "[worker] %s\n", scanner.Text())
	}
}

func (a *agent) handleWorkerLine(line string) {
	var env workerEnvelope
	if err := json.Unmarshal([]byte(line), &env); err != nil {
		fmt.Fprintf(os.Stderr, "[agent] invalid worker JSON: %s\n", line)
		return
	}

	if env.V != contractVersion {
		fmt.Fprintf(os.Stderr, "[agent] ignoring worker message with unsupported version: %s\n", env.V)
		return
	}

	typeName := strings.ToLower(strings.TrimSpace(env.Type))
	switch typeName {
	case "action":
		var payload struct {
			Action string `json:"action"`
		}
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			fmt.Fprintf(os.Stderr, "[agent] worker action payload parse error: %v\n", err)
			return
		}
		action := strings.TrimSpace(payload.Action)
		if action == "" {
			return
		}
		action = strings.ReplaceAll(action, "\n", "")
		action = strings.ReplaceAll(action, "\r", "")
		_ = a.sendServerCommand("MOVE " + action)
	case "log":
		var payload struct {
			Level   string `json:"level"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			fmt.Fprintf(os.Stderr, "[agent] worker log payload parse error: %v\n", err)
			return
		}
		fmt.Fprintf(os.Stderr, "[worker:%s] %s\n", strings.TrimSpace(payload.Level), strings.TrimSpace(payload.Message))
	default:
		fmt.Fprintf(os.Stderr, "[agent] ignoring unsupported worker message type=%s (expected action/log)\n", env.Type)
	}
}

func (a *agent) readServer() {
	scanner := bufio.NewScanner(a.conn)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		a.handleServerLine(line)
	}

	err := scanner.Err()
	select {
	case a.serverErrCh <- err:
	default:
	}
}

func (a *agent) handleServerLine(line string) {
	var msg serverMessage
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		fmt.Fprintf(os.Stderr, "[agent] server text: %s\n", line)
		return
	}

	msgType := strings.ToLower(strings.TrimSpace(msg.Type))
	status := strings.ToLower(strings.TrimSpace(msg.Status))

	if msgType == "register" {
		select {
		case a.registerCh <- msg:
		default:
		}
		return
	}

	switch msgType {
	case "join":
		if payloadMap, ok := msg.Payload.(map[string]interface{}); ok {
			a.joinedArenaID = asInt(payloadMap["arena_id"])
			a.joinedPlayerID = asInt(payloadMap["player_id"])
			a.joinedGame = asString(payloadMap["game"])
			a.joinedTimeMS = asInt(payloadMap["time_limit_ms"])
			a.joinedMoveMS = asInt(payloadMap["effective_time_limit_ms"])
			if a.joinedMoveMS <= 0 {
				a.joinedMoveMS = a.joinedTimeMS
			}
			a.turnStep = 0
			a.pendingResponse = nil
			a.lastStatePayload = nil

			welcome := map[string]interface{}{
				"agent_name":              "bbs-agent",
				"agent_version":           agentVersion,
				"server":                  a.server,
				"session_id":              a.sessionID,
				"arena_id":                a.joinedArenaID,
				"player_id":               a.joinedPlayerID,
				"env":                     a.joinedGame,
				"time_limit_ms":           a.joinedTimeMS,
				"effective_time_limit_ms": a.joinedMoveMS,
				"capabilities":            splitCapabilities(a.capabilities),
			}
			_ = a.sendWorker(contractMessage{V: contractVersion, Type: "welcome", Payload: welcome})
		}
	case "data":
		statePayload := buildStatePayload(msg.Payload, a.joinedPlayerID)
		a.lastStatePayload = statePayload
		if !shouldForwardTurn(statePayload, a.joinedPlayerID) {
			return
		}

		a.turnStep++
		turnPayload := map[string]interface{}{
			"step":        a.turnStep,
			"deadline_ms": a.joinedMoveMS,
			"obs":         statePayload,
			"reward":      0.0,
			"done":        false,
			"truncated":   false,
		}
		if a.pendingResponse != nil {
			turnPayload["response"] = a.pendingResponse
		}
		a.pendingResponse = nil
		_ = a.sendWorker(contractMessage{V: contractVersion, Type: "turn", Payload: turnPayload})
	case "move", "error", "timeout", "ejected":
		a.pendingResponse = map[string]interface{}{
			"type":    msgType,
			"status":  status,
			"payload": msg.Payload,
		}

		if a.lastStatePayload != nil && shouldForwardTurn(a.lastStatePayload, a.joinedPlayerID) && status == "err" {
			a.turnStep++
			turnPayload := map[string]interface{}{
				"step":        a.turnStep,
				"deadline_ms": a.joinedMoveMS,
				"obs":         a.lastStatePayload,
				"reward":      0.0,
				"done":        false,
				"truncated":   false,
				"response":    a.pendingResponse,
			}
			a.pendingResponse = nil
			_ = a.sendWorker(contractMessage{V: contractVersion, Type: "turn", Payload: turnPayload})
		}

		if msgType == "timeout" || msgType == "ejected" {
			a.turnStep++
			terminalPayload := map[string]interface{}{
				"step":      a.turnStep,
				"reward":    0.0,
				"done":      true,
				"truncated": true,
				"response":  a.pendingResponse,
			}
			if a.lastStatePayload != nil {
				terminalPayload["obs"] = a.lastStatePayload
			}
			a.pendingResponse = nil
			_ = a.sendWorker(contractMessage{V: contractVersion, Type: "turn", Payload: terminalPayload})
		}
	case "gameover":
		a.turnStep++
		terminalPayload := map[string]interface{}{
			"step":      a.turnStep,
			"reward":    0.0,
			"done":      true,
			"truncated": false,
			"response": map[string]interface{}{
				"type":    msgType,
				"status":  status,
				"payload": msg.Payload,
			},
		}
		if a.lastStatePayload != nil {
			terminalPayload["obs"] = a.lastStatePayload
		}
		a.pendingResponse = nil
		_ = a.sendWorker(contractMessage{V: contractVersion, Type: "turn", Payload: terminalPayload})
	default:
		if status == "err" {
			fmt.Fprintf(os.Stderr, "[agent] server err type=%s payload=%v\n", msgType, msg.Payload)
		}
	}
}

func (a *agent) sendWorker(msg contractMessage) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	a.workerMu.Lock()
	defer a.workerMu.Unlock()

	_, err = a.workerStdin.Write(append(payload, '\n'))
	return err
}

func (a *agent) sendServerCommand(command string) error {
	if a.conn == nil {
		return errors.New("server connection is not established")
	}
	line := strings.TrimSpace(command)
	if line == "" {
		return nil
	}

	a.serverMu.Lock()
	defer a.serverMu.Unlock()

	_, err := a.conn.Write([]byte(line + "\n"))
	return err
}

func (a *agent) shutdown(reason string) {
	a.cancel()

	_ = a.sendWorker(contractMessage{
		V:    contractVersion,
		Type: "shutdown",
		Payload: map[string]interface{}{
			"reason": reason,
		},
	})

	_ = a.sendServerCommand("QUIT")

	if a.conn != nil {
		_ = a.conn.Close()
	}
	if a.workerStdin != nil {
		_ = a.workerStdin.Close()
	}
}

func parseServerAddress(raw string) (string, int, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", 0, errors.New("--server is required")
	}

	host, portRaw, err := net.SplitHostPort(value)
	if err != nil {
		return "", 0, fmt.Errorf("invalid --server %q; expected host:port", raw)
	}
	if strings.TrimSpace(host) == "" || strings.TrimSpace(portRaw) == "" {
		return "", 0, fmt.Errorf("invalid --server %q; expected host:port", raw)
	}

	var port int
	if _, err := fmt.Sscanf(portRaw, "%d", &port); err != nil || port <= 0 || port > 65535 {
		return "", 0, fmt.Errorf("invalid --server port in %q", raw)
	}

	return host, port, nil
}

func buildRegisterCommand(name string, creds credentials, capabilitiesCSV, ownerToken string) string {
	parts := []string{"REGISTER", strings.TrimSpace(name)}

	if strings.TrimSpace(creds.BotID) != "" {
		parts = append(parts, strings.TrimSpace(creds.BotID))
	} else {
		parts = append(parts, "\"\"")
	}

	if strings.TrimSpace(creds.BotSecret) != "" {
		parts = append(parts, strings.TrimSpace(creds.BotSecret))
	} else {
		parts = append(parts, "\"\"")
	}

	caps := strings.TrimSpace(capabilitiesCSV)
	if caps != "" {
		parts = append(parts, caps)
	}

	ownerToken = strings.TrimSpace(ownerToken)
	if ownerToken != "" {
		parts = append(parts, "owner_token="+ownerToken)
	}

	return strings.Join(parts, " ")
}

func splitCapabilities(csv string) []string {
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		capability := strings.TrimSpace(part)
		if capability == "" {
			continue
		}
		out = append(out, capability)
	}
	return out
}

func shouldForwardTurn(statePayload map[string]interface{}, joinedPlayerID int) bool {
	if joinedPlayerID <= 0 {
		return true
	}
	if raw, ok := statePayload["your_turn"]; ok {
		return asBool(raw)
	}
	turnPlayer := asInt(statePayload["turn_player"])
	if turnPlayer > 0 {
		return turnPlayer == joinedPlayerID
	}
	return true
}

func loadCredentials(path string) (credentials, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return credentials{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return credentials{}, nil
		}
		return credentials{}, err
	}

	var out credentials
	for _, line := range strings.Split(string(data), "\n") {
		entry := strings.TrimSpace(line)
		if entry == "" || strings.HasPrefix(entry, "#") {
			continue
		}
		if !strings.Contains(entry, "=") {
			continue
		}
		parts := strings.SplitN(entry, "=", 2)
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		switch key {
		case "bot_id":
			out.BotID = value
		case "bot_secret":
			out.BotSecret = value
		}
	}

	return out, nil
}

func saveCredentials(path string, creds credentials) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("credentials path is empty")
	}

	content := strings.Builder{}
	content.WriteString("# Build-a-Bot Stadium bot credentials\n")
	content.WriteString("bot_id=" + strings.TrimSpace(creds.BotID) + "\n")
	content.WriteString("bot_secret=" + strings.TrimSpace(creds.BotSecret) + "\n")

	return os.WriteFile(path, []byte(content.String()), 0o600)
}

var sanitizeNameRE = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func defaultCredentialsFilePath(name string) string {
	clean := sanitizeNameRE.ReplaceAllString(strings.TrimSpace(name), "_")
	clean = strings.Trim(clean, "_")
	if clean == "" {
		clean = "agent_bot"
	}
	return clean + "_credentials.txt"
}

func asString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return strings.TrimSpace(val)
	default:
		if v == nil {
			return ""
		}
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}

func asBool(v interface{}) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		lower := strings.ToLower(strings.TrimSpace(val))
		return lower == "true" || lower == "1" || lower == "yes"
	default:
		return false
	}
}

func asNumber(v interface{}) interface{} {
	switch val := v.(type) {
	case float64:
		if float64(int64(val)) == val {
			return int64(val)
		}
		return val
	case int, int32, int64, uint, uint32, uint64:
		return val
	default:
		return v
	}
}

func asInt(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case int32:
		return int(val)
	case int64:
		return int(val)
	case float64:
		return int(val)
	case json.Number:
		i, err := val.Int64()
		if err == nil {
			return int(i)
		}
		f, ferr := val.Float64()
		if ferr == nil {
			return int(f)
		}
		return 0
	case string:
		trimmed := strings.TrimSpace(val)
		if trimmed == "" {
			return 0
		}
		var parsed int
		if _, err := fmt.Sscanf(trimmed, "%d", &parsed); err == nil {
			return parsed
		}
		return 0
	default:
		return 0
	}
}

func buildStatePayload(raw interface{}, joinedPlayerID int) map[string]interface{} {
	payload := map[string]interface{}{
		"source": "server_data",
	}

	rawState := ""
	var stateObj map[string]interface{}

	switch val := raw.(type) {
	case string:
		rawState = strings.TrimSpace(val)
		if rawState != "" {
			var parsed interface{}
			if err := json.Unmarshal([]byte(rawState), &parsed); err == nil {
				if obj, ok := parsed.(map[string]interface{}); ok {
					stateObj = obj
				}
			}
		}
	case map[string]interface{}:
		stateObj = val
		encoded, err := json.Marshal(val)
		if err == nil {
			rawState = string(encoded)
		}
	default:
		encoded, err := json.Marshal(raw)
		if err == nil {
			rawState = string(encoded)
		}
	}

	if rawState != "" {
		payload["raw_state"] = rawState
	} else {
		payload["raw_state"] = raw
	}

	if stateObj != nil {
		payload["state_obj"] = stateObj

		turnPlayer := asInt(stateObj["turn_player"])
		if turnPlayer == 0 {
			turnPlayer = asInt(stateObj["turn"])
		}
		if turnPlayer > 0 {
			payload["turn_player"] = turnPlayer
			if joinedPlayerID > 0 {
				payload["your_turn"] = turnPlayer == joinedPlayerID
			}
		}
	}

	return payload
}
