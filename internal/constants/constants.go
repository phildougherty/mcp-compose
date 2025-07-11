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
	
	// Additional timeout constants
	WebSocketReadTimeout        = 60 * time.Second
	MaxIdleTime                 = 10 * time.Minute
	StdioConnectTimeout         = 15 * time.Second
	LongOperationTimeout        = 120 * time.Second
	
	// Error buffer constants
	ErrorChannelSize            = 10
	
	// WebSocket specific constants
	WebSocketBufferSizeLarge    = 4096
	WebSocketPingIntervalLegacy = 54 * time.Second
	
	// Protocol error timeouts
	ProtocolErrorTimeout5       = 5
	ProtocolErrorTimeout30      = 30
	ProtocolErrorTimeout60      = 60
	
	// Path parsing constants
	MinPathParts                = 4
	MinMatchParts               = 2
	MinServerParts              = 2
	
	// Read buffer constants
	ReadBufferLimit             = 1024
	StreamChannelSize           = 100

	// Container and task scheduler constants
	TaskSchedulerDefaultPort    = 8080
	TaskSchedulerRetryLimit     = 3
	ContainerHealthTimeout      = 30 * time.Second
	ImageBuildRetryLimit        = 3
	ImageBuildDelay             = 5 * time.Second

	// Resource limits
	ResourceLimitCPUs           = "2.0"
	ResourceLimitMemory         = "1g"

	// Max length constraints
	MaxPrefixLength             = 10000

	// ID generation
	IDGenerationBase            = 10

	// HTTP status codes
	HTTPStatusNotFound          = 404
	HTTPStatusOK                = 200

	// NanoTime conversion
	NanoTimeBase                = 1e9

	// String lengths
	ContainerIDDisplayLength    = 12
	DefaultWorkspacePath        = "/home/phil"
	DefaultDatabasePath         = "/data/task-scheduler.db"
	DefaultHostInterface        = "0.0.0.0"
	DefaultLogLevel             = "info"
	DefaultGoModulesProxy       = "proxy.golang.org"

	// Default values
	DefaultDockerProgressMode   = "plain"
	DefaultBuildCacheOption     = "--no-cache"

	// Container timeouts
	ContainerStartRetryDelay    = 2 * time.Second
	ContainerHealthCheckDelay   = 1 * time.Second

	// WebSocket write timeout
	WebSocketWriteDeadline      = 5 * time.Second
	
	// Metrics and monitoring
	MetricsUpdateInterval       = 5 * time.Second
	HealthCheckBufferSize       = 100
	StaleConnectionThreshold    = 15 * time.Minute
	MonitoringInterval          = 2 * time.Minute

	// HTTP request timeouts
	HTTPRequestTimeout          = 30 * time.Second
	HTTPInitTimeout             = 90 * time.Second
	HTTPNotificationTimeout     = 20 * time.Second
	HTTPQuickTimeout            = 10 * time.Second
	HTTPExtendedTimeout         = 60 * time.Second
	HTTPLongTimeout             = 90 * time.Second
	HTTPStreamTimeout           = 120 * time.Second
	HTTPContextTimeout          = 15 * time.Second

	// Buffer sizes for HTTP responses
	HTTPResponseBufferSize      = 1024
	HTTPErrorBufferSize         = 256
	HTTPLogBufferSize           = 512

	// Retry and backoff
	RetryBackoffBase            = 2
	RetryBackoffMultiplier      = 3
	RetryAttemptThreshold       = 3
	RetryMaxAttempts            = 5

	// Path parsing
	URLPathParts                = 3
	URLPathPartsExtended        = 4
	ServerNameParts             = 2

	// Stream and channel sizes
	StreamChannelBuffer         = 100
	SSEResponseBuffer           = 1024
	SSEStreamBuffer             = 100

	// Sleep durations
	ShortSleepDuration          = 100 * time.Millisecond
	MediumSleepDuration         = 500 * time.Millisecond
	LongSleepDuration           = 2 * time.Second

	// Sync intervals
	SyncIntervalDefault         = 5 * time.Second
	SyncIntervalLong            = 30 * time.Second
	SyncFallbackTimeout         = 5 * time.Second

	// Idle timeouts
	IdleTimeoutDefault          = 10 * time.Minute
	IdleTimeoutExtended         = 15 * time.Minute

	// STDIO buffer sizes
	STDIOBufferSize             = 8192

	// Cleanup intervals
	CleanupIntervalDefault      = 30 * time.Minute
	CleanupIntervalExtended     = 60 * time.Second

	// Additional HTTP and connection constants
	HTTPTransportMaxIdleConns      = 50
	HTTPTransportMaxIdleConnsPerHost = 10
	HTTPTransportIdleConnTimeout   = 30 * time.Second
	HTTPTransportTLSHandshakeTimeout = 10 * time.Second
	HTTPTransportMaxConnsPerHost   = 20
	HTTPTransportBufferSize        = 4096

	// HTTP/2 transport constants
	HTTP2TransportMaxIdleConns     = 10
	HTTP2TransportMaxIdleConnsPerHost = 5
	HTTP2TransportIdleConnTimeout  = 5 * time.Minute
	HTTP2TransportMaxConnsPerHost  = 5

	// Additional timeout constants
	HTTPClientTimeout              = 60 * time.Second
	LifecycleTimeout               = 5 * time.Minute
	PingTimeout                    = 30 * time.Second
	KeepAlivePeriod                = 15 * time.Second
	WriteDeadlineTimeout           = 60 * time.Second
	STDIOBridgeTimeout             = 120 * time.Second
	STDIOBridgeConnectTimeout      = 60 * time.Second
	ToolDiscoveryTimeout           = 10 * time.Second
	ManagerCleanupTimeout          = 30 * time.Second
	ManagerRetryDelay              = 5 * time.Second
	ManagerIdleConnDivisor         = 2
	ToolDiscoveryRetryMultiplier   = 2

	// HTTP status codes
	HTTPStatusSuccess              = 200

	// Enhanced performance constants
	PerformanceShortSleep          = 100 * time.Millisecond
)