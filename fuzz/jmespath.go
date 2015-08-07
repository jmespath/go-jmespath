package jmespath

import "github.com/jmespath/go-jmespath"

func Fuzz(data []byte) int {
	p := jmespath.NewParser()
	_, err := p.Parse(string(data))
	if err != nil {
		return 1
	}
	return 0
}
