package audit

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/phildougherty/mcp-compose/internal/logging"
)

func TestNewFileStorage(t *testing.T) {
	logger := logging.NewLogger("debug")
	tempDir := t.TempDir()

	tests := []struct {
		name    string
		config  FileStorageConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: FileStorageConfig{
				LogDir:      tempDir,
				MaxFileSize: 1024 * 1024,
				MaxAge:      24 * time.Hour,
				Enabled:     true,
			},
			wantErr: false,
		},
		{
			name: "disabled storage",
			config: FileStorageConfig{
				Enabled: false,
			},
			wantErr: false,
		},
		{
			name: "missing log dir",
			config: FileStorageConfig{
				Enabled: true,
			},
			wantErr: true,
		},
		{
			name: "default values",
			config: FileStorageConfig{
				LogDir:  tempDir,
				Enabled: true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs, err := NewFileStorage(tt.config, logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewFileStorage() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if err == nil && fs != nil {
				defer fs.Close()

				if tt.config.Enabled && fs == nil {
					t.Error("Expected non-nil file storage for enabled config")
				}
			}
		})
	}
}

func TestFileStorage_Write(t *testing.T) {
	logger := logging.NewLogger("debug")
	tempDir := t.TempDir()

	config := FileStorageConfig{
		LogDir:      tempDir,
		MaxFileSize: 1024 * 1024,
		MaxAge:      24 * time.Hour,
		Enabled:     true,
	}

	fs, err := NewFileStorage(config, logger)
	if err != nil {
		t.Fatalf("NewFileStorage() error = %v", err)
	}
	defer fs.Close()

	tests := []struct {
		name    string
		entry   *AuditEntry
		wantErr bool
	}{
		{
			name: "valid entry",
			entry: &AuditEntry{
				ID:        "test-1",
				Timestamp: time.Now(),
				Event:     "test.event",
				UserID:    "user1",
				ClientID:  "client1",
				Success:   true,
			},
			wantErr: false,
		},
		{
			name:    "nil entry",
			entry:   nil,
			wantErr: true,
		},
		{
			name: "entry with details",
			entry: &AuditEntry{
				ID:        "test-2",
				Timestamp: time.Now(),
				Event:     "test.event",
				Details: map[string]interface{}{
					"key": "value",
				},
				Success: true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fs.Write(tt.entry)
			if (err != nil) != tt.wantErr {
				t.Errorf("Write() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFileStorage_WriteBatch(t *testing.T) {
	logger := logging.NewLogger("debug")
	tempDir := t.TempDir()

	config := FileStorageConfig{
		LogDir:      tempDir,
		MaxFileSize: 1024 * 1024,
		MaxAge:      24 * time.Hour,
		Enabled:     true,
	}

	fs, err := NewFileStorage(config, logger)
	if err != nil {
		t.Fatalf("NewFileStorage() error = %v", err)
	}
	defer fs.Close()

	entries := []*AuditEntry{
		{
			ID:        "batch-1",
			Timestamp: time.Now(),
			Event:     "test.event",
			Success:   true,
		},
		{
			ID:        "batch-2",
			Timestamp: time.Now(),
			Event:     "test.event",
			Success:   false,
		},
	}

	err = fs.WriteBatch(entries)
	if err != nil {
		t.Errorf("WriteBatch() error = %v", err)
	}

	err = fs.WriteBatch(nil)
	if err != nil {
		t.Errorf("WriteBatch() with nil should not error, got = %v", err)
	}

	err = fs.WriteBatch([]*AuditEntry{})
	if err != nil {
		t.Errorf("WriteBatch() with empty slice should not error, got = %v", err)
	}
}

func TestFileStorage_Flush(t *testing.T) {
	logger := logging.NewLogger("debug")
	tempDir := t.TempDir()

	config := FileStorageConfig{
		LogDir:      tempDir,
		MaxFileSize: 1024 * 1024,
		MaxAge:      24 * time.Hour,
		Enabled:     true,
	}

	fs, err := NewFileStorage(config, logger)
	if err != nil {
		t.Fatalf("NewFileStorage() error = %v", err)
	}
	defer fs.Close()

	entry := &AuditEntry{
		ID:        "flush-test",
		Timestamp: time.Now(),
		Event:     "test.event",
		Success:   true,
	}

	if err := fs.Write(entry); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if err := fs.Flush(); err != nil {
		t.Errorf("Flush() error = %v", err)
	}
}

func TestFileStorage_ReadEntries(t *testing.T) {
	logger := logging.NewLogger("debug")
	tempDir := t.TempDir()

	config := FileStorageConfig{
		LogDir:      tempDir,
		MaxFileSize: 1024 * 1024,
		MaxAge:      24 * time.Hour,
		Enabled:     true,
	}

	fs, err := NewFileStorage(config, logger)
	if err != nil {
		t.Fatalf("NewFileStorage() error = %v", err)
	}
	defer fs.Close()

	entries := []*AuditEntry{
		{
			ID:        "read-1",
			Timestamp: time.Now(),
			Event:     "test.event",
			UserID:    "user1",
			Success:   true,
		},
		{
			ID:        "read-2",
			Timestamp: time.Now(),
			Event:     "test.event",
			UserID:    "user2",
			Success:   false,
		},
	}

	for _, entry := range entries {
		if err := fs.Write(entry); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}

	if err := fs.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	readEntries, total, err := fs.ReadEntries(10, 0, nil)
	if err != nil {
		t.Errorf("ReadEntries() error = %v", err)
	}

	if total < len(entries) {
		t.Errorf("ReadEntries() total = %d, want >= %d", total, len(entries))
	}

	if len(readEntries) < len(entries) {
		t.Errorf("ReadEntries() got %d entries, want >= %d", len(readEntries), len(entries))
	}
}

func TestFileStorage_ReadEntriesWithFilter(t *testing.T) {
	logger := logging.NewLogger("debug")
	tempDir := t.TempDir()

	config := FileStorageConfig{
		LogDir:      tempDir,
		MaxFileSize: 1024 * 1024,
		MaxAge:      24 * time.Hour,
		Enabled:     true,
	}

	fs, err := NewFileStorage(config, logger)
	if err != nil {
		t.Fatalf("NewFileStorage() error = %v", err)
	}
	defer fs.Close()

	entries := []*AuditEntry{
		{
			ID:        "filter-1",
			Timestamp: time.Now(),
			Event:     "test.event",
			UserID:    "user1",
			Success:   true,
		},
		{
			ID:        "filter-2",
			Timestamp: time.Now(),
			Event:     "test.other",
			UserID:    "user2",
			Success:   false,
		},
	}

	for _, entry := range entries {
		if err := fs.Write(entry); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}

	if err := fs.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	filter := &AuditFilter{
		Event: "test.event",
	}

	readEntries, _, err := fs.ReadEntries(10, 0, filter)
	if err != nil {
		t.Errorf("ReadEntries() error = %v", err)
	}

	for _, entry := range readEntries {
		if entry.Event != "test.event" {
			t.Errorf("ReadEntries() with filter returned wrong event: %s", entry.Event)
		}
	}
}

func TestFileStorage_Rotation(t *testing.T) {
	logger := logging.NewLogger("debug")
	tempDir := t.TempDir()

	config := FileStorageConfig{
		LogDir:      tempDir,
		MaxFileSize: 100,
		MaxAge:      24 * time.Hour,
		Enabled:     true,
	}

	fs, err := NewFileStorage(config, logger)
	if err != nil {
		t.Fatalf("NewFileStorage() error = %v", err)
	}
	defer fs.Close()

	for i := 0; i < 10; i++ {
		entry := &AuditEntry{
			ID:        generateAuditID(),
			Timestamp: time.Now(),
			Event:     "test.event.with.long.name.to.trigger.rotation",
			UserID:    "user1",
			Success:   true,
		}

		if err := fs.Write(entry); err != nil {
			t.Errorf("Write() error = %v", err)
		}
	}

	if err := fs.Flush(); err != nil {
		t.Errorf("Flush() error = %v", err)
	}

	files, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}

	if len(files) < 1 {
		t.Error("Expected at least one log file after rotation")
	}
}

func TestFileStorage_Compression(t *testing.T) {
	logger := logging.NewLogger("debug")
	tempDir := t.TempDir()

	config := FileStorageConfig{
		LogDir:      tempDir,
		MaxFileSize: 1024,
		MaxAge:      24 * time.Hour,
		Enabled:     true,
	}

	fs, err := NewFileStorage(config, logger)
	if err != nil {
		t.Fatalf("NewFileStorage() error = %v", err)
	}

	oldFile := filepath.Join(tempDir, "audit-old.jsonl")
	f, err := os.Create(oldFile)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	f.WriteString(`{"id":"test","timestamp":"2020-01-01T00:00:00Z","event":"test","success":true}` + "\n")
	f.Close()

	info, _ := os.Stat(oldFile)
	oldTime := time.Now().Add(-25 * time.Hour)
	os.Chtimes(oldFile, oldTime, oldTime)

	if err := fs.compressFile(oldFile); err != nil {
		t.Errorf("compressFile() error = %v", err)
	}

	compressedFile := oldFile + ".gz"
	if _, err := os.Stat(compressedFile); os.IsNotExist(err) {
		t.Error("Expected compressed file to exist")
	}

	if _, err := os.Stat(oldFile); err == nil {
		t.Error("Expected original file to be removed after compression")
	}

	fs.Close()

	if info != nil {
		t.Logf("Original file info: %v", info)
	}
}

func TestFileStorage_Cleanup(t *testing.T) {
	logger := logging.NewLogger("debug")
	tempDir := t.TempDir()

	config := FileStorageConfig{
		LogDir:      tempDir,
		MaxFileSize: 1024,
		MaxAge:      1 * time.Hour,
		Enabled:     true,
	}

	fs, err := NewFileStorage(config, logger)
	if err != nil {
		t.Fatalf("NewFileStorage() error = %v", err)
	}
	defer fs.Close()

	oldFile := filepath.Join(tempDir, "audit-very-old.jsonl")
	f, err := os.Create(oldFile)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	f.WriteString(`{"id":"test","timestamp":"2020-01-01T00:00:00Z","event":"test","success":true}` + "\n")
	f.Close()

	oldTime := time.Now().Add(-2 * time.Hour)
	os.Chtimes(oldFile, oldTime, oldTime)

	fs.cleanupOldFiles()

	if _, err := os.Stat(oldFile); err == nil {
		t.Error("Expected old file to be removed by cleanup")
	}
}

func TestMatchesFilter(t *testing.T) {
	now := time.Now()
	entry := AuditEntry{
		ID:        "test",
		Timestamp: now,
		Event:     "test.event",
		UserID:    "user1",
		ClientID:  "client1",
		Success:   true,
	}

	tests := []struct {
		name   string
		entry  AuditEntry
		filter *AuditFilter
		want   bool
	}{
		{
			name:   "nil filter",
			entry:  entry,
			filter: nil,
			want:   true,
		},
		{
			name:  "matching event",
			entry: entry,
			filter: &AuditFilter{
				Event: "test.event",
			},
			want: true,
		},
		{
			name:  "non-matching event",
			entry: entry,
			filter: &AuditFilter{
				Event: "other.event",
			},
			want: false,
		},
		{
			name:  "matching user",
			entry: entry,
			filter: &AuditFilter{
				UserID: "user1",
			},
			want: true,
		},
		{
			name:  "matching client",
			entry: entry,
			filter: &AuditFilter{
				ClientID: "client1",
			},
			want: true,
		},
		{
			name:  "matching success",
			entry: entry,
			filter: &AuditFilter{
				Success: boolPtr(true),
			},
			want: true,
		},
		{
			name:  "non-matching success",
			entry: entry,
			filter: &AuditFilter{
				Success: boolPtr(false),
			},
			want: false,
		},
		{
			name:  "matching time range",
			entry: entry,
			filter: &AuditFilter{
				StartTime: now.Add(-1 * time.Hour),
				EndTime:   now.Add(1 * time.Hour),
			},
			want: true,
		},
		{
			name:  "outside time range",
			entry: entry,
			filter: &AuditFilter{
				StartTime: now.Add(-2 * time.Hour),
				EndTime:   now.Add(-1 * time.Hour),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesFilter(tt.entry, tt.filter)
			if got != tt.want {
				t.Errorf("matchesFilter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func boolPtr(b bool) *bool {

	return &b
}