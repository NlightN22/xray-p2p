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
	DiffUnsupported  DiffKind = "unsupported"
	DiffNoop         DiffKind = "noop"
	DiffRoutingOnly  DiffKind = "routing_only"
	DiffInboundOnly  DiffKind = "inbound_only"
	DiffOutboundOnly DiffKind = "outbound_only"
	DiffInboundUsers DiffKind = "inbound_users"
	DiffMixed        DiffKind = "mixed"
)

type RoutingRuleChange struct {
	RuleTag string
	Rule    map[string]any
}

type InboundChange struct {
	Tag     string
	Inbound map[string]any
}

type OutboundChange struct {
	Tag      string
	Outbound map[string]any
}

type InboundUserChange struct {
	InboundTag string
	Email      string
	Password   string
	User       map[string]any
}

type Diff struct {
	Kind                DiffKind
	AddedRules          []RoutingRuleChange
	RemovedRuleTag      []string
	RemovedRules        []RoutingRuleChange
	AddedInbounds       []InboundChange
	RemovedInboundTags  []string
	RemovedInbounds     []InboundChange
	AddedOutbounds      []OutboundChange
	RemovedOutboundTags []string
	RemovedOutbounds    []OutboundChange
	AddedInboundUsers   []InboundUserChange
	RemovedInboundUsers []InboundUserChange
	Reason              string
}

func ClassifyXrayConfigDiff(current, candidate []byte) (Diff, error) {
	diff, done, err := classifyRoutingDiff(current, candidate)
	if err != nil || done {
		return diff, err
	}
	diff, done, err = classifyInboundUsersDiff(current, candidate)
	if err != nil || done {
		return diff, err
	}
	diff, done, err = classifyInboundDiff(current, candidate)
	if err != nil || done {
		return diff, err
	}
	diff, done, err = classifyOutboundDiff(current, candidate)
	if err != nil || done {
		return diff, err
	}
	diff, done, err = classifyMixedResourceDiff(current, candidate)
	if err != nil || done {
		return diff, err
	}
	return unsupported("unsupported config diff"), nil
}

func classifyRoutingDiff(current, candidate []byte) (Diff, bool, error) {
	currentDoc, err := parseJSONDocument(current)
	if err != nil {
		return Diff{}, false, fmt.Errorf("parse current xray config: %w", err)
	}
	candidateDoc, err := parseJSONDocument(candidate)
	if err != nil {
		return Diff{}, false, fmt.Errorf("parse candidate xray config: %w", err)
	}
	if reflect.DeepEqual(currentDoc, candidateDoc) {
		return Diff{Kind: DiffNoop}, true, nil
	}

	currentRules, ok, err := detachRoutingRules(currentDoc)
	if err != nil {
		return Diff{}, false, err
	}
	if !ok {
		return Diff{}, false, nil
	}
	candidateRules, ok, err := detachRoutingRules(candidateDoc)
	if err != nil {
		return Diff{}, false, err
	}
	if !ok {
		return Diff{}, false, nil
	}
	if !reflect.DeepEqual(currentDoc, candidateDoc) {
		return Diff{}, false, nil
	}

	diff, err := classifyRoutingRules(currentRules, candidateRules)
	if err != nil {
		return Diff{}, false, err
	}
	return diff, true, nil
}

func classifyInboundDiff(current, candidate []byte) (Diff, bool, error) {
	currentDoc, err := parseJSONDocument(current)
	if err != nil {
		return Diff{}, false, fmt.Errorf("parse current xray config: %w", err)
	}
	candidateDoc, err := parseJSONDocument(candidate)
	if err != nil {
		return Diff{}, false, fmt.Errorf("parse candidate xray config: %w", err)
	}
	currentInbounds, ok, err := detachTopLevelObjectArray(currentDoc, "inbounds")
	if err != nil {
		return Diff{}, false, err
	}
	if !ok {
		return Diff{}, false, nil
	}
	candidateInbounds, ok, err := detachTopLevelObjectArray(candidateDoc, "inbounds")
	if err != nil {
		return Diff{}, false, err
	}
	if !ok {
		return Diff{}, false, nil
	}
	if !reflect.DeepEqual(currentDoc, candidateDoc) {
		return Diff{}, false, nil
	}
	diff, err := classifyInbounds(currentInbounds, candidateInbounds)
	if err != nil {
		return Diff{}, false, err
	}
	return diff, true, nil
}

func classifyOutboundDiff(current, candidate []byte) (Diff, bool, error) {
	currentDoc, err := parseJSONDocument(current)
	if err != nil {
		return Diff{}, false, fmt.Errorf("parse current xray config: %w", err)
	}
	candidateDoc, err := parseJSONDocument(candidate)
	if err != nil {
		return Diff{}, false, fmt.Errorf("parse candidate xray config: %w", err)
	}
	currentOutbounds, ok, err := detachTopLevelObjectArray(currentDoc, "outbounds")
	if err != nil {
		return Diff{}, false, err
	}
	if !ok {
		return Diff{}, false, nil
	}
	candidateOutbounds, ok, err := detachTopLevelObjectArray(candidateDoc, "outbounds")
	if err != nil {
		return Diff{}, false, err
	}
	if !ok {
		return Diff{}, false, nil
	}
	if !reflect.DeepEqual(currentDoc, candidateDoc) {
		return Diff{}, false, nil
	}
	diff, err := classifyOutbounds(currentOutbounds, candidateOutbounds)
	if err != nil {
		return Diff{}, false, err
	}
	return diff, true, nil
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

func classifyInbounds(currentInbounds, candidateInbounds []map[string]any) (Diff, error) {
	currentByTag, err := objectsByTag(currentInbounds, "tag", "inbound")
	if err != nil {
		return unsupported(err.Error()), nil
	}
	candidateByTag, err := objectsByTag(candidateInbounds, "tag", "inbound")
	if err != nil {
		return unsupported(err.Error()), nil
	}
	diff := Diff{Kind: DiffInboundOnly}
	for tag, current := range currentByTag {
		candidate, exists := candidateByTag[tag]
		if !exists {
			diff.RemovedInboundTags = append(diff.RemovedInboundTags, tag)
			diff.RemovedInbounds = append(diff.RemovedInbounds, InboundChange{
				Tag:     tag,
				Inbound: current,
			})
			continue
		}
		if !reflect.DeepEqual(current, candidate) {
			return unsupported("tagged inbound changed in place"), nil
		}
	}
	for tag, candidate := range candidateByTag {
		if _, exists := currentByTag[tag]; exists {
			continue
		}
		diff.AddedInbounds = append(diff.AddedInbounds, InboundChange{
			Tag:     tag,
			Inbound: candidate,
		})
	}
	sort.Strings(diff.RemovedInboundTags)
	sort.Slice(diff.RemovedInbounds, func(i, j int) bool {
		return diff.RemovedInbounds[i].Tag < diff.RemovedInbounds[j].Tag
	})
	sort.Slice(diff.AddedInbounds, func(i, j int) bool {
		return diff.AddedInbounds[i].Tag < diff.AddedInbounds[j].Tag
	})
	if len(diff.AddedInbounds) == 0 && len(diff.RemovedInboundTags) == 0 {
		diff.Kind = DiffNoop
	}
	return diff, nil
}

func classifyOutbounds(currentOutbounds, candidateOutbounds []map[string]any) (Diff, error) {
	currentByTag, err := objectsByTag(currentOutbounds, "tag", "outbound")
	if err != nil {
		return unsupported(err.Error()), nil
	}
	candidateByTag, err := objectsByTag(candidateOutbounds, "tag", "outbound")
	if err != nil {
		return unsupported(err.Error()), nil
	}
	diff := Diff{Kind: DiffOutboundOnly}
	for tag, current := range currentByTag {
		candidate, exists := candidateByTag[tag]
		if !exists {
			diff.RemovedOutboundTags = append(diff.RemovedOutboundTags, tag)
			diff.RemovedOutbounds = append(diff.RemovedOutbounds, OutboundChange{
				Tag:      tag,
				Outbound: current,
			})
			continue
		}
		if !reflect.DeepEqual(current, candidate) {
			diff.RemovedOutboundTags = append(diff.RemovedOutboundTags, tag)
			diff.RemovedOutbounds = append(diff.RemovedOutbounds, OutboundChange{
				Tag:      tag,
				Outbound: current,
			})
			diff.AddedOutbounds = append(diff.AddedOutbounds, OutboundChange{
				Tag:      tag,
				Outbound: candidate,
			})
		}
	}
	for tag, candidate := range candidateByTag {
		if _, exists := currentByTag[tag]; exists {
			continue
		}
		diff.AddedOutbounds = append(diff.AddedOutbounds, OutboundChange{
			Tag:      tag,
			Outbound: candidate,
		})
	}
	sort.Strings(diff.RemovedOutboundTags)
	sort.Slice(diff.RemovedOutbounds, func(i, j int) bool {
		return diff.RemovedOutbounds[i].Tag < diff.RemovedOutbounds[j].Tag
	})
	sort.Slice(diff.AddedOutbounds, func(i, j int) bool {
		return diff.AddedOutbounds[i].Tag < diff.AddedOutbounds[j].Tag
	})
	if len(diff.AddedOutbounds) == 0 && len(diff.RemovedOutboundTags) == 0 {
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

func detachTopLevelObjectArray(doc map[string]any, key string) ([]map[string]any, bool, error) {
	rawItems, ok := doc[key].([]any)
	if !ok {
		return nil, false, nil
	}
	items := make([]map[string]any, 0, len(rawItems))
	for _, raw := range rawItems {
		item, ok := raw.(map[string]any)
		if !ok {
			return nil, false, fmt.Errorf("%s item is not an object", key)
		}
		items = append(items, item)
	}
	doc[key] = []any{}
	return items, true, nil
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
	return objectsByTag(rules, "ruleTag", "routing rule")
}

func objectsByTag(items []map[string]any, field string, label string) (map[string]map[string]any, error) {
	result := make(map[string]map[string]any, len(items))
	for _, item := range items {
		tag, _ := item[field].(string)
		tag = strings.TrimSpace(tag)
		if tag == "" {
			return nil, fmt.Errorf("%s without %s", label, field)
		}
		if _, exists := result[tag]; exists {
			return nil, fmt.Errorf("duplicate %s %q", field, tag)
		}
		result[tag] = item
	}
	return result, nil
}

func unsupported(reason string) Diff {
	return Diff{Kind: DiffUnsupported, Reason: reason}
}
