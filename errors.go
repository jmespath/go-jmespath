package jmespath

import (
	"errors"
	"fmt"
)

func notEnoughArgumentsSupplied(name string, count int, minExpected int, variadic bool) error {
	return errors.New(formatNotEnoughArguments(name, count, minExpected, variadic))
}

func tooManyArgumentsSupplied(name string, count int, maxExpected int) error {
	return errors.New(formatTooManyArguments(name, count, maxExpected))
}

func formatNotEnoughArguments(name string, count int, minExpected int, variadic bool) string {

	more := ""
	only := ""

	if variadic {
		more = "or more "
		only = "only "
	}

	report := fmt.Sprintf("%s%d ", only, count)
	if count == 0 {
		report = "none "
	}

	plural := ""
	if minExpected > 1 {
		plural = "s"
	}

	return fmt.Sprintf(
		"invalid arity, the function '%s' expects %d argument%s %sbut %swere supplied",
		name,
		minExpected,
		plural,
		more,
		report)
}

func formatTooManyArguments(name string, count int, maxExpected int) string {
	return fmt.Sprintf(
		"invalid arity, the function '%s' expects at most %d arguments but %d were supplied",
		name,
		maxExpected,
		count)
}
