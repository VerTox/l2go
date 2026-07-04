package registry

import (
	"sync"

	"github.com/VerTox/l2go/internal/gameserver/transport/client"
)

// ConnectionRegistry manages mapping between account names and active client connections
// This enables broadcasting packets to specific players by account name
type ConnectionRegistry struct {
	mutex       sync.RWMutex
	connections map[string]*client.ClientConn // accountName -> ClientConn
}

// NewConnectionRegistry creates a new connection registry
func NewConnectionRegistry() *ConnectionRegistry {
	return &ConnectionRegistry{
		connections: make(map[string]*client.ClientConn),
	}
}

// Register adds a client connection to the registry
func (cr *ConnectionRegistry) Register(accountName string, conn *client.ClientConn) {
	cr.mutex.Lock()
	defer cr.mutex.Unlock()
	
	cr.connections[accountName] = conn
}

// Unregister removes a client connection from the registry
func (cr *ConnectionRegistry) Unregister(accountName string) {
	cr.mutex.Lock()
	defer cr.mutex.Unlock()
	
	delete(cr.connections, accountName)
}

// UnregisterIf removes the connection for accountName only if the currently
// registered connection is conn. This makes disconnect cleanup safe when a
// newer login for the same account has already replaced the registration:
// the old connection's deferred cleanup must not clobber the new one.
func (cr *ConnectionRegistry) UnregisterIf(accountName string, conn *client.ClientConn) {
	cr.mutex.Lock()
	defer cr.mutex.Unlock()

	if cr.connections[accountName] == conn {
		delete(cr.connections, accountName)
	}
}

// GetConnection retrieves a client connection by account name
func (cr *ConnectionRegistry) GetConnection(accountName string) *client.ClientConn {
	cr.mutex.RLock()
	defer cr.mutex.RUnlock()
	
	return cr.connections[accountName]
}

// GetAllConnections returns all active connections
func (cr *ConnectionRegistry) GetAllConnections() map[string]*client.ClientConn {
	cr.mutex.RLock()
	defer cr.mutex.RUnlock()
	
	// Create a copy to avoid race conditions
	result := make(map[string]*client.ClientConn)
	for accountName, conn := range cr.connections {
		result[accountName] = conn
	}
	
	return result
}

// GetConnectionCount returns the number of active connections
func (cr *ConnectionRegistry) GetConnectionCount() int {
	cr.mutex.RLock()
	defer cr.mutex.RUnlock()
	
	return len(cr.connections)
}

// BroadcastToAll sends a packet to all connected clients
func (cr *ConnectionRegistry) BroadcastToAll(packetData []byte) {
	cr.mutex.RLock()
	connections := make([]*client.ClientConn, 0, len(cr.connections))
	for _, conn := range cr.connections {
		connections = append(connections, conn)
	}
	cr.mutex.RUnlock()
	
	// Send outside of the lock to avoid blocking
	for _, conn := range connections {
		if err := conn.Send(packetData); err != nil {
			// Log error but continue with other connections
			// Note: Don't remove from registry here - let normal disconnect handle it
		}
	}
}

// BroadcastToAccounts sends a packet to specific accounts
func (cr *ConnectionRegistry) BroadcastToAccounts(packetData []byte, accountNames []string) {
	cr.mutex.RLock()
	connections := make([]*client.ClientConn, 0, len(accountNames))
	for _, accountName := range accountNames {
		if conn, exists := cr.connections[accountName]; exists {
			connections = append(connections, conn)
		}
	}
	cr.mutex.RUnlock()
	
	// Send outside of the lock to avoid blocking
	for _, conn := range connections {
		if err := conn.Send(packetData); err != nil {
			// Log error but continue with other connections
		}
	}
}