package main

import (
	"fmt"
	"log"

	"github.com/xy200303/tokencalc"
)

func main() {
	service := tokencalc.New()

	result, err := service.Estimate(tokencalc.EstimateRequest{
		Protocol: tokencalc.ProtocolOpenAIChat,
		RequestBody: []byte(`{
			"model":"gpt-4o-mini",
			"messages":[{"role":"user","content":"Hello"}]
		}`),
		ResponseBody: []byte(`{"choices":[{"message":{"role":"assistant","content":"Hi!"}}]}`),
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("usage=%+v source=%s model=%s encoding=%s\n", result.Usage, result.Source, result.ResolvedModel, result.Encoding)
}
