package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mandarl/pennyclaw/internal/knowledge"
	"github.com/mandarl/pennyclaw/internal/skills"
)

// registerKnowledgeSkills adds knowledge graph skills to the registry.
func (a *Agent) registerKnowledgeSkills() {
	if a.graph == nil {
		return
	}

	a.skills.Register(&skills.Skill{
		Name:        "knowledge_add",
		Description: "Add an entity (person, place, concept, preference, project, event, fact) to the knowledge graph. If the entity already exists, it reinforces the memory and merges properties. Use this to remember important information about the user's world.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {
					"type": "string",
					"description": "Name of the entity (e.g., 'Alice', 'Python', 'Birthday Party')"
				},
				"entity_type": {
					"type": "string",
					"enum": ["person", "place", "organization", "concept", "preference", "project", "event", "fact"],
					"description": "Type of entity"
				},
				"properties": {
					"type": "object",
					"description": "Key-value properties (e.g., {\"role\": \"manager\", \"team\": \"engineering\"})"
				}
			},
			"required": ["name", "entity_type"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				Name       string            `json:"name"`
				EntityType string            `json:"entity_type"`
				Properties map[string]string `json:"properties"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			entity, err := a.graph.UpsertEntity(params.Name, knowledge.EntityType(params.EntityType), params.Properties)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("Stored entity '%s' (type: %s, strength: %.0f%%, access count: %d)", entity.Name, entity.Type, entity.Strength*100, entity.AccessCount), nil
		},
	})

	a.skills.Register(&skills.Skill{
		Name:        "knowledge_relate",
		Description: "Create or reinforce a relationship between two entities in the knowledge graph. Both entities must already exist. Use this to connect people, places, concepts, etc.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"from_id": {
					"type": "integer",
					"description": "ID of the source entity"
				},
				"to_id": {
					"type": "integer",
					"description": "ID of the target entity"
				},
				"relation_type": {
					"type": "string",
					"description": "Type of relationship (e.g., 'works_at', 'likes', 'knows', 'located_in', 'part_of')"
				}
			},
			"required": ["from_id", "to_id", "relation_type"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				FromID       int64  `json:"from_id"`
				ToID         int64  `json:"to_id"`
				RelationType string `json:"relation_type"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			rel, err := a.graph.UpsertRelation(params.FromID, params.ToID, params.RelationType)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("Relation '%s' created/reinforced (strength: %.0f%%)", rel.Type, rel.Strength*100), nil
		},
	})

	a.skills.Register(&skills.Skill{
		Name:        "knowledge_query",
		Description: "Search the knowledge graph for entities matching a term. Returns entities ordered by memory strength. Use this to recall what you know about a topic.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"search": {
					"type": "string",
					"description": "Search term to match against entity names"
				},
				"limit": {
					"type": "integer",
					"description": "Maximum number of results (default 20)"
				}
			},
			"required": ["search"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				Search string `json:"search"`
				Limit  int    `json:"limit"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			if params.Limit <= 0 {
				params.Limit = 20
			}
			entities, err := a.graph.Query(params.Search, params.Limit)
			if err != nil {
				return "", err
			}
			if len(entities) == 0 {
				return fmt.Sprintf("No entities found matching '%s'.", params.Search), nil
			}
			var lines []string
			for _, e := range entities {
				line := fmt.Sprintf("- [ID:%d] **%s** (%s) — strength: %.0f%%, accessed %d times", e.ID, e.Name, e.Type, e.Strength*100, e.AccessCount)
				var props map[string]string
				if json.Unmarshal([]byte(e.Properties), &props) == nil && len(props) > 0 {
					var details []string
					for k, v := range props {
						details = append(details, fmt.Sprintf("%s=%s", k, v))
					}
					line += " [" + strings.Join(details, ", ") + "]"
				}
				lines = append(lines, line)
			}
			return fmt.Sprintf("Found %d entities:\n%s", len(entities), strings.Join(lines, "\n")), nil
		},
	})

	a.skills.Register(&skills.Skill{
		Name:        "knowledge_relations",
		Description: "Get all relationships for a specific entity. Shows both incoming and outgoing connections.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"entity_id": {
					"type": "integer",
					"description": "ID of the entity to get relations for"
				}
			},
			"required": ["entity_id"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				EntityID int64 `json:"entity_id"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			relations, err := a.graph.GetRelations(params.EntityID)
			if err != nil {
				return "", err
			}
			if len(relations) == 0 {
				return "No relations found for this entity.", nil
			}
			var lines []string
			for _, r := range relations {
				lines = append(lines, fmt.Sprintf("- %s '%s' %s (strength: %.0f%%)",
					r["direction"], r["type"], r["related_name"], r["strength"]))
			}
			return fmt.Sprintf("%d relations:\n%s", len(relations), strings.Join(lines, "\n")), nil
		},
	})

	a.skills.Register(&skills.Skill{
		Name:        "knowledge_delete",
		Description: "Remove an entity and all its relations from the knowledge graph.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"entity_id": {
					"type": "integer",
					"description": "ID of the entity to delete"
				}
			},
			"required": ["entity_id"]
		}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				EntityID int64 `json:"entity_id"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}
			if err := a.graph.DeleteEntity(params.EntityID); err != nil {
				return "", err
			}
			return fmt.Sprintf("Entity %d and its relations have been deleted.", params.EntityID), nil
		},
	})

	a.skills.Register(&skills.Skill{
		Name:        "knowledge_stats",
		Description: "Get statistics about the knowledge graph — total entities, relations, average memory strength, and decay parameters.",
		Parameters:  json.RawMessage(`{"type": "object", "properties": {}}`),
		Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
			stats := a.graph.Stats()
			data, _ := json.MarshalIndent(stats, "", "  ")
			return string(data), nil
		},
	})
}
