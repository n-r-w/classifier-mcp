package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/samber/mo"

	"github.com/n-r-w/classifier-mcp/internal/domain"
)

// parseCommand validates shared semantic rules before assembling the use-case command.
func parseCommand(input classifyInput) (Command, error) {
	if strings.TrimSpace(input.Task) == "" {
		return Command{}, errors.New("invalid arguments: task must not be blank")
	}
	objects := make([]Object, 0, len(input.Objects))
	// IDs are scoped to one call; input order determines the output association.
	ids := make(map[string]struct{}, len(input.Objects))
	for i, object := range input.Objects {
		if _, exists := ids[object.ID]; exists {
			return Command{}, fmt.Errorf("invalid arguments: objects[%d].id duplicates an earlier id", i)
		}
		ids[object.ID] = struct{}{}
		objects = append(objects, Object{ID: object.ID, Source: mapSource(object.Source)})
	}
	questions := make(map[string]domain.Question, len(input.Questions))
	for id, question := range input.Questions {
		if err := question.validate(); err != nil {
			return Command{}, fmt.Errorf("invalid arguments: questions[%q]: %w", id, err)
		}
		switch question.Type {
		case domain.ChoiceKind:
			var criteria map[string]any
			if err := decodeJSON(question.Criteria.OrEmpty(), &criteria); err != nil {
				return Command{}, fmt.Errorf("criteria: %w", err)
			}
			questions[id] = mo.NewEither3Arg1[domain.ChoiceQuestion, domain.NoulQuestion, domain.ScoreQuestion](
				domain.ChoiceQuestion{Instructions: question.Instructions, Criteria: criteria},
			)
		case domain.NoulKind:
			var criteria map[string]any
			if question.Criteria.IsSome() {
				if err := decodeJSON(question.Criteria.OrEmpty(), &criteria); err != nil {
					return Command{}, fmt.Errorf("criteria: %w", err)
				}
			}
			questions[id] = mo.NewEither3Arg2[domain.ChoiceQuestion, domain.NoulQuestion, domain.ScoreQuestion](
				domain.NoulQuestion{
					Instructions: question.Instructions,
					Criteria:     mo.TupleToOption(criteria, question.Criteria.IsSome()),
				},
			)
		case domain.ScoreKind:
			var criteria []any
			if err := decodeJSON(question.Criteria.OrEmpty(), &criteria); err != nil {
				return Command{}, fmt.Errorf("criteria: %w", err)
			}
			questions[id] = mo.NewEither3Arg3[domain.ChoiceQuestion, domain.NoulQuestion, domain.ScoreQuestion](
				domain.ScoreQuestion{Instructions: question.Instructions, Criteria: criteria},
			)
		}
	}
	return Command{
		Objects:   objects,
		Task:      input.Task,
		Questions: questions,
		Full:      input.ResultMode.OrElse("compact") == "full",
	}, nil
}

// decodeJSON preserves numeric tokens when caller guidance is decoded into structured values.
func decodeJSON(encoded []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	return decoder.Decode(target)
}
