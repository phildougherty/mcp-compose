package server

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/phildougherty/mcp-compose/internal/config"
	"github.com/phildougherty/mcp-compose/internal/logging"
)

// ConnectionPool manages a pool of reusable connections per server
type ConnectionPool struct {
	serverName      string
	serverConfig    config.ServerConfig
	handler         *ProxyHandler
	logger          *logging.Logger
	mu              sync.RWMutex
	connections     chan *PooledConnection
	maxConnections  int
	maxIdleTime     time.Duration
	maxLifetime     time.Duration
	activeCount     int
	totalCreated    int64
	totalReused     int64
	totalClosed     int64
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	cleanupInterval time.Duration
}

// PooledConnection wraps a connection with lifecycle management
type PooledConnection struct {
	conn       interface{}
	serverName string
	createdAt  time.Time
	lastUsed   time.Time
	useCount   int64
	mu         sync.Mutex
}

// ConnectionPoolConfig holds pool configuration
type ConnectionPoolConfig struct {
	MaxConnections  int
	MaxIdleTime     time.Duration
	MaxLifetime     time.Duration
	CleanupInterval time.Duration
}

// DefaultPoolConfig returns sensible defaults for connection pooling
func DefaultPoolConfig() ConnectionPoolConfig {
	return ConnectionPoolConfig{
		MaxConnections:  10,
		MaxIdleTime:     60 * time.Second,
		MaxLifetime:     10 * time.Minute,
		CleanupInterval: 30 * time.Second,
	}
}

// NewConnectionPool creates a new connection pool for a server
func NewConnectionPool(serverName string, serverConfig config.ServerConfig, handler *ProxyHandler, cfg ConnectionPoolConfig) *ConnectionPool {
	ctx, cancel := context.WithCancel(context.Background())

	pool := &ConnectionPool{
		serverName:      serverName,
		serverConfig:    serverConfig,
		handler:         handler,
		logger:          handler.logger,
		connections:     make(chan *PooledConnection, cfg.MaxConnections),
		maxConnections:  cfg.MaxConnections,
		maxIdleTime:     cfg.MaxIdleTime,
		maxLifetime:     cfg.MaxLifetime,
		cleanupInterval: cfg.CleanupInterval,
		ctx:             ctx,
		cancel:          cancel,
	}

	pool.wg.Add(1)
	go pool.cleanupWorker()

	return pool
}

// Get retrieves a connection from the pool or creates a new one
func (p *ConnectionPool) Get(ctx context.Context) (*PooledConnection, error) {
	select {
	case <-ctx.Done():

		return nil, ctx.Err()
	case conn := <-p.connections:
		p.mu.Lock()
		p.totalReused++
		p.mu.Unlock()

		conn.mu.Lock()
		conn.lastUsed = time.Now()
		conn.useCount++
		conn.mu.Unlock()

		if p.isConnectionValid(conn) {
			p.logger.Debug("Reusing pooled connection for %s (used %d times)", p.serverName, conn.useCount)

			return conn, nil
		}

		p.logger.Debug("Pooled connection expired for %s, creating new one", p.serverName)
		p.closeConnection(conn)

		return p.createConnection()
	default:
		p.mu.RLock()
		canCreate := p.activeCount < p.maxConnections
		p.mu.RUnlock()

		if !canCreate {
			select {
			case <-ctx.Done():

				return nil, ctx.Err()
			case conn := <-p.connections:
				p.mu.Lock()
				p.totalReused++
				p.mu.Unlock()

				conn.mu.Lock()
				conn.lastUsed = time.Now()
				conn.useCount++
				conn.mu.Unlock()

				return conn, nil
			case <-time.After(5 * time.Second):

				return nil, fmt.Errorf("timeout waiting for available connection to %s", p.serverName)
			}
		}

		return p.createConnection()
	}
}

// Put returns a connection to the pool
func (p *ConnectionPool) Put(conn *PooledConnection) {
	if conn == nil {

		return
	}

	conn.mu.Lock()
	conn.lastUsed = time.Now()
	conn.mu.Unlock()

	if !p.isConnectionValid(conn) {
		p.logger.Debug("Connection expired, not returning to pool for %s", p.serverName)
		p.closeConnection(conn)

		return
	}

	select {
	case p.connections <- conn:
		p.logger.Debug("Returned connection to pool for %s", p.serverName)
	default:
		p.logger.Debug("Pool full, closing connection for %s", p.serverName)
		p.closeConnection(conn)
	}
}

// createConnection creates a new pooled connection
func (p *ConnectionPool) createConnection() (*PooledConnection, error) {
	p.mu.Lock()
	p.activeCount++
	p.totalCreated++
	p.mu.Unlock()

	now := time.Now()
	conn := &PooledConnection{
		serverName: p.serverName,
		createdAt:  now,
		lastUsed:   now,
		useCount:   1,
	}

	var err error
	switch p.serverConfig.Protocol {
	case "http":
		httpConn, httpErr := p.handler.getServerConnection(p.serverName)
		if httpErr != nil {
			p.mu.Lock()
			p.activeCount--
			p.mu.Unlock()

			return nil, httpErr
		}
		conn.conn = httpConn
	case "sse":
		sseConn, sseErr := p.handler.getOptimalSSEConnection(p.serverName)
		if sseErr != nil {
			p.mu.Lock()
			p.activeCount--
			p.mu.Unlock()

			return nil, sseErr
		}
		conn.conn = sseConn
	default:
		p.mu.Lock()
		p.activeCount--
		p.mu.Unlock()

		return nil, fmt.Errorf("connection pooling not yet supported for protocol %s", p.serverConfig.Protocol)
	}

	p.logger.Info("Created new pooled connection for %s (active: %d/%d)", p.serverName, p.activeCount, p.maxConnections)

	return conn, err
}

// isConnectionValid checks if a connection is still valid
func (p *ConnectionPool) isConnectionValid(conn *PooledConnection) bool {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	now := time.Now()

	if now.Sub(conn.createdAt) > p.maxLifetime {

		return false
	}

	if now.Sub(conn.lastUsed) > p.maxIdleTime {

		return false
	}

	return true
}

// closeConnection closes a connection and decrements active count
func (p *ConnectionPool) closeConnection(conn *PooledConnection) {
	if conn == nil {

		return
	}

	p.mu.Lock()
	p.activeCount--
	p.totalClosed++
	p.mu.Unlock()

	switch c := conn.conn.(type) {
	case *EnhancedMCPSSEConnection:
		p.handler.closeEnhancedSSEConnection(c)
	case *MCPSSEConnection:
		p.handler.closeSSEConnection(c)
	case *MCPSTDIOConnection:
		if c.Connection != nil {
			_ = c.Connection.Close()
		}
	}

	p.logger.Debug("Closed pooled connection for %s (active: %d)", p.serverName, p.activeCount)
}

// cleanupWorker periodically removes expired connections
func (p *ConnectionPool) cleanupWorker() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			p.logger.Debug("Connection pool cleanup worker stopping for %s", p.serverName)

			return
		case <-ticker.C:
			p.cleanupExpiredConnections()
		}
	}
}

// cleanupExpiredConnections removes expired connections from the pool
func (p *ConnectionPool) cleanupExpiredConnections() {
	var expiredConns []*PooledConnection

	poolSize := len(p.connections)
	for i := 0; i < poolSize; i++ {
		select {
		case conn := <-p.connections:
			if !p.isConnectionValid(conn) {
				expiredConns = append(expiredConns, conn)
			} else {
				p.connections <- conn
			}
		default:

			break
		}
	}

	for _, conn := range expiredConns {
		p.logger.Debug("Cleaning up expired connection for %s", p.serverName)
		p.closeConnection(conn)
	}

	if len(expiredConns) > 0 {
		p.logger.Info("Cleaned up %d expired connections for %s", len(expiredConns), p.serverName)
	}
}

// Close shuts down the connection pool
func (p *ConnectionPool) Close() error {
	p.logger.Info("Closing connection pool for %s", p.serverName)

	if p.cancel != nil {
		p.cancel()
	}

	close(p.connections)

	for conn := range p.connections {
		p.closeConnection(conn)
	}

	p.wg.Wait()

	p.logger.Info("Connection pool closed for %s (created: %d, reused: %d, closed: %d)",
		p.serverName, p.totalCreated, p.totalReused, p.totalClosed)

	return nil
}

// Stats returns current pool statistics
func (p *ConnectionPool) Stats() PoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return PoolStats{
		ServerName:     p.serverName,
		ActiveCount:    p.activeCount,
		IdleCount:      len(p.connections),
		MaxConnections: p.maxConnections,
		TotalCreated:   p.totalCreated,
		TotalReused:    p.totalReused,
		TotalClosed:    p.totalClosed,
	}
}

// PoolStats contains pool statistics
type PoolStats struct {
	ServerName     string
	ActiveCount    int
	IdleCount      int
	MaxConnections int
	TotalCreated   int64
	TotalReused    int64
	TotalClosed    int64
}