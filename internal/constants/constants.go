package constants

import "time"

const (
	// Common timeout values
	DefaultConnectTimeout       = 10 * time.Second
	DefaultReadTimeout          = 30 * time.Second
	DefaultWriteTimeout         = 30 * time.Second
	DefaultShutdownTimeout      = 30 * time.Second
	DefaultHealthTimeout        = 5 * time.Second
	DefaultStatsTimeout         = 10 * time.Second
	DefaultLogStreamTimeout     = 120 * time.Second
	DefaultCleanupInterval      = 5 * time.Minute
	DefaultSessionCleanupTime   = 30 * time.Minute
	DefaultWebSocketTimeout     = 5 * time.Second
	DefaultConnectionTimeout    = 3 * time.Second
	DailyCleanupInterval        = 24 * time.Hour
	WebSocketPingInterval       = 30 * time.Second
	DefaultIdleTimeout          = 60 * time.Second
	ShortTimeout                = 15 * time.Second
	FileOperationTimeout        = 5 * time.Minute
	ConnectionKeepAlive         = 2 * time.Minute
	DefaultRetryDelay           = 2 * time.Second
	
	// Buffer sizes
	DefaultBufferSize           = 100
	DefaultChannelBuffer        = 100
	DefaultIOBufferSize         = 8192
	WebSocketBufferSize         = 1024
	WebSocketChannelSize        = 10
	ActivityChannelSize         = 1000
	
	// Time constants
	HoursInDay                  = 24
	SecondsInMinute             = 60
	
	// String parsing constants
	StringSplitParts            = 2
	LogFieldCount               = 6
	RandomStringLength          = 6
	
	// File permissions
	DefaultFileMode             = 0644
	DefaultDirMode              = 0755
	ExecutableFileMode          = 0755
	
	// WebSocket constants
	WebSocketPingIntervalOld    = 54 * time.Second
	WebSocketWriteTimeout       = 10 * time.Second
	
	// Protocol constants
	DefaultProtoTimeout         = 60 * time.Second
	
	// Percentage multiplier
	PercentageMultiplier        = 100
	PercentageMultiplierFloat   = 100.0
	
	// Rate limiting constants
	RateLimitInterval           = 50
	RateLimitDelay              = 10 * time.Millisecond
	RecentActivitiesCount       = 50
	
	// Retry constants
	DefaultRetryAttempts        = 3
	DefaultRetryLimit           = 3
	DefaultRetryCount           = 5
	
	// Port constants
	DefaultProxyPort            = 9876
	DefaultMemoryHTTPPort       = 3001
	
	// Time conversion constants
	NanosecondsToMilliseconds   = 1e6
	
	// Table formatting constants
	TableColumnSpacing          = 2
)