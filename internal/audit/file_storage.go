package audit

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/phildougherty/mcp-compose/internal/constants"
	"github.com/phildougherty/mcp-compose/internal/logging"
)

const (
	defaultMaxFileSize     = 100 * 1024 * 1024
	defaultMaxAge          = 7 * 24 * time.Hour
	defaultRotationCheck   = 5 * time.Minute
	compressedFileExt      = ".gz"
	timestampFormat        = "20060102-150405"
	defaultFlushInterval   = 5 * time.Second
	defaultBufferSize      = 8192
	maxWriteRetries        = 3
	writeRetryDelay        = 100 * time.Millisecond
)

type FileStorageConfig struct {
	LogDir       string
	MaxFileSize  int64
	MaxAge       time.Duration
	BufferSize   int
	FlushInterval time.Duration
	Enabled      bool
}

type FileStorage struct {
	config    FileStorageConfig
	logger    *logging.Logger
	mu        sync.Mutex
	file      *os.File
	writer    *bufio.Writer
	currentSize int64
	stopCh    chan struct{}
	wg        sync.WaitGroup
	flushCh   chan struct{}
	ctx       context.Context
	cancel    context.CancelFunc
}

func NewFileStorage(config FileStorageConfig, logger *logging.Logger) (*FileStorage, error) {
	if !config.Enabled {

		return nil, nil
	}

	if config.LogDir == "" {

		return nil, fmt.Errorf("log directory is required")
	}

	if config.MaxFileSize <= 0 {
		config.MaxFileSize = defaultMaxFileSize
	}
	if config.MaxAge <= 0 {
		config.MaxAge = defaultMaxAge
	}
	if config.BufferSize <= 0 {
		config.BufferSize = defaultBufferSize
	}
	if config.FlushInterval <= 0 {
		config.FlushInterval = defaultFlushInterval
	}

	if err := os.MkdirAll(config.LogDir, constants.DefaultDirMode); err != nil {

		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	fs := &FileStorage{
		config:  config,
		logger:  logger,
		stopCh:  make(chan struct{}),
		flushCh: make(chan struct{}, 1),
		ctx:     ctx,
		cancel:  cancel,
	}

	if err := fs.openNewFile(); err != nil {
		cancel()

		return nil, fmt.Errorf("failed to open initial log file: %w", err)
	}

	fs.wg.Add(2)
	go fs.rotationWorker()
	go fs.flushWorker()

	return fs, nil
}

func (fs *FileStorage) Write(entry *AuditEntry) error {
	if entry == nil {

		return fmt.Errorf("audit entry is nil")
	}

	data, err := json.Marshal(entry)
	if err != nil {

		return fmt.Errorf("failed to marshal audit entry: %w", err)
	}

	data = append(data, '\n')

	fs.mu.Lock()
	defer fs.mu.Unlock()

	if fs.writer == nil {

		return fmt.Errorf("file writer is not initialized")
	}

	var writeErr error
	for attempt := 0; attempt < maxWriteRetries; attempt++ {
		n, writeErr := fs.writer.Write(data)
		if writeErr == nil {
			fs.currentSize += int64(n)

			select {
			case fs.flushCh <- struct{}{}:
			default:
			}

			return nil
		}

		fs.logger.Warning("Write attempt %d failed: %v", attempt+1, writeErr)
		time.Sleep(writeRetryDelay)
	}

	return fmt.Errorf("failed to write audit entry after %d attempts: %w", maxWriteRetries, writeErr)
}

func (fs *FileStorage) WriteBatch(entries []*AuditEntry) error {
	if len(entries) == 0 {

		return nil
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	if fs.writer == nil {

		return fmt.Errorf("file writer is not initialized")
	}

	for _, entry := range entries {
		data, err := json.Marshal(entry)
		if err != nil {
			fs.logger.Warning("Failed to marshal audit entry: %v", err)

			continue
		}

		data = append(data, '\n')

		n, err := fs.writer.Write(data)
		if err != nil {

			return fmt.Errorf("failed to write audit entry: %w", err)
		}

		fs.currentSize += int64(n)
	}

	if err := fs.writer.Flush(); err != nil {

		return fmt.Errorf("failed to flush buffer: %w", err)
	}

	return nil
}

func (fs *FileStorage) Flush() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if fs.writer == nil {

		return nil
	}

	if err := fs.writer.Flush(); err != nil {

		return fmt.Errorf("failed to flush buffer: %w", err)
	}

	if fs.file != nil {
		if err := fs.file.Sync(); err != nil {

			return fmt.Errorf("failed to sync file: %w", err)
		}
	}

	return nil
}

func (fs *FileStorage) Close() error {
	fs.cancel()
	close(fs.stopCh)

	done := make(chan struct{})
	go func() {
		fs.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		fs.logger.Debug("File storage workers stopped")
	case <-time.After(constants.DefaultShutdownTimeout):
		fs.logger.Warning("File storage shutdown timeout")
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	if fs.writer != nil {
		if err := fs.writer.Flush(); err != nil {
			fs.logger.Warning("Failed to flush buffer during close: %v", err)
		}
		fs.writer = nil
	}

	if fs.file != nil {
		if err := fs.file.Close(); err != nil {
			fs.logger.Warning("Failed to close file: %v", err)
		}
		fs.file = nil
	}

	return nil
}

func (fs *FileStorage) openNewFile() error {
	timestamp := time.Now().Format(timestampFormat)
	filename := filepath.Join(fs.config.LogDir, fmt.Sprintf("audit-%s.jsonl", timestamp))

	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, constants.DefaultFileMode)
	if err != nil {

		return fmt.Errorf("failed to open file: %w", err)
	}

	if fs.writer != nil {
		if err := fs.writer.Flush(); err != nil {
			file.Close()

			return fmt.Errorf("failed to flush old buffer: %w", err)
		}
	}

	if fs.file != nil {
		if err := fs.file.Close(); err != nil {
			file.Close()

			return fmt.Errorf("failed to close old file: %w", err)
		}
	}

	fs.file = file
	fs.writer = bufio.NewWriterSize(file, fs.config.BufferSize)
	fs.currentSize = 0

	fileInfo, err := file.Stat()
	if err == nil {
		fs.currentSize = fileInfo.Size()
	}

	fs.logger.Info("Opened new audit log file: %s", filename)

	return nil
}

func (fs *FileStorage) rotationWorker() {
	defer fs.wg.Done()

	ticker := time.NewTicker(defaultRotationCheck)
	defer ticker.Stop()

	for {
		select {
		case <-fs.stopCh:

			return
		case <-ticker.C:
			fs.checkRotation()
			fs.cleanupOldFiles()
			fs.compressOldFiles()
		}
	}
}

func (fs *FileStorage) flushWorker() {
	defer fs.wg.Done()

	ticker := time.NewTicker(fs.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-fs.stopCh:

			return
		case <-fs.flushCh:
			if err := fs.Flush(); err != nil {
				fs.logger.Warning("Failed to flush: %v", err)
			}
		case <-ticker.C:
			if err := fs.Flush(); err != nil {
				fs.logger.Warning("Failed to flush on interval: %v", err)
			}
		}
	}
}

func (fs *FileStorage) checkRotation() {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if fs.currentSize >= fs.config.MaxFileSize {
		fs.logger.Info("Rotating audit log file (size: %d bytes)", fs.currentSize)

		if err := fs.openNewFile(); err != nil {
			fs.logger.Warning("Failed to rotate log file: %v", err)
		}
	}
}

func (fs *FileStorage) cleanupOldFiles() {
	entries, err := os.ReadDir(fs.config.LogDir)
	if err != nil {
		fs.logger.Warning("Failed to read log directory: %v", err)

		return
	}

	cutoff := time.Now().Add(-fs.config.MaxAge)

	for _, entry := range entries {
		if entry.IsDir() {

			continue
		}

		info, err := entry.Info()
		if err != nil {

			continue
		}

		if info.ModTime().Before(cutoff) {
			filePath := filepath.Join(fs.config.LogDir, entry.Name())

			if err := os.Remove(filePath); err != nil {
				fs.logger.Warning("Failed to remove old log file %s: %v", filePath, err)
			} else {
				fs.logger.Debug("Removed old audit log file: %s", filePath)
			}
		}
	}
}

func (fs *FileStorage) compressOldFiles() {
	entries, err := os.ReadDir(fs.config.LogDir)
	if err != nil {
		fs.logger.Warning("Failed to read log directory: %v", err)

		return
	}

	cutoff := time.Now().Add(-24 * time.Hour)

	for _, entry := range entries {
		if entry.IsDir() {

			continue
		}

		name := entry.Name()

		if filepath.Ext(name) != ".jsonl" {

			continue
		}

		info, err := entry.Info()
		if err != nil {

			continue
		}

		if info.ModTime().Before(cutoff) {
			filePath := filepath.Join(fs.config.LogDir, name)

			if err := fs.compressFile(filePath); err != nil {
				fs.logger.Warning("Failed to compress file %s: %v", filePath, err)
			} else {
				fs.logger.Debug("Compressed audit log file: %s", filePath)
			}
		}
	}
}

func (fs *FileStorage) compressFile(filePath string) error {
	fs.mu.Lock()
	currentFile := ""
	if fs.file != nil {
		currentFile = fs.file.Name()
	}
	fs.mu.Unlock()

	if filePath == currentFile {

		return nil
	}

	input, err := os.Open(filePath)
	if err != nil {

		return fmt.Errorf("failed to open file: %w", err)
	}
	defer input.Close()

	outputPath := filePath + compressedFileExt
	output, err := os.Create(outputPath)
	if err != nil {

		return fmt.Errorf("failed to create compressed file: %w", err)
	}
	defer output.Close()

	gzWriter := gzip.NewWriter(output)
	defer gzWriter.Close()

	if _, err := io.Copy(gzWriter, input); err != nil {

		return fmt.Errorf("failed to compress file: %w", err)
	}

	if err := gzWriter.Close(); err != nil {

		return fmt.Errorf("failed to close gzip writer: %w", err)
	}

	if err := output.Close(); err != nil {

		return fmt.Errorf("failed to close output file: %w", err)
	}

	if err := input.Close(); err != nil {

		return fmt.Errorf("failed to close input file: %w", err)
	}

	if err := os.Remove(filePath); err != nil {

		return fmt.Errorf("failed to remove original file: %w", err)
	}

	return nil
}

func (fs *FileStorage) ReadEntries(limit int, offset int, filter *AuditFilter) ([]AuditEntry, int, error) {
	entries, err := os.ReadDir(fs.config.LogDir)
	if err != nil {

		return nil, 0, fmt.Errorf("failed to read log directory: %w", err)
	}

	var allEntries []AuditEntry

	for _, entry := range entries {
		if entry.IsDir() {

			continue
		}

		name := entry.Name()
		ext := filepath.Ext(name)

		if ext != ".jsonl" && ext != compressedFileExt {

			continue
		}

		filePath := filepath.Join(fs.config.LogDir, name)

		fileEntries, err := fs.readEntriesFromFile(filePath)
		if err != nil {
			fs.logger.Warning("Failed to read entries from %s: %v", filePath, err)

			continue
		}

		allEntries = append(allEntries, fileEntries...)
	}

	var filtered []AuditEntry
	for _, entry := range allEntries {
		if matchesFilter(entry, filter) {
			filtered = append(filtered, entry)
		}
	}

	total := len(filtered)

	start := offset
	if start > len(filtered) {
		start = len(filtered)
	}

	end := start + limit
	if end > len(filtered) {
		end = len(filtered)
	}

	return filtered[start:end], total, nil
}

func (fs *FileStorage) readEntriesFromFile(filePath string) ([]AuditEntry, error) {
	var entries []AuditEntry

	var reader io.ReadCloser
	file, err := os.Open(filePath)
	if err != nil {

		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	if filepath.Ext(filePath) == compressedFileExt {
		gzReader, err := gzip.NewReader(file)
		if err != nil {

			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader
	} else {
		reader = file
	}

	scanner := bufio.NewScanner(reader)

	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		var entry AuditEntry

		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			fs.logger.Warning("Failed to unmarshal audit entry: %v", err)

			continue
		}

		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {

		return nil, fmt.Errorf("scanner error: %w", err)
	}

	return entries, nil
}

func matchesFilter(entry AuditEntry, filter *AuditFilter) bool {
	if filter == nil {

		return true
	}

	if filter.Event != "" && entry.Event != filter.Event {

		return false
	}

	if filter.UserID != "" && entry.UserID != filter.UserID {

		return false
	}

	if filter.ClientID != "" && entry.ClientID != filter.ClientID {

		return false
	}

	if filter.Success != nil && entry.Success != *filter.Success {

		return false
	}

	if !filter.StartTime.IsZero() && entry.Timestamp.Before(filter.StartTime) {

		return false
	}

	if !filter.EndTime.IsZero() && entry.Timestamp.After(filter.EndTime) {

		return false
	}

	return true
}