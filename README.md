# tokencalc

`tokencalc` 是一个面向 Go 的 LLM token 统计与估算库，用来处理多种大模型请求/响应协议下的：

- 文本 token 计数
- prompt / completion 提取
- 上游 usage 归一化
- 本地兜底估算
- 流式响应聚合
- 自定义协议扩展
- 批量统计与批量估算

它不负责网关转发、扣费、落库或业务鉴权，而是专注做一件事：把不同协议下的 token 统计能力统一起来。

## 安装

```bash
go get github.com/xy200303/tokencalc
```

## 当前支持

支持的协议：

- `openai_chat`
- `openai_responses`
- `anthropic_messages`
- `gemini_contents`

支持的能力：

- 根据模型名解析 encoding
- 从请求体或响应体自动识别模型
- 优先读取上游返回的 usage
- usage 不完整时与本地估算合并
- 无 usage 时做本地 prompt / completion 估算
- 内置流式响应收集器
- 自定义 estimator 和 stream collector 注册
- 批量 `CountTexts` 与 `EstimateBatch`

模型识别优先级：

1. `UpstreamModel`
2. `RequestModel`
3. 请求体中的模型字段
4. 响应体中的模型字段

## 快速开始

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
		Protocol: tokencalc.ProtocolOpenAIChat,
		RequestBody: []byte(`{
			"model":"gpt-4o-mini",
			"messages":[
				{"role":"system","content":"You are helpful."},
				{"role":"user","content":"Count to three."}
			]
		}`),
		ResponseBody: []byte(`{
			"choices":[
				{"message":{"role":"assistant","content":"One, two, three."}}
			]
		}`),
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("usage=%+v\n", result.Usage)
	fmt.Printf("source=%s model=%s encoding=%s\n", result.Source, result.ResolvedModel, result.Encoding)
	fmt.Printf("note=%s\n", result.Note)
}
```

## 核心 API

主要入口：

- `tokencalc.New(options ...Option) BatchService`
- `service.CountText(model, text)`
- `service.CountTexts(requests)`
- `service.Estimate(req)`
- `service.EstimateBatch(requests)`
- `tokencalc.NewStreamCollector(protocol)`
- `tokencalc.NewStreamingCounter(req, options...)`

常用类型：

```go
type EstimateRequest struct {
	Protocol      Protocol
	RequestModel  string
	UpstreamModel string
	IsStream      bool
	RequestBody   []byte
	ResponseBody  []byte
	ReportedUsage *Usage
}

type EstimateResult struct {
	Usage             Usage
	ResolvedModel     string
	Source            EstimateSource
	Encoding          string
	Supported         bool
	Note              string
	PromptTextLen     int
	CompletionTextLen int
}

type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}
```

`EstimateResult.Source` 取值：

- `reported_usage`: 直接使用上游 usage
- `local_estimate`: 本地从文本估算
- `merged`: 上游 usage 与本地估算合并
- `unsupported`: 当前协议或载荷无法处理

## 使用示例

### 1. 统计一段纯文本的 token

如果你已经拿到了纯文本，不需要构造请求协议，直接用 `CountText`：

```go
service := tokencalc.New()

count, encoding, err := service.CountText("gpt-4o-mini", "你好，帮我总结一下这段内容。")
if err != nil {
	log.Fatal(err)
}

fmt.Println("count:", count)
fmt.Println("encoding:", encoding)
```

### 2. 只统计请求 token

如果你只关心请求消耗，不关心响应，可以只传 `RequestBody`：

```go
service := tokencalc.New()

result, err := service.Estimate(tokencalc.EstimateRequest{
	Protocol: tokencalc.ProtocolOpenAIChat,
	RequestBody: []byte(`{
		"model":"gpt-4o-mini",
		"messages":[
			{"role":"system","content":"You are helpful."},
			{"role":"user","content":"Count to three."}
		]
	}`),
})
if err != nil {
	log.Fatal(err)
}

fmt.Println("prompt_tokens:", result.Usage.PromptTokens)
fmt.Println("completion_tokens:", result.Usage.CompletionTokens) // 0
fmt.Println("total_tokens:", result.Usage.TotalTokens)
fmt.Println("source:", result.Source)
```

### 3. OpenAI Chat 请求 + 响应估算

```go
service := tokencalc.New()

result, err := service.Estimate(tokencalc.EstimateRequest{
	Protocol: tokencalc.ProtocolOpenAIChat,
	RequestBody: []byte(`{
		"model":"gpt-4o-mini",
		"messages":[
			{"role":"system","content":"You are helpful."},
			{"role":"user","content":"Count to three."}
		]
	}`),
	ResponseBody: []byte(`{
		"choices":[
			{"message":{"role":"assistant","content":"One, two, three."}}
		]
	}`),
})
if err != nil {
	log.Fatal(err)
}

fmt.Printf("%+v\n", result.Usage)
```

### 4. 响应里已经带 usage，直接归一化返回

```go
service := tokencalc.New()

result, err := service.Estimate(tokencalc.EstimateRequest{
	Protocol:     tokencalc.ProtocolOpenAIChat,
	RequestModel: "gpt-4o-mini",
	RequestBody:  []byte(`{"messages":[{"role":"user","content":"hello"}]}`),
	ResponseBody: []byte(`{
		"usage":{"prompt_tokens":111,"completion_tokens":222,"total_tokens":333},
		"choices":[{"message":{"role":"assistant","content":"hi"}}]
	}`),
})
if err != nil {
	log.Fatal(err)
}

fmt.Println(result.Source) // reported_usage
fmt.Printf("%+v\n", result.Usage)
```

### 5. OpenAI Responses 协议估算

```go
service := tokencalc.New()

result, err := service.Estimate(tokencalc.EstimateRequest{
	Protocol:     tokencalc.ProtocolOpenAIResponses,
	RequestModel: "gpt-4.1-mini",
	RequestBody: []byte(`{
		"input":[
			{
				"role":"user",
				"content":[{"type":"input_text","text":"Explain gravity in one sentence."}]
			}
		]
	}`),
	ResponseBody: []byte(`{
		"output":[
			{
				"content":[{"type":"output_text","text":"Gravity pulls objects together."}]
			}
		]
	}`),
})
if err != nil {
	log.Fatal(err)
}

fmt.Printf("%+v\n", result.Usage)
```

### 6. 调用方已知部分 usage，和本地估算合并

比如上游只返回了 `prompt_tokens`，你希望把剩余字段补齐：

```go
service := tokencalc.New()

result, err := service.Estimate(tokencalc.EstimateRequest{
	Protocol:     tokencalc.ProtocolOpenAIResponses,
	RequestModel: "gpt-4.1-mini",
	RequestBody: []byte(`{
		"input":[
			{
				"role":"user",
				"content":[{"type":"input_text","text":"Explain gravity in one sentence."}]
			}
		]
	}`),
	ResponseBody: []byte(`{
		"output":[
			{
				"content":[{"type":"output_text","text":"Gravity pulls objects together."}]
			}
		]
	}`),
	ReportedUsage: &tokencalc.Usage{
		PromptTokens: 50,
	},
})
if err != nil {
	log.Fatal(err)
}

fmt.Println(result.Source) // merged
fmt.Printf("%+v\n", result.Usage)
```

### 7. Anthropic Messages 协议估算

Anthropic 请求里如果包含图片等非文本内容，会按占位策略追加估算 token：

```go
service := tokencalc.New()

result, err := service.Estimate(tokencalc.EstimateRequest{
	Protocol:     tokencalc.ProtocolAnthropic,
	RequestModel: "claude-3-5-sonnet",
	RequestBody: []byte(`{
		"system":"You are Claude.",
		"messages":[
			{"role":"user","content":[{"type":"text","text":"Say hi."}]}
		]
	}`),
	ResponseBody: []byte(`{
		"content":[{"type":"text","text":"Hi there."}]
	}`),
})
if err != nil {
	log.Fatal(err)
}

fmt.Printf("%+v\n", result.Usage)
fmt.Println(result.Note)
```

### 8. Gemini Contents 协议估算

```go
service := tokencalc.New()

result, err := service.Estimate(tokencalc.EstimateRequest{
	Protocol:     tokencalc.ProtocolGemini,
	RequestModel: "gemini-2.0-flash",
	RequestBody: []byte(`{
		"contents":[
			{
				"role":"user",
				"parts":[{"text":"Summarize stars."}]
			}
		],
		"systemInstruction":{
			"parts":[{"text":"Keep it short."}]
		}
	}`),
	ResponseBody: []byte(`{
		"candidates":[
			{
				"content":{
					"parts":[{"text":"Stars are hot balls of gas."}]
				}
			}
		]
	}`),
})
if err != nil {
	log.Fatal(err)
}

fmt.Printf("%+v\n", result.Usage)
```

### 9. 流式响应统计

推荐优先使用 `StreamingCounter`，因为它同时覆盖两种需求：

- 一边接收 chunk，一边持续拿累计 token
- 流结束后拿最终结果

```go
counter, err := tokencalc.NewStreamingCounter(tokencalc.EstimateRequest{
	Protocol:     tokencalc.ProtocolOpenAIChat,
	RequestModel: "gpt-4o-mini",
	RequestBody: []byte(`{
		"messages":[{"role":"user","content":"Count to three."}]
	}`),
})
if err != nil {
	log.Fatal(err)
}

chunks := [][]byte{
	[]byte("data: {\"choices\":[{\"delta\":{\"content\":\"One\"}}]}\n\n"),
	[]byte("data: {\"choices\":[{\"delta\":{\"content\":\"two\"}}]}\n\n"),
	[]byte("data: [DONE]\n\n"),
}

for _, chunk := range chunks {
	update, err := counter.AddChunk(chunk)
	if err != nil {
		log.Fatal(err)
	}

	if !update.Updated {
		continue
	}

	fmt.Printf("累计 usage=%+v\n", update.Result.Usage)
	fmt.Printf("本次增量 delta=%+v\n", update.Delta)
}

final, err := counter.FinalResult()
if err != nil {
	log.Fatal(err)
}

fmt.Printf("最终 usage=%+v\n", final.Result.Usage)

if err := counter.Clear(); err != nil {
	log.Fatal(err)
}

if err := counter.Reset(tokencalc.EstimateRequest{
	Protocol:     tokencalc.ProtocolOpenAIChat,
	RequestModel: "gpt-4o-mini",
	RequestBody: []byte(`{
		"messages":[{"role":"user","content":"Start a new conversation."}]
	}`),
}); err != nil {
	log.Fatal(err)
}

if err := counter.ResetRequestBody([]byte(`{
	"messages":[{"role":"user","content":"Reuse the same config, but start another dialog."}]
}`)); err != nil {
	log.Fatal(err)
}
```

`StreamingCounter` 的行为是：

- `AddChunk` 每次喂入原始 chunk
- 如果这次 chunk 还不足以组成完整事件，会先缓存，返回 `Updated=false`
- 一旦已经能形成新的有效流式结果，就返回最新累计值
- `update.Delta` 表示相对上一次成功结果的增量
- `FinalResult()` 用来在流结束时做最终校验；如果最后还残留半条坏数据，会返回错误
- `Clear()` 会清空当前会话的 chunk 缓冲和累计结果，但保留当前请求配置
- `Reset(req)` 会在复用同一个 counter 的同时切换到新的请求上下文，适合连续统计多个对话
- `ResetRequestBody(body)` 会复用当前协议、模型和其它配置，只替换请求体，适合同类对话连续统计

如果某些模型是在最后一个 chunk 才返回总的 `usage`，当前实现也会自动同步：

- 中间没有 `usage` 时，`Result.Source` 会是 `local_estimate`
- 一旦某个新 chunk 里出现完整 `usage`，下一次 `AddChunk` 会自动切到 `reported_usage`
- 如果上游只返回了部分 usage，则会自动走 `merged`

如果你只需要底层的流式聚合能力、不需要中间 token 统计，也可以继续使用 `NewStreamCollector(...)` 自己收集 chunk，最后再配合 `Estimate(...)` 计算。

### 10. 只统计单个流式 chunk

如果你不是想统计整段流，而是只想看某一个 chunk 本身的 token 结果，可以直接把这一段 chunk 当成 `ResponseBody` 传入：

```go
service := tokencalc.New()

result, err := service.Estimate(tokencalc.EstimateRequest{
	Protocol:     tokencalc.ProtocolOpenAIChat,
	RequestModel: "gpt-4o-mini",
	ResponseBody: []byte("data: {\"choices\":[{\"delta\":{\"content\":\"One\"}}]}\n\n"),
	IsStream:     true,
})
if err != nil {
	log.Fatal(err)
}

fmt.Println("prompt_tokens:", result.Usage.PromptTokens)         // 0
fmt.Println("completion_tokens:", result.Usage.CompletionTokens) // 该 chunk 的增量 token
fmt.Println("total_tokens:", result.Usage.TotalTokens)
```

注意：

- 这种写法适合“一个完整事件”的 chunk，比如一条完整 SSE `data: {...}\n\n`
- 如果你传入 `RequestBody`，那么 prompt 也会一起参与估算；按 chunk 做增量统计时通常不建议传
- 如果拿到的是被网络截断的半条数据，而不是完整事件，需要先自行缓冲或交给 collector 聚合

### 11. 批量统计文本

```go
service := tokencalc.New()

results := service.CountTexts([]tokencalc.CountTextRequest{
	{Model: "gpt-4o-mini", Text: "hello"},
	{Model: "claude-3-5-sonnet", Text: "world"},
	{Model: "gemini-2.0-flash", Text: "batch example"},
})

for _, item := range results {
	if item.Error != nil {
		log.Fatal(item.Error)
	}
	fmt.Printf("count=%d encoding=%s\n", item.Count, item.Encoding)
}
```

### 12. 批量估算请求

```go
service := tokencalc.New()

results := service.EstimateBatch([]tokencalc.EstimateRequest{
	{
		Protocol: tokencalc.ProtocolOpenAIChat,
		RequestBody: []byte(`{
			"model":"gpt-4o-mini",
			"messages":[{"role":"user","content":"Hello"}]
		}`),
		ResponseBody: []byte(`{
			"choices":[{"message":{"role":"assistant","content":"Hi!"}}]
		}`),
	},
	{
		Protocol:     tokencalc.ProtocolGemini,
		RequestModel: "gemini-2.0-flash",
		RequestBody: []byte(`{
			"contents":[{"role":"user","parts":[{"text":"Summarize stars."}]}]
		}`),
		ResponseBody: []byte(`{
			"candidates":[{"content":{"parts":[{"text":"Stars are hot balls of gas."}]}}]
		}`),
	},
})

for _, item := range results {
	if item.Error != nil {
		log.Fatal(item.Error)
	}
	fmt.Printf("source=%s usage=%+v\n", item.Result.Source, item.Result.Usage)
}
```

### 13. 使用顶层便捷函数

如果你不想自己持有 service，也可以直接用包级函数：

```go
count, encoding, err := tokencalc.CountText("gpt-4o-mini", "hello")
if err != nil {
	log.Fatal(err)
}

result, err := tokencalc.Estimate(tokencalc.EstimateRequest{
	Protocol: tokencalc.ProtocolOpenAIChat,
	RequestBody: []byte(`{
		"model":"gpt-4o-mini",
		"messages":[{"role":"user","content":"Hello"}]
	}`),
	ResponseBody: []byte(`{
		"choices":[{"message":{"role":"assistant","content":"Hi!"}}]
	}`),
})
if err != nil {
	log.Fatal(err)
}

fmt.Println(count, encoding)
fmt.Printf("%+v\n", result.Usage)
```

### 14. 注册自定义协议 estimator

如果你的协议不在内置支持范围内，可以注入自己的 estimator：

```go
type myEstimator struct{}

func (myEstimator) ExtractPrompt(body []byte) (tokencalc.ExtractResult, error) {
	return tokencalc.ExtractResult{
		Text:      "ping",
		Supported: true,
	}, nil
}

func (myEstimator) ExtractCompletion(body []byte, isStream bool) (tokencalc.ExtractResult, error) {
	return tokencalc.ExtractResult{
		Text:      "pong",
		Supported: true,
	}, nil
}

func (myEstimator) ExtractReportedUsage(body []byte, isStream bool) (tokencalc.ReportedUsageResult, error) {
	return tokencalc.ReportedUsageResult{}, nil
}

service := tokencalc.New(
	tokencalc.WithEstimator("custom_echo", myEstimator{}),
)

result, err := service.Estimate(tokencalc.EstimateRequest{
	Protocol:      "custom_echo",
	UpstreamModel: "gpt-4o-mini",
})
if err != nil {
	log.Fatal(err)
}

fmt.Printf("%+v\n", result.Usage)
```

### 15. 注册自定义流式收集器

```go
type myCollector struct {
	body []byte
}

func (c *myCollector) AddChunk(part []byte) error {
	c.body = append(c.body, part...)
	return nil
}

func (c *myCollector) FinalBody() []byte {
	return append([]byte(nil), c.body...)
}

collector, err := tokencalc.NewStreamCollectorWithOptions(
	"custom_stream",
	tokencalc.WithStreamCollectorFactory("custom_stream", func() tokencalc.StreamCollector {
		return &myCollector{}
	}),
)
if err != nil {
	log.Fatal(err)
}

_ = collector
```

## Placeholder 策略

对于图片、音频、文件等非文本内容，当前版本采用占位估算策略。

默认值：

- `ImageTokenCost: 256`
- `AudioTokenCost: 128`
- `FileTokenCost: 64`

可以在创建 service 时覆盖：

```go
service := tokencalc.New(
	tokencalc.WithPlaceholderPolicy(tokencalc.PlaceholderPolicy{
		ImageTokenCost: 512,
		AudioTokenCost: 256,
		FileTokenCost:  128,
	}),
)
```

## 性能测试

基准测试命令：

```bash
go test -run '^$' -bench . -benchmem .
```

以下结果采集于 `2026-05-22`，环境如下：

- OS: `windows`
- Arch: `amd64`
- CPU: `AMD Ryzen 7 H 255 w/ Radeon 780M`
- Go 包: `github.com/xy200303/tokencalc`

| Benchmark | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| `BenchmarkCountText` | 113603 | 26144 | 374 |
| `BenchmarkCountTextsBatch8` | 847566 | 209472 | 2993 |
| `BenchmarkEstimateOpenAIChatLocal` | 34217 | 10480 | 167 |
| `BenchmarkEstimateOpenAIChatPromptOnly` | 19666 | 5856 | 97 |
| `BenchmarkEstimateOpenAIChatReportedUsage` | 10435 | 5128 | 81 |
| `BenchmarkEstimateOpenAIChatStream` | 42481 | 19024 | 222 |
| `BenchmarkEstimateOpenAIChatSingleStreamChunk` | 8854 | 7008 | 45 |
| `BenchmarkEstimateBatchOpenAIChat8` | 335881 | 84992 | 1337 |
| `BenchmarkEstimateOpenAIResponsesLocal` | 34461 | 9912 | 161 |
| `BenchmarkEstimateOpenAIResponsesReportedUsage` | 12253 | 5487 | 92 |
| `BenchmarkEstimateAnthropicLocal` | 36263 | 14712 | 242 |
| `BenchmarkEstimateAnthropicReportedUsage` | 22531 | 11120 | 182 |
| `BenchmarkEstimateGeminiLocal` | 43364 | 17960 | 256 |
| `BenchmarkEstimateGeminiReportedUsage` | 26455 | 12720 | 178 |
| `BenchmarkStreamingCounterOpenAIChatLocal` | 55114 | 18032 | 266 |
| `BenchmarkStreamingCounterOpenAIChatUsageSync` | 44767 | 16592 | 246 |

可以粗略理解为：

- 纯 `CountText` 会更依赖 tokenizer 编码过程
- 只统计请求侧 token 的 `EstimateOpenAIChatPromptOnly` 明显快于完整请求+响应估算
- 已带 usage 的响应路径最快，因为无需完整本地估算
- 单个完整流式 chunk 的直接估算非常轻量，适合做增量统计
- 流式场景会多一层事件聚合开销
- `StreamingCounter` 的 benchmark 是“整段流会话”的端到端成本，不是单个 chunk 的成本
- 经过当前这轮优化，`StreamingCounter` 已改成增量事件处理，避免每个 chunk 都对整段流重新解析
- 相比早期“整段流反复全量重算”的实现，`StreamingCounter` 的耗时和分配都已经下降到更可用的量级
- Anthropic 和 Gemini 的本地估算分配更高，主要因为协议结构和占位符处理更复杂
- 批量接口适合在单个 service 上复用配置和协议实现

注意：benchmark 会受到 Go 版本、CPU、请求体大小和 payload 结构影响，实际业务值请以你的环境复测为准。

## 设计说明

更多设计背景、边界和实现思路见：

- [docs/技术开发.md](docs/技术开发.md)

仓库里也提供了一个最小可运行示例：

- [examples/basic/main.go](examples/basic/main.go)
