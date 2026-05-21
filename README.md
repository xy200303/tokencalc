# tokencalc

`tokencalc` is a Go library for counting and estimating LLM token usage across multiple provider payload formats.

It focuses on:

- model-to-encoding resolution
- prompt and completion extraction
- reported usage normalization
- local fallback estimation
- stream body collection

## Install

```bash
go get github.com/xy200303/tokencalc
```

## Quick Start

```go
package main

import (
	"fmt"
	"log"

	"github.com/xy200303/tokencalc"
)

func main() {
	service := tokencalc.New()

	result, err := service.Estimate(tokencalc.EstimateRequest{
		Protocol:     tokencalc.ProtocolOpenAIChat,
		RequestModel: "gpt-4o-mini",
		RequestBody:  []byte(`{"messages":[{"role":"user","content":"Hello"}]}`),
		ResponseBody: []byte(`{"choices":[{"message":{"role":"assistant","content":"Hi!"}}]}`),
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%+v\n", result.Usage)
}
```

More design notes are available in [docs/技术开发.md](docs/技术开发.md).
