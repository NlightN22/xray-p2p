package main

import (
	"go/token"

	"golang.org/x/tools/go/ssa"
)

func (a *ownershipAnalysis) creationOrigins(value ssa.Value, seen map[ssa.Value]bool) []token.Pos {
	if value == nil || seen[value] || a.traceValues[value] {
		return nil
	}
	a.traceValues[value] = true
	defer delete(a.traceValues, value)
	seen[value] = true
	switch current := value.(type) {
	case *ssa.Call:
		if a.callResultCreatesOwned(&current.Call, 0) {
			return []token.Pos{current.Pos()}
		}
		return a.closureCallOrigins(&current.Call, 0, seen)
	case *ssa.Extract:
		call, ok := current.Tuple.(*ssa.Call)
		if ok && a.callResultCreatesOwned(&call.Call, current.Index) {
			return []token.Pos{call.Pos()}
		}
	case *ssa.ChangeInterface:
		return a.creationOrigins(current.X, seen)
	case *ssa.Convert:
		return a.creationOrigins(current.X, seen)
	case *ssa.Field:
		return a.creationOrigins(current.X, seen)
	case *ssa.MakeInterface:
		return a.creationOrigins(current.X, seen)
	case *ssa.Phi:
		var origins []token.Pos
		for _, edge := range current.Edges {
			origins = append(origins, a.creationOrigins(edge, seen)...)
		}
		return uniquePositions(origins)
	case *ssa.Alloc:
		return a.addressOrigins(current, make(map[ssa.Value]bool))
	case *ssa.FieldAddr:
		return a.addressOrigins(current, make(map[ssa.Value]bool))
	case *ssa.IndexAddr:
		return a.addressOrigins(current, make(map[ssa.Value]bool))
	case *ssa.UnOp:
		if current.Op == token.MUL {
			return a.addressOrigins(current.X, make(map[ssa.Value]bool))
		}
	}
	return nil
}

func (a *ownershipAnalysis) addressOrigins(address ssa.Value, seen map[ssa.Value]bool) []token.Pos {
	if address == nil || seen[address] || a.traceAddresses[address] {
		return nil
	}
	a.traceAddresses[address] = true
	defer delete(a.traceAddresses, address)
	seen[address] = true
	var origins []token.Pos
	if field, ok := address.(*ssa.FieldAddr); ok {
		references := field.X.Referrers()
		if references != nil {
			for _, reference := range *references {
				sibling, ok := reference.(*ssa.FieldAddr)
				if ok && sibling.Field == field.Field {
					origins = append(origins, a.addressOrigins(sibling, seen)...)
				}
			}
		}
	}
	references := address.Referrers()
	if references == nil {
		return nil
	}
	for _, reference := range *references {
		switch current := reference.(type) {
		case *ssa.Store:
			if current.Addr == address {
				origins = append(origins, a.creationOrigins(current.Val, make(map[ssa.Value]bool))...)
			}
		case *ssa.FieldAddr:
			if current.X == address {
				origins = append(origins, a.addressOrigins(current, seen)...)
			}
		case *ssa.IndexAddr:
			if current.X == address {
				origins = append(origins, a.addressOrigins(current, seen)...)
			}
		}
	}
	return uniquePositions(origins)
}

func (a *ownershipAnalysis) closureCallOrigins(
	call *ssa.CallCommon,
	resultIndex int,
	seen map[ssa.Value]bool,
) []token.Pos {
	closure, ok := call.Value.(*ssa.MakeClosure)
	if !ok {
		return nil
	}
	function, ok := closure.Fn.(*ssa.Function)
	if !ok || function.Signature == nil || resultIndex >= function.Signature.Results().Len() {
		return nil
	}
	indexes := make(map[int]bool)
	for _, block := range function.Blocks {
		for _, instruction := range block.Instrs {
			returned, ok := instruction.(*ssa.Return)
			if !ok || resultIndex >= len(returned.Results) {
				continue
			}
			collectFreeVarIndexes(function, returned.Results[resultIndex], make(map[ssa.Value]bool), indexes)
		}
	}
	var origins []token.Pos
	for index := range indexes {
		if index < len(closure.Bindings) {
			origins = append(origins, a.creationOrigins(closure.Bindings[index], seen)...)
		}
	}
	return uniquePositions(origins)
}

func collectFreeVarIndexes(
	function *ssa.Function,
	value ssa.Value,
	seen map[ssa.Value]bool,
	indexes map[int]bool,
) {
	if value == nil || seen[value] {
		return
	}
	seen[value] = true
	switch current := value.(type) {
	case *ssa.FreeVar:
		for index, freeVar := range function.FreeVars {
			if current == freeVar {
				indexes[index] = true
				return
			}
		}
	case *ssa.ChangeInterface:
		collectFreeVarIndexes(function, current.X, seen, indexes)
	case *ssa.Convert:
		collectFreeVarIndexes(function, current.X, seen, indexes)
	case *ssa.Field:
		collectFreeVarIndexes(function, current.X, seen, indexes)
	case *ssa.MakeInterface:
		collectFreeVarIndexes(function, current.X, seen, indexes)
	case *ssa.Phi:
		for _, edge := range current.Edges {
			collectFreeVarIndexes(function, edge, seen, indexes)
		}
	case *ssa.UnOp:
		collectFreeVarIndexes(function, current.X, seen, indexes)
	}
}
