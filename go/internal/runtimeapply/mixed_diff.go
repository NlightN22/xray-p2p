package runtimeapply

import "reflect"

func classifyMixedResourceDiff(current, candidate []byte) (Diff, bool, error) {
	if diff, ok, err := classifyRoutingOutboundDiff(current, candidate); err != nil || ok {
		return diff, ok, err
	}
	return classifyRoutingInboundUsersDiff(current, candidate)
}

func classifyRoutingOutboundDiff(current, candidate []byte) (Diff, bool, error) {
	currentDoc, err := parseJSONDocument(current)
	if err != nil {
		return Diff{}, false, err
	}
	candidateDoc, err := parseJSONDocument(candidate)
	if err != nil {
		return Diff{}, false, err
	}
	currentRules, ok, err := detachRoutingRules(currentDoc)
	if err != nil || !ok {
		return Diff{}, false, err
	}
	candidateRules, ok, err := detachRoutingRules(candidateDoc)
	if err != nil || !ok {
		return Diff{}, false, err
	}
	currentOutbounds, ok, err := detachTopLevelObjectArray(currentDoc, "outbounds")
	if err != nil || !ok {
		return Diff{}, false, err
	}
	candidateOutbounds, ok, err := detachTopLevelObjectArray(candidateDoc, "outbounds")
	if err != nil || !ok {
		return Diff{}, false, err
	}
	if !reflect.DeepEqual(currentDoc, candidateDoc) {
		return Diff{}, false, nil
	}

	routingDiff, err := classifyRoutingRules(currentRules, candidateRules)
	if err != nil || routingDiff.Kind == DiffUnsupported {
		return routingDiff, true, err
	}
	outboundDiff, err := classifyOutbounds(currentOutbounds, candidateOutbounds)
	if err != nil || outboundDiff.Kind == DiffUnsupported {
		return outboundDiff, true, err
	}
	return mergeDiffs(routingDiff, outboundDiff), true, nil
}

func classifyRoutingInboundUsersDiff(current, candidate []byte) (Diff, bool, error) {
	currentDoc, err := parseJSONDocument(current)
	if err != nil {
		return Diff{}, false, err
	}
	candidateDoc, err := parseJSONDocument(candidate)
	if err != nil {
		return Diff{}, false, err
	}
	currentRules, ok, err := detachRoutingRules(currentDoc)
	if err != nil || !ok {
		return Diff{}, false, err
	}
	candidateRules, ok, err := detachRoutingRules(candidateDoc)
	if err != nil || !ok {
		return Diff{}, false, err
	}
	currentInbounds, ok, err := detachTopLevelObjectArray(currentDoc, "inbounds")
	if err != nil || !ok {
		return Diff{}, false, err
	}
	candidateInbounds, ok, err := detachTopLevelObjectArray(candidateDoc, "inbounds")
	if err != nil || !ok {
		return Diff{}, false, err
	}
	userDiff, ok, err := classifyInboundUsers(currentInbounds, candidateInbounds)
	if err != nil || !ok {
		return Diff{}, ok, err
	}
	currentDoc["inbounds"] = currentInbounds
	candidateDoc["inbounds"] = candidateInbounds
	if !reflect.DeepEqual(currentDoc, candidateDoc) {
		return Diff{}, false, nil
	}
	routingDiff, err := classifyRoutingRules(currentRules, candidateRules)
	if err != nil || routingDiff.Kind == DiffUnsupported {
		return routingDiff, true, err
	}
	return mergeDiffs(routingDiff, userDiff), true, nil
}

func mergeDiffs(parts ...Diff) Diff {
	merged := Diff{Kind: DiffNoop}
	kinds := 0
	for _, part := range parts {
		if part.Kind == DiffNoop {
			continue
		}
		kinds++
		merged.AddedRules = append(merged.AddedRules, part.AddedRules...)
		merged.RemovedRuleTag = append(merged.RemovedRuleTag, part.RemovedRuleTag...)
		merged.RemovedRules = append(merged.RemovedRules, part.RemovedRules...)
		merged.AddedOutbounds = append(merged.AddedOutbounds, part.AddedOutbounds...)
		merged.RemovedOutboundTags = append(merged.RemovedOutboundTags, part.RemovedOutboundTags...)
		merged.RemovedOutbounds = append(merged.RemovedOutbounds, part.RemovedOutbounds...)
		merged.AddedInboundUsers = append(merged.AddedInboundUsers, part.AddedInboundUsers...)
		merged.RemovedInboundUsers = append(merged.RemovedInboundUsers, part.RemovedInboundUsers...)
		if merged.Kind == DiffNoop {
			merged.Kind = part.Kind
		}
	}
	if kinds > 1 {
		merged.Kind = DiffMixed
	}
	return merged
}
