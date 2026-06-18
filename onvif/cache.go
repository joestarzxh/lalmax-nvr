package onvif

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ClientPool manages a pool of ONVIF clients.
type ClientPool struct {
	clients map[string]*Client
	mu      sync.RWMutex
	factory func(endpoint, username, password string, opts ...ClientOption) (*Client, error)
}

// NewClientPool creates a new client pool.
func NewClientPool() *ClientPool {
	return &ClientPool{
		clients: make(map[string]*Client),
		factory: NewClient,
	}
}

// GetClient returns a client for the given endpoint, creating one if needed.
func (p *ClientPool) GetClient(endpoint, username, password string) (*Client, error) {
	key := p.clientKey(endpoint, username)

	// Try to get existing client
	p.mu.RLock()
	if client, ok := p.clients[key]; ok && client.IsReady() {
		p.mu.RUnlock()
		return client, nil
	}
	p.mu.RUnlock()

	// Create new client
	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check after acquiring write lock
	if client, ok := p.clients[key]; ok && client.IsReady() {
		return client, nil
	}

	client, err := p.factory(endpoint, username, password)
	if err != nil {
		return nil, fmt.Errorf("onvif: create client failed: %w", err)
	}

	// Connect with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		return nil, fmt.Errorf("onvif: connect failed: %w", err)
	}

	p.clients[key] = client
	return client, nil
}

// RemoveClient removes a client from the pool.
func (p *ClientPool) RemoveClient(endpoint, username string) {
	key := p.clientKey(endpoint, username)

	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.clients, key)
}

// Clear removes all clients from the pool.
func (p *ClientPool) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.clients = make(map[string]*Client)
}

// Size returns the number of clients in the pool.
func (p *ClientPool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return len(p.clients)
}

// clientKey generates a unique key for a client.
func (p *ClientPool) clientKey(endpoint, username string) string {
	return fmt.Sprintf("%s@%s", username, endpoint)
}

// Cache provides caching for ONVIF data.
type Cache struct {
	items map[string]*cacheItem
	mu    sync.RWMutex
}

type cacheItem struct {
	data      interface{}
	expiresAt time.Time
}

// NewCache creates a new cache.
func NewCache() *Cache {
	return &Cache{
		items: make(map[string]*cacheItem),
	}
}

// Get retrieves an item from the cache.
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, ok := c.items[key]
	if !ok {
		return nil, false
	}

	if time.Now().After(item.expiresAt) {
		delete(c.items, key)
		return nil, false
	}

	return item.data, true
}

// Set stores an item in the cache with a TTL.
func (c *Cache) Set(key string, data interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = &cacheItem{
		data:      data,
		expiresAt: time.Now().Add(ttl),
	}
}

// Delete removes an item from the cache.
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
}

// Clear removes all items from the cache.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*cacheItem)
}

// Cleanup removes expired items from the cache.
func (c *Cache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, item := range c.items {
		if now.After(item.expiresAt) {
			delete(c.items, key)
		}
	}
}

// StartCleanup starts a background goroutine to clean up expired items.
func (c *Cache) StartCleanup(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.Cleanup()
			}
		}
	}()
}
