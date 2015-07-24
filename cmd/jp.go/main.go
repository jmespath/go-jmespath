package main

import "fmt"
import "os"

import (
	"encoding/json"

	"github.com/jmespath/jmespath.go/jmespath"
	"github.com/kr/pretty"
)

func main() {
	parser := jmespath.NewParser()

	expression := os.Args[1]
	input := []byte(os.Args[2])
	fmt.Println(expression)
	parsed, err := parser.Parse(expression)
	if err != nil {
		fmt.Println("Error parsing expression")
		fmt.Println(err)
		return
	}
	pretty.Print(parsed)
	fmt.Println("")
	var data interface{}
	json.Unmarshal(input, &data)
	result, err := jmespath.Search(expression, data)
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
