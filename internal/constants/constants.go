package constants

import "time"

const (
	// Common timeout values
	DefaultConnectTimeout   = 10 * time.Second
	DefaultReadTimeout      = 30 * time.Second
	DefaultWriteTimeout     = 30 * time.Second
	DefaultShutdownTimeout  = 30 * time.Second
	DefaultHealthTimeout    = 5 * time.Second
	DefaultStatsTimeout     = 10 * time.Second
	DefaultLogStreamTimeout = 120 * time.Second
	
	// Buffer sizes
	DefaultBufferSize       = 100
	DefaultChannelBuffer    = 100
	DefaultIOBufferSize     = 8192
	
	// Time constants
	HoursInDay              = 24
	SecondsInMinute         = 60
	
	// String parsing constants
	StringSplitParts        = 2
	
	// File permissions
	DefaultFileMode         = 0644
	DefaultDirMode          = 0755
	
	// WebSocket constants
	WebSocketPingInterval   = 54 * time.Second
	WebSocketWriteTimeout   = 10 * time.Second
	
	// Protocol constants
	DefaultProtoTimeout     = 60 * time.Second
	
	// Percentage multiplier
	PercentageMultiplier    = 100
)