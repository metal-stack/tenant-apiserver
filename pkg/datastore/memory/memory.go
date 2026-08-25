package memory

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	v1 "github.com/metal-stack/tenant-api/go/api/v1"
	"github.com/metal-stack/tenant-apiserver/pkg/api"
	"github.com/metal-stack/tenant-apiserver/pkg/errorutil"
)

type memoryDatastore[E api.Entity] struct {
	lock     sync.RWMutex
	entities map[string]E
	entity   string
	log      *slog.Logger
}

func NewMemory[E api.Entity](log *slog.Logger, e E) api.Storage[E] {
	entity := e.JSONField()
	return &memoryDatastore[E]{
		lock:     sync.RWMutex{},
		entities: make(map[string]E),
		entity:   entity,
		log:      log,
	}
}

// Create implements Storage.
func (m *memoryDatastore[E]) Create(ctx context.Context, ve E) error {
	m.log.Debug("create", "entity", m.entity, "value", ve)
	meta := ve.GetMeta()
	if meta == nil {
		return fmt.Errorf("create of type:%s failed, meta is nil", m.entity)
	}

	id := ve.GetMeta().Id
	m.lock.Lock()
	defer m.lock.Unlock()

	_, ok := m.entities[id]
	if ok {
		return errorutil.Conflict("an entity of type:%s with the id:%s already exists", m.entity, id)
	}
	m.entities[id] = ve
	return nil
}

// Delete implements Storage.
func (m *memoryDatastore[E]) Delete(ctx context.Context, id string) error {
	m.log.Debug("delete", "entity", m.entity, "id", id)

	m.lock.Lock()
	defer m.lock.Unlock()

	_, ok := m.entities[id]
	if !ok {
		return errorutil.NotFound("delete of %s with id %s", m.entity, id)
	}
	delete(m.entities, id)

	return nil
}

// DeleteAll implements Storage.
func (m *memoryDatastore[E]) DeleteAll(ctx context.Context, ids ...string) error {
	m.entities = make(map[string]E)
	return nil
}

// Find implements Storage.
func (m *memoryDatastore[E]) Find(ctx context.Context, paging *v1.Paging, filters ...any) ([]E, *uint64, error) {
	m.log.Debug("find", "entity", m.entity, "filter", filters)

	m.lock.Lock()
	defer m.lock.Unlock()

	var result []E
	for _, e := range m.entities {
		// FIXME implement filtering
		result = append(result, e)
	}

	return result, nil, nil
}

// Get implements Storage.
func (m *memoryDatastore[E]) Get(ctx context.Context, id string) (E, error) {
	m.log.Debug("get", "entity", m.entity, "id", id)
	var zero E
	m.lock.Lock()
	defer m.lock.Unlock()

	e, ok := m.entities[id]
	if !ok {
		return zero, errorutil.NotFound("get of %s with id %s", m.entity, id)
	}

	return e, nil
}

// GetHistory implements Storage.
func (m *memoryDatastore[E]) GetHistory(ctx context.Context, id string, at time.Time, ve E) error {
	return fmt.Errorf("gethistory is not implemented in the memory backend")
}

// GetHistoryCreated implements Storage.
func (m *memoryDatastore[E]) GetHistoryCreated(ctx context.Context, id string, ve E) error {
	return fmt.Errorf("gethistorycreated is not implemented in the memory backend")
}

// Update implements Storage.
func (m *memoryDatastore[E]) Update(ctx context.Context, ve E) error {
	m.log.Debug("update", "entity", m.entity)
	meta := ve.GetMeta()
	if meta == nil {
		return fmt.Errorf("update of type:%s failed, meta is nil", m.entity)
	}
	id := ve.GetMeta().Id
	m.lock.Lock()
	defer m.lock.Unlock()

	_, ok := m.entities[id]
	if !ok {
		return errorutil.NotFound("update of %s with id %s", m.entity, id)
	}

	m.entities[id] = ve
	return nil
}
