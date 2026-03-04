package nanoclaw

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Message represents a message from nanoclaw's SQLite database.
type Message struct {
	ID           string `json:"id"`
	ChatJID      string `json:"chat_jid"`
	Sender       string `json:"sender"`
	SenderName   string `json:"sender_name"`
	Content      string `json:"content"`
	Timestamp    string `json:"timestamp"`
	IsFromMe     int    `json:"is_from_me"`
	IsBotMessage int    `json:"is_bot_message"`
}

// Group represents a registered nanoclaw group.
type Group struct {
	JID    string `json:"jid"`
	Name   string `json:"name"`
	Folder string `json:"folder"`
	IsMain int    `json:"is_main"`
}

// Bridge communicates with a running nanoclaw instance.
type Bridge struct {
	// NanoClawDir is the root directory of the nanoclaw installation (e.g. ~/nanoclaw).
	NanoClawDir string
}

// NewBridge creates a new Bridge pointing at the given nanoclaw directory.
// If dir is empty, it defaults to ~/nanoclaw.
func NewBridge(dir string) *Bridge {
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, "nanoclaw")
	}
	return &Bridge{NanoClawDir: dir}
}

func (b *Bridge) dataDir() string  { return filepath.Join(b.NanoClawDir, "data") }
func (b *Bridge) storeDir() string { return filepath.Join(b.NanoClawDir, "store") }
func (b *Bridge) dbPath() string   { return filepath.Join(b.storeDir(), "messages.db") }

// Available returns true if the nanoclaw installation looks valid (DB exists).
func (b *Bridge) Available() bool {
	_, err := os.Stat(b.dbPath())
	return err == nil
}

// nodeQuery runs a JavaScript snippet using nanoclaw's better-sqlite3 and returns stdout.
func (b *Bridge) nodeQuery(script string) ([]byte, error) {
	// Use nanoclaw's node_modules for better-sqlite3
	cmd := exec.Command("node", "-e", script)
	cmd.Dir = b.NanoClawDir
	cmd.Env = append(os.Environ(), "NODE_PATH="+filepath.Join(b.NanoClawDir, "node_modules"))
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("node query failed: %s", string(exitErr.Stderr))
		}
		return nil, err
	}
	return out, nil
}

// ListGroups returns all registered groups.
func (b *Bridge) ListGroups() ([]Group, error) {
	script := fmt.Sprintf(`
const Database = require('better-sqlite3');
const db = new Database(%q);
const rows = db.prepare('SELECT jid, name, folder, is_main FROM registered_groups ORDER BY name').all();
console.log(JSON.stringify(rows));
db.close();
`, b.dbPath())

	out, err := b.nodeQuery(script)
	if err != nil {
		return nil, err
	}
	var groups []Group
	if err := json.Unmarshal(out, &groups); err != nil {
		return nil, fmt.Errorf("failed to parse groups: %w", err)
	}
	return groups, nil
}

// GetRecentMessages returns recent messages, newest first.
func (b *Bridge) GetRecentMessages(limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 50
	}
	script := fmt.Sprintf(`
const Database = require('better-sqlite3');
const db = new Database(%q);
const rows = db.prepare('SELECT id, chat_jid, sender, sender_name, content, timestamp, is_from_me, is_bot_message FROM messages ORDER BY timestamp DESC LIMIT ?').all(%d);
console.log(JSON.stringify(rows));
db.close();
`, b.dbPath(), limit)

	out, err := b.nodeQuery(script)
	if err != nil {
		return nil, err
	}
	var msgs []Message
	if err := json.Unmarshal(out, &msgs); err != nil {
		return nil, fmt.Errorf("failed to parse messages: %w", err)
	}
	// Reverse so oldest is first
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil
}

// GetMessagesForGroup returns recent messages for a specific group folder.
func (b *Bridge) GetMessagesForGroup(groupFolder string, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 50
	}
	script := fmt.Sprintf(`
const Database = require('better-sqlite3');
const db = new Database(%q);
const group = db.prepare('SELECT jid FROM registered_groups WHERE folder = ?').get(%q);
if (!group) { console.log('[]'); db.close(); process.exit(0); }
const rows = db.prepare('SELECT id, chat_jid, sender, sender_name, content, timestamp, is_from_me, is_bot_message FROM messages WHERE chat_jid = ? ORDER BY timestamp DESC LIMIT ?').all(group.jid, %d);
console.log(JSON.stringify(rows));
db.close();
`, b.dbPath(), groupFolder, limit)

	out, err := b.nodeQuery(script)
	if err != nil {
		return nil, err
	}
	var msgs []Message
	if err := json.Unmarshal(out, &msgs); err != nil {
		return nil, fmt.Errorf("failed to parse messages: %w", err)
	}
	// Reverse so oldest is first
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil
}

// MessageMeta contains metadata attached to messages sent from claude-squad.
type MessageMeta struct {
	RepoPath string `json:"repo_path,omitempty"`
	RepoID   string `json:"repo_id,omitempty"`
	Branch   string `json:"branch,omitempty"`
	Program  string `json:"program,omitempty"`
}

// SendMessage sends a message to a nanoclaw group.
// It writes an IPC input file (for active containers) AND inserts into the
// SQLite database (so the nanoclaw message loop picks it up if no container
// is running). Optional metadata is included in the IPC file and prepended
// as context to the DB message.
func (b *Bridge) SendMessage(groupFolder string, text string, meta *MessageMeta) error {
	// Build IPC payload with metadata
	ipcPayload := map[string]interface{}{
		"type": "message",
		"text": text,
	}
	if meta != nil {
		ipcPayload["meta"] = meta
	}

	// Write IPC input file for active container
	ipcInputDir := filepath.Join(b.dataDir(), "ipc", groupFolder, "input")
	if err := os.MkdirAll(ipcInputDir, 0755); err != nil {
		return fmt.Errorf("failed to create IPC input dir: %w", err)
	}

	filename := fmt.Sprintf("%d-cs-bridge-%s.json",
		time.Now().UnixMilli(),
		randomString(6))

	data, err := json.Marshal(ipcPayload)
	if err != nil {
		return err
	}

	filePath := filepath.Join(ipcInputDir, filename)
	tmpPath := filePath + ".tmp"

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write IPC file: %w", err)
	}
	if err := os.Rename(tmpPath, filePath); err != nil {
		return fmt.Errorf("failed to rename IPC file: %w", err)
	}

	// Build the DB content with metadata context
	dbContent := text
	if meta != nil {
		var parts []string
		if meta.RepoPath != "" {
			parts = append(parts, fmt.Sprintf("repo: %s", meta.RepoPath))
		}
		if meta.Branch != "" {
			parts = append(parts, fmt.Sprintf("branch: %s", meta.Branch))
		}
		if meta.Program != "" {
			parts = append(parts, fmt.Sprintf("program: %s", meta.Program))
		}
		if len(parts) > 0 {
			dbContent = fmt.Sprintf("[%s]\n%s", strings.Join(parts, " | "), text)
		}
	}

	// Also insert into SQLite so the message loop picks it up
	msgID := fmt.Sprintf("cs-bridge-%d-%s", time.Now().UnixMilli(), randomString(6))
	script := fmt.Sprintf(`
const Database = require('better-sqlite3');
const db = new Database(%q);
const group = db.prepare('SELECT jid, name FROM registered_groups WHERE folder = ?').get(%q);
if (!group) { console.error('Group not found'); process.exit(1); }
const msgId = %q;
const now = new Date().toISOString();
db.prepare(
  'INSERT OR REPLACE INTO messages (id, chat_jid, sender, sender_name, content, timestamp, is_from_me, is_bot_message) VALUES (?, ?, ?, ?, ?, ?, ?, ?)'
).run(msgId, group.jid, 'cs-bridge', 'Claude Squad', %q, now, 0, 0);
db.prepare(
  'INSERT OR REPLACE INTO chats (jid, name, last_message_time) VALUES (?, ?, ?)'
).run(group.jid, group.name, now);
db.close();
`, b.dbPath(), groupFolder, msgID, dbContent)

	if _, err := b.nodeQuery(script); err != nil {
		return fmt.Errorf("failed to insert message into DB: %w", err)
	}

	return nil
}

// Status returns a summary of the nanoclaw instance.
func (b *Bridge) Status() (string, error) {
	script := fmt.Sprintf(`
const Database = require('better-sqlite3');
const db = new Database(%q);
const groups = db.prepare('SELECT COUNT(*) as count FROM registered_groups').get();
const messages = db.prepare('SELECT COUNT(*) as count FROM messages').get();
let tasks = { count: 0 };
try { tasks = db.prepare("SELECT COUNT(*) as count FROM scheduled_tasks WHERE status = 'active'").get(); } catch(e) {}
console.log(JSON.stringify({ groups: groups.count, messages: messages.count, tasks: tasks.count }));
db.close();
`, b.dbPath())

	out, err := b.nodeQuery(script)
	if err != nil {
		return "", err
	}
	out = []byte(strings.TrimSpace(string(out)))

	var status struct {
		Groups   int `json:"groups"`
		Messages int `json:"messages"`
		Tasks    int `json:"tasks"`
	}
	if err := json.Unmarshal(out, &status); err != nil {
		return "", err
	}

	return fmt.Sprintf("Groups: %d | Messages: %d | Active tasks: %d",
		status.Groups, status.Messages, status.Tasks), nil
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
