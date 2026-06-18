package runtimeapply

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

type DiffKind string

const (
	DiffUnsupported DiffKind = "unsupported"
	DiffNoop        DiffKind = "noop"
	DiffRoutingOnly DiffKind = "routing_only"
)

type RoutingRuleChange struct {
	RuleTag string
	Rule    map[string]any
}

type Diff struct {
	Kind           DiffKind
	AddedRules     []RoutingRuleChange
	RemovedRuleTag []string
	RemovedRules   []RoutingRuleChange
	Reason         string
}

func ClassifyXrayConfigDiff(current, candidate []byte) (Diff, error) {
	currentDoc, err := parseJSONDocument(current)
	if err != nil {
		return Diff{}, fmt.Errorf("parse current xray config: %w", err)
	}
	candidateDoc, err := parseJSONDocument(candidate)
	if err != nil {
		return Diff{}, fmt.Errorf("parse candidate xray config: %w", err)
	}
	if reflect.DeepEqual(currentDoc, candidateDoc) {
		return Diff{Kind: DiffNoop}, nil
	}

	currentRules, ok, err := detachRoutingRules(currentDoc)
	if err != nil {
		return Diff{}, err
	}
	if !ok {
		return unsupported("current routing.rules is missing or invalid"), nil
	}
	candidateRules, ok, err := detachRoutingRules(candidateDoc)
	if err != nil {
		return Diff{}, err
	}
	if !ok {
		return unsupported("candidate routing.rules is missing or invalid"), nil
	}
	if !reflect.DeepEqual(currentDoc, candidateDoc) {
		return unsupported("non-routing config changed"), nil
	}

	diff, err := classifyRoutingRules(currentRules, candidateRules)
	if err != nil {
		return Diff{}, err
	}
	return diff, nil
}

func classifyRoutingRules(currentRules, candidateRules []map[string]any) (Diff, error) {
	currentByTag, err := rulesByTag(currentRules)
	if err != nil {
		return unsupported(err.Error()), nil
	}
	candidateByTag, err := rulesByTag(candidateRules)
	if err != nil {
		return unsupported(err.Error()), nil
	}

	diff := Diff{Kind: DiffRoutingOnly}
	for tag, current := range currentByTag {
		candidate, exists := candidateByTag[tag]
		if !exists {
			diff.RemovedRuleTag = append(diff.RemovedRuleTag, tag)
			diff.RemovedRules = append(diff.RemovedRules, RoutingRuleChange{
				RuleTag: tag,
				Rule:    current,
			})
			continue
		}
		if !reflect.DeepEqual(current, candidate) {
			return unsupported("tagged routing rule changed in place"), nil
		}
	}
	for tag, candidate := range candidateByTag {
		if _, exists := currentByTag[tag]; exists {
			continue
		}
		diff.AddedRules = append(diff.AddedRules, RoutingRuleChange{
			RuleTag: tag,
			Rule:    candidate,
		})
	}
	sort.Strings(diff.RemovedRuleTag)
	sort.Slice(diff.RemovedRules, func(i, j int) bool {
		return diff.RemovedRules[i].RuleTag < diff.RemovedRules[j].RuleTag
	})
	sort.Slice(diff.AddedRules, func(i, j int) bool {
		return diff.AddedRules[i].RuleTag < diff.AddedRules[j].RuleTag
	})
	if len(diff.AddedRules) == 0 && len(diff.RemovedRuleTag) == 0 {
		diff.Kind = DiffNoop
	}
	return diff, nil
}

func parseJSONDocument(data []byte) (map[string]any, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, errors.New("empty JSON document")
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func detachRoutingRules(doc map[string]any) ([]map[string]any, bool, error) {
	routing, ok := doc["routing"].(map[string]any)
	if !ok {
		return nil, false, nil
	}
	rawRules, ok := routing["rules"].([]any)
	if !ok {
		return nil, false, nil
	}
	rules := make([]map[string]any, 0, len(rawRules))
	for _, raw := range rawRules {
		rule, ok := raw.(map[string]any)
		if !ok {
			return nil, false, errors.New("routing rule is not an object")
		}
		rules = append(rules, rule)
	}
	routing["rules"] = []any{}
	return rules, true, nil
}

func rulesByTag(rules []map[string]any) (map[string]map[string]any, error) {
	result := make(map[string]map[string]any, len(rules))
	for _, rule := range rules {
		tag, _ := rule["ruleTag"].(string)
		tag = strings.TrimSpace(tag)
		if tag == "" {
			return nil, errors.New("routing rule without ruleTag")
		}
		if _, exists := result[tag]; exists {
			return nil, fmt.Errorf("duplicate routing ruleTag %q", tag)
		}
		result[tag] = rule
	}
	return result, nil
}

func unsupported(reason string) Diff {
	return Diff{Kind: DiffUnsupported, Reason: reason}
}
