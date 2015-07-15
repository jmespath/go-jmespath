package main

import "fmt"
import "os"

import (
	"encoding/json"

	"github.com/jmespath/jmespath.go/jmespath"
	"github.com/kr/pretty"
)

func main() {
	lexer := jmespath.NewLexer()
	parser := jmespath.NewParser()
	interpreter := jmespath.NewInterpreter()

	expression := os.Args[1]
	input := []byte(os.Args[2])
	fmt.Println(expression)
	tokens, err := lexer.Tokenize(expression)
	if err != nil {
		fmt.Println("Error tokenizing expression")
		fmt.Println(err)
		return
	}
	pretty.Print(tokens)
	fmt.Println("")
	parsed, err := parser.Parse(expression)
	if err != nil {
		fmt.Println("Error tokenizing expression")
		fmt.Println(err)
		return
	}
	pretty.Print(parsed)
	fmt.Println("")
	var data interface{}
	json.Unmarshal(input, &data)
	result, err := interpreter.Execute(parsed, data)
	if err != nil {
		fmt.Println("Error executing expression")
		fmt.Println(err)
		return
	}
	fmt.Println(result)
	toJSON, err := json.Marshal(result)
	if err != nil {
		fmt.Println("Error serializing JSON")
		fmt.Println(err)
		return
	}
	fmt.Println(string(toJSON))
}
