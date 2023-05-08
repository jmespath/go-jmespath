package jmespath

import (
	"fmt"
	"strings"
)

type ExpressionEvaluator func(value interface{}) (interface{}, error)

func NewExpressionEvaluator(intrArg interface{}, expArg interface{}) ExpressionEvaluator {
	intr := intrArg.(*treeInterpreter)
	node := expArg.(expRef).ref
	return func(value interface{}) (interface{}, error) {
		return intr.Execute(node, value)
	}
}

func (jp *JMESPath) RegisterFunction(name string, args string, variadic bool, handler func([]interface{}) (interface{}, error)) error {
	hasExpRef := false
	var arguments []argSpec
	for _, arg := range strings.Split(args, ",") {
		var argTypes []jpType
		for _, argType := range strings.Split(arg, "|") {
			switch t := jpType(argType); t {
			case jpExpref:
				hasExpRef = true
				fallthrough
			case jpNumber, jpString, jpArray, jpObject, jpArrayNumber, jpArrayString, jpAny:
				argTypes = append(argTypes, t)
			default:
				return fmt.Errorf("unknown argument type: %s", argType)
			}
		}
		arguments = append(arguments, argSpec{
			types: argTypes,
		})
	}
	if variadic {
		if len(arguments) == 0 {
			return fmt.Errorf("variadic functions require at least one argument")
		}
		arguments[len(arguments)-1].variadic = true
	}
	jp.intr.fCall.functionTable[name] = functionEntry{
		name:      name,
		arguments: arguments,
		handler:   handler,
		hasExpRef: hasExpRef,
	}
	return nil
}
