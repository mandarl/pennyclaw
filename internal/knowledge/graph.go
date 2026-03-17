// Package knowledge implements a lightweight knowledge graph with Ebbinghaus
// memory decay. Entities (people, places, concepts, preferences) are extracted
// from conversations and stored with weighted relationships. Memories that are
// never reinforced gradually decay, keeping the graph lean and relevant.
//
// Storage: SQLite tables in the same database as memory/cron.
// Zero external dependencies.
package knowledge

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"
)

// EntityType classifies knowledge graph nodes.
type EntityType string

const (
	EntityPerson     EntityType = "person"
	EntityPlace      EntityType = "place"
	EntityOrg        EntityType = "organization"
	EntityConcept    EntityType = "concept"
	EntityPreference EntityType = "preference"
	EntityProject    EntityType = "project"
	EntityEvent      EntityType = "event"
	EntityFact       EntityType = "fact"
)

// Entity represents a node in the knowledge graph.
type Entity struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	Type       EntityType `json:"type"`
	Properties string     `json:"properties"` // JSON blob of key-value pairs
	Strength   float64    `json:"strength"`   // 0.0 to 1.0 — decays over time
	CreatedAt  time.Time  `json:"created_at"`
	LastSeen   time.Time  `json:"last_seen"`  // Last time this entity was referenced
	AccessCount int       `json:"access_count"`
}

// Relation represents an edge in the knowledge graph.
type Relation struct {
	ID         int64     `json:"id"`
	FromID     int64     `json:"from_id"`
	ToID       int64     `json:"to_id"`
	Type       string    `json:"type"`       // e.g., "works_at", "likes", "knows", "located_in"
	Strength   float64   `json:"strength"`   // 0.0 to 1.0
	CreatedAt  time.Time `json:"created_at"`
	LastSeen   time.Time `json:"last_seen"`
}

// Graph is the knowledge graph backed by SQLite.
type Graph struct {
	db *sql.DB
	mu sync.RWMutex

	// Ebbinghaus decay parameters
	decayRate    float64 // How fast memories fade (default: 0.3)
	decayMinimum float64 // Floor strength — never fully forgotten (default: 0.05)
	pruneThreshold float64 // Remove entities below this strength (default: 0.02)
}

// NewGraph creates a new knowledge graph using the given SQLite database.
func NewGraph(db *sql.DB) (*Graph, error) {
	g := &Graph{
		db:             db,
		decayRate:      0.3,
		decayMinimum:   0.05,
		pruneThreshold: 0.02,
	}

	if err := g.migrate(); err != nil {
		return nil, fmt.Errorf("migrating knowledge graph: %w", err)
	}

	// Start background decay goroutine
	go g.decayLoop()

	return g, nil
}

// migrate creates the knowledge graph tables if they don't exist.
func (g *Graph) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS kg_entities (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			properties TEXT DEFAULT '{}',
			strength REAL DEFAULT 1.0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
			access_count INTEGER DEFAULT 1
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_kg_entities_name_type ON kg_entities(name, type)`,
		`CREATE INDEX IF NOT EXISTS idx_kg_entities_strength ON kg_entities(strength)`,
		`CREATE TABLE IF NOT EXISTS kg_relations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			from_id INTEGER NOT NULL REFERENCES kg_entities(id) ON DELETE CASCADE,
			to_id INTEGER NOT NULL REFERENCES kg_entities(id) ON DELETE CASCADE,
			type TEXT NOT NULL,
			strength REAL DEFAULT 1.0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_seen DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_kg_relations_unique ON kg_relations(from_id, to_id, type)`,
		`CREATE INDEX IF NOT EXISTS idx_kg_relations_from ON kg_relations(from_id)`,
		`CREATE INDEX IF NOT EXISTS idx_kg_relations_to ON kg_relations(to_id)`,
	}

	for _, q := range queries {
		if _, err := g.db.Exec(q); err != nil {
			return fmt.Errorf("executing migration: %w", err)
		}
	}
	return nil
}

// UpsertEntity creates or reinforces an entity. If the entity already exists,
// its strength is boosted and last_seen is updated.
func (g *Graph) UpsertEntity(name string, entityType EntityType, properties map[string]string) (*Entity, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("entity name cannot be empty")
	}

	propsJSON, _ := json.Marshal(properties)
	now := time.Now()

	// Try to find existing entity
	var entity Entity
	err := g.db.QueryRow(
		`SELECT id, name, type, properties, strength, created_at, last_seen, access_count
		 FROM kg_entities WHERE name = ? AND type = ?`,
		name, string(entityType),
	).Scan(&entity.ID, &entity.Name, &entity.Type, &entity.Properties,
		&entity.Strength, &entity.CreatedAt, &entity.LastSeen, &entity.AccessCount)

	if err == sql.ErrNoRows {
		// Create new entity
		result, err := g.db.Exec(
			`INSERT INTO kg_entities (name, type, properties, strength, created_at, last_seen, access_count)
			 VALUES (?, ?, ?, 1.0, ?, ?, 1)`,
			name, string(entityType), string(propsJSON), now, now,
		)
		if err != nil {
			return nil, fmt.Errorf("inserting entity: %w", err)
		}
		id, _ := result.LastInsertId()
		return &Entity{
			ID: id, Name: name, Type: entityType,
			Properties: string(propsJSON), Strength: 1.0,
			CreatedAt: now, LastSeen: now, AccessCount: 1,
		}, nil
	} else if err != nil {
		return nil, fmt.Errorf("querying entity: %w", err)
	}

	// Reinforce existing entity — Ebbinghaus spacing effect:
	// Each reinforcement boosts strength, with diminishing returns
	newStrength := math.Min(1.0, entity.Strength+0.3*(1.0-entity.Strength))

	// Merge properties
	var existingProps map[string]string
	json.Unmarshal([]byte(entity.Properties), &existingProps)
	if existingProps == nil {
		existingProps = make(map[string]string)
	}
	for k, v := range properties {
		existingProps[k] = v
	}
	mergedJSON, _ := json.Marshal(existingProps)

	_, err = g.db.Exec(
		`UPDATE kg_entities SET strength = ?, last_seen = ?, access_count = access_count + 1, properties = ?
		 WHERE id = ?`,
		newStrength, now, string(mergedJSON), entity.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("updating entity: %w", err)
	}

	entity.Strength = newStrength
	entity.LastSeen = now
	entity.AccessCount++
	entity.Properties = string(mergedJSON)
	return &entity, nil
}

// UpsertRelation creates or reinforces a relationship between two entities.
func (g *Graph) UpsertRelation(fromID, toID int64, relType string) (*Relation, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()

	var rel Relation
	err := g.db.QueryRow(
		`SELECT id, from_id, to_id, type, strength, created_at, last_seen
		 FROM kg_relations WHERE from_id = ? AND to_id = ? AND type = ?`,
		fromID, toID, relType,
	).Scan(&rel.ID, &rel.FromID, &rel.ToID, &rel.Type, &rel.Strength, &rel.CreatedAt, &rel.LastSeen)

	if err == sql.ErrNoRows {
		result, err := g.db.Exec(
			`INSERT INTO kg_relations (from_id, to_id, type, strength, created_at, last_seen)
			 VALUES (?, ?, ?, 1.0, ?, ?)`,
			fromID, toID, relType, now, now,
		)
		if err != nil {
			return nil, fmt.Errorf("inserting relation: %w", err)
		}
		id, _ := result.LastInsertId()
		return &Relation{
			ID: id, FromID: fromID, ToID: toID, Type: relType,
			Strength: 1.0, CreatedAt: now, LastSeen: now,
		}, nil
	} else if err != nil {
		return nil, fmt.Errorf("querying relation: %w", err)
	}

	// Reinforce
	newStrength := math.Min(1.0, rel.Strength+0.3*(1.0-rel.Strength))
	_, err = g.db.Exec(
		`UPDATE kg_relations SET strength = ?, last_seen = ? WHERE id = ?`,
		newStrength, now, rel.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("updating relation: %w", err)
	}

	rel.Strength = newStrength
	rel.LastSeen = now
	return &rel, nil
}

// Query returns entities matching a search term, ordered by strength.
func (g *Graph) Query(search string, limit int) ([]Entity, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}

	rows, err := g.db.Query(
		`SELECT id, name, type, properties, strength, created_at, last_seen, access_count
		 FROM kg_entities
		 WHERE name LIKE ? AND strength > ?
		 ORDER BY strength DESC, access_count DESC
		 LIMIT ?`,
		"%"+search+"%", g.pruneThreshold, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("querying entities: %w", err)
	}
	defer rows.Close()

	var entities []Entity
	for rows.Next() {
		var e Entity
		if err := rows.Scan(&e.ID, &e.Name, &e.Type, &e.Properties,
			&e.Strength, &e.CreatedAt, &e.LastSeen, &e.AccessCount); err != nil {
			continue
		}
		entities = append(entities, e)
	}
	return entities, nil
}

// GetRelations returns all relations for an entity (both directions).
func (g *Graph) GetRelations(entityID int64) ([]map[string]interface{}, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	rows, err := g.db.Query(
		`SELECT r.id, r.type, r.strength,
		        CASE WHEN r.from_id = ? THEN e2.name ELSE e1.name END as related_name,
		        CASE WHEN r.from_id = ? THEN e2.type ELSE e1.type END as related_type,
		        CASE WHEN r.from_id = ? THEN 'outgoing' ELSE 'incoming' END as direction
		 FROM kg_relations r
		 JOIN kg_entities e1 ON r.from_id = e1.id
		 JOIN kg_entities e2 ON r.to_id = e2.id
		 WHERE (r.from_id = ? OR r.to_id = ?) AND r.strength > ?
		 ORDER BY r.strength DESC`,
		entityID, entityID, entityID, entityID, entityID, g.pruneThreshold,
	)
	if err != nil {
		return nil, fmt.Errorf("querying relations: %w", err)
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id int64
		var relType, relatedName, relatedType, direction string
		var strength float64
		if err := rows.Scan(&id, &relType, &strength, &relatedName, &relatedType, &direction); err != nil {
			continue
		}
		results = append(results, map[string]interface{}{
			"id":           id,
			"type":         relType,
			"strength":     strength,
			"related_name": relatedName,
			"related_type": relatedType,
			"direction":    direction,
		})
	}
	return results, nil
}

// GetContext returns a formatted string of the most relevant knowledge for
// inclusion in the system prompt. This is the main integration point with
// the agent — it provides contextual memory.
func (g *Graph) GetContext(limit int) string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if limit <= 0 {
		limit = 15
	}

	rows, err := g.db.Query(
		`SELECT name, type, properties, strength
		 FROM kg_entities
		 WHERE strength > 0.2
		 ORDER BY strength DESC, access_count DESC
		 LIMIT ?`, limit,
	)
	if err != nil {
		return ""
	}
	defer rows.Close()

	var parts []string
	for rows.Next() {
		var name, entityType, propsJSON string
		var strength float64
		if err := rows.Scan(&name, &entityType, &propsJSON, &strength); err != nil {
			continue
		}

		entry := fmt.Sprintf("- %s (%s, strength: %.0f%%)", name, entityType, strength*100)

		var props map[string]string
		if json.Unmarshal([]byte(propsJSON), &props) == nil && len(props) > 0 {
			var details []string
			for k, v := range props {
				details = append(details, fmt.Sprintf("%s: %s", k, v))
			}
			entry += " [" + strings.Join(details, ", ") + "]"
		}

		parts = append(parts, entry)
	}

	if len(parts) == 0 {
		return ""
	}

	return "--- Knowledge Graph (strongest memories) ---\n" + strings.Join(parts, "\n")
}

// Stats returns summary statistics about the knowledge graph.
func (g *Graph) Stats() map[string]interface{} {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var entityCount, relationCount int
	var avgStrength float64

	g.db.QueryRow(`SELECT COUNT(*), COALESCE(AVG(strength), 0) FROM kg_entities WHERE strength > ?`,
		g.pruneThreshold).Scan(&entityCount, &avgStrength)
	g.db.QueryRow(`SELECT COUNT(*) FROM kg_relations WHERE strength > ?`,
		g.pruneThreshold).Scan(&relationCount)

	return map[string]interface{}{
		"entities":      entityCount,
		"relations":     relationCount,
		"avg_strength":  math.Round(avgStrength*100) / 100,
		"decay_rate":    g.decayRate,
		"prune_threshold": g.pruneThreshold,
	}
}

// decayLoop runs every hour and applies Ebbinghaus forgetting curve to all
// entities and relations. The formula: S(t) = S0 * e^(-λt) where t is hours
// since last seen and λ is the decay rate.
func (g *Graph) decayLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		g.applyDecay()
	}
}

// applyDecay applies the Ebbinghaus forgetting curve to all entities and relations.
func (g *Graph) applyDecay() {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()

	// Decay entities
	rows, err := g.db.Query(
		`SELECT id, strength, last_seen, access_count FROM kg_entities WHERE strength > ?`,
		g.pruneThreshold,
	)
	if err != nil {
		log.Printf("Knowledge graph decay error: %v", err)
		return
	}

	type decayUpdate struct {
		id          int64
		newStrength float64
	}
	var updates []decayUpdate
	var pruneIDs []int64

	for rows.Next() {
		var id int64
		var strength float64
		var lastSeen time.Time
		var accessCount int

		if err := rows.Scan(&id, &strength, &lastSeen, &accessCount); err != nil {
			continue
		}

		// Hours since last seen
		hoursSince := now.Sub(lastSeen).Hours()
		if hoursSince < 1 {
			continue // Skip recently seen entities
		}

		// Ebbinghaus decay with spacing effect:
		// More frequently accessed memories decay slower
		adjustedRate := g.decayRate / math.Max(1.0, math.Log2(float64(accessCount)+1))
		newStrength := strength * math.Exp(-adjustedRate*hoursSince/24.0) // Decay per day

		// Apply minimum floor
		if newStrength < g.decayMinimum {
			newStrength = g.decayMinimum
		}

		if newStrength < g.pruneThreshold {
			pruneIDs = append(pruneIDs, id)
		} else if math.Abs(newStrength-strength) > 0.001 {
			updates = append(updates, decayUpdate{id, newStrength})
		}
	}
	rows.Close()

	// Apply updates
	for _, u := range updates {
		g.db.Exec(`UPDATE kg_entities SET strength = ? WHERE id = ?`, u.newStrength, u.id)
	}

	// Prune dead entities and their relations
	for _, id := range pruneIDs {
		g.db.Exec(`DELETE FROM kg_relations WHERE from_id = ? OR to_id = ?`, id, id)
		g.db.Exec(`DELETE FROM kg_entities WHERE id = ?`, id)
	}

	// Decay relations similarly
	g.db.Exec(
		`UPDATE kg_relations SET strength = strength * 0.99
		 WHERE last_seen < datetime('now', '-1 day') AND strength > ?`,
		g.pruneThreshold,
	)
	g.db.Exec(`DELETE FROM kg_relations WHERE strength < ?`, g.pruneThreshold)

	if len(pruneIDs) > 0 {
		log.Printf("Knowledge graph: decayed %d entities, pruned %d", len(updates), len(pruneIDs))
	}
}

// AllEntities returns all entities above the prune threshold.
func (g *Graph) AllEntities(limit int) ([]Entity, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	rows, err := g.db.Query(
		`SELECT id, name, type, properties, strength, created_at, last_seen, access_count
		 FROM kg_entities
		 WHERE strength > ?
		 ORDER BY strength DESC
		 LIMIT ?`,
		g.pruneThreshold, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entities []Entity
	for rows.Next() {
		var e Entity
		if err := rows.Scan(&e.ID, &e.Name, &e.Type, &e.Properties,
			&e.Strength, &e.CreatedAt, &e.LastSeen, &e.AccessCount); err != nil {
			continue
		}
		entities = append(entities, e)
	}
	return entities, nil
}

// DeleteEntity removes an entity and its relations.
func (g *Graph) DeleteEntity(id int64) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	_, err := g.db.Exec(`DELETE FROM kg_relations WHERE from_id = ? OR to_id = ?`, id, id)
	if err != nil {
		return fmt.Errorf("deleting relations: %w", err)
	}
	_, err = g.db.Exec(`DELETE FROM kg_entities WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting entity: %w", err)
	}
	return nil
}
