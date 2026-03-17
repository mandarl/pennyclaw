package knowledge

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	f, err := os.CreateTemp("", "kg-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })

	db, err := sql.Open("sqlite3", f.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestUpsertEntity(t *testing.T) {
	db := testDB(t)
	g, err := NewGraph(db)
	if err != nil {
		t.Fatal(err)
	}

	// Create new entity
	e, err := g.UpsertEntity("Alice", EntityPerson, map[string]string{"role": "engineer"})
	if err != nil {
		t.Fatal(err)
	}
	if e.Name != "Alice" || e.Type != EntityPerson || e.Strength != 1.0 {
		t.Errorf("unexpected entity: %+v", e)
	}

	// Reinforce — strength should stay at 1.0 (already max)
	e2, err := g.UpsertEntity("Alice", EntityPerson, map[string]string{"team": "infra"})
	if err != nil {
		t.Fatal(err)
	}
	if e2.ID != e.ID {
		t.Error("should reuse same entity ID")
	}
	if e2.AccessCount != 2 {
		t.Errorf("access count should be 2, got %d", e2.AccessCount)
	}
}

func TestUpsertRelation(t *testing.T) {
	db := testDB(t)
	g, err := NewGraph(db)
	if err != nil {
		t.Fatal(err)
	}

	e1, _ := g.UpsertEntity("Alice", EntityPerson, nil)
	e2, _ := g.UpsertEntity("Acme Corp", EntityOrg, nil)

	rel, err := g.UpsertRelation(e1.ID, e2.ID, "works_at")
	if err != nil {
		t.Fatal(err)
	}
	if rel.FromID != e1.ID || rel.ToID != e2.ID || rel.Type != "works_at" {
		t.Errorf("unexpected relation: %+v", rel)
	}

	// Reinforce
	rel2, err := g.UpsertRelation(e1.ID, e2.ID, "works_at")
	if err != nil {
		t.Fatal(err)
	}
	if rel2.ID != rel.ID {
		t.Error("should reuse same relation ID")
	}
}

func TestQuery(t *testing.T) {
	db := testDB(t)
	g, err := NewGraph(db)
	if err != nil {
		t.Fatal(err)
	}

	g.UpsertEntity("Alice", EntityPerson, nil)
	g.UpsertEntity("Bob", EntityPerson, nil)
	g.UpsertEntity("Golang", EntityConcept, nil)

	results, err := g.Query("Ali", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Name != "Alice" {
		t.Errorf("expected Alice, got %v", results)
	}

	// Empty search returns all
	all, err := g.Query("", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 entities, got %d", len(all))
	}
}

func TestGetRelations(t *testing.T) {
	db := testDB(t)
	g, err := NewGraph(db)
	if err != nil {
		t.Fatal(err)
	}

	e1, _ := g.UpsertEntity("Alice", EntityPerson, nil)
	e2, _ := g.UpsertEntity("Acme", EntityOrg, nil)
	e3, _ := g.UpsertEntity("Go", EntityConcept, nil)

	g.UpsertRelation(e1.ID, e2.ID, "works_at")
	g.UpsertRelation(e1.ID, e3.ID, "knows")

	rels, err := g.GetRelations(e1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rels) != 2 {
		t.Errorf("expected 2 relations, got %d", len(rels))
	}
}

func TestGetContext(t *testing.T) {
	db := testDB(t)
	g, err := NewGraph(db)
	if err != nil {
		t.Fatal(err)
	}

	g.UpsertEntity("Alice", EntityPerson, map[string]string{"role": "engineer"})
	g.UpsertEntity("PennyClaw", EntityProject, map[string]string{"language": "Go"})

	ctx := g.GetContext(10)
	if ctx == "" {
		t.Error("expected non-empty context")
	}
	if !containsStr(ctx, "Alice") || !containsStr(ctx, "PennyClaw") {
		t.Errorf("context missing entities: %s", ctx)
	}
}

func TestStats(t *testing.T) {
	db := testDB(t)
	g, err := NewGraph(db)
	if err != nil {
		t.Fatal(err)
	}

	g.UpsertEntity("Alice", EntityPerson, nil)
	g.UpsertEntity("Bob", EntityPerson, nil)

	stats := g.Stats()
	if stats["entities"].(int) != 2 {
		t.Errorf("expected 2 entities, got %v", stats["entities"])
	}
}

func TestDeleteEntity(t *testing.T) {
	db := testDB(t)
	g, err := NewGraph(db)
	if err != nil {
		t.Fatal(err)
	}

	e, _ := g.UpsertEntity("Alice", EntityPerson, nil)
	e2, _ := g.UpsertEntity("Bob", EntityPerson, nil)
	g.UpsertRelation(e.ID, e2.ID, "knows")

	if err := g.DeleteEntity(e.ID); err != nil {
		t.Fatal(err)
	}

	results, _ := g.Query("Alice", 10)
	if len(results) != 0 {
		t.Error("Alice should be deleted")
	}

	// Relation should also be deleted
	rels, _ := g.GetRelations(e2.ID)
	if len(rels) != 0 {
		t.Error("relation should be deleted when entity is deleted")
	}
}

func TestApplyDecay(t *testing.T) {
	db := testDB(t)
	g, err := NewGraph(db)
	if err != nil {
		t.Fatal(err)
	}

	e, _ := g.UpsertEntity("OldMemory", EntityFact, nil)

	// Manually set last_seen to 30 days ago
	db.Exec(`UPDATE kg_entities SET last_seen = datetime('now', '-30 days') WHERE id = ?`, e.ID)

	g.applyDecay()

	// Check that strength decreased
	var strength float64
	db.QueryRow(`SELECT strength FROM kg_entities WHERE id = ?`, e.ID).Scan(&strength)
	if strength >= 1.0 {
		t.Errorf("expected decayed strength, got %f", strength)
	}
}

func TestEmptyNameRejected(t *testing.T) {
	db := testDB(t)
	g, err := NewGraph(db)
	if err != nil {
		t.Fatal(err)
	}

	_, err = g.UpsertEntity("", EntityPerson, nil)
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
