package tokencalc

import "testing"

func TestStreamingCounterTracksCumulativeAndDelta(t *testing.T) {
	t.Parallel()

	service := New()
	request := EstimateRequest{
		Protocol:     ProtocolOpenAIChat,
		RequestModel: "gpt-4o-mini",
		RequestBody:  readFixture(t, "testdata/openai_chat/request.json"),
	}

	counter, err := NewStreamingCounter(request)
	if err != nil {
		t.Fatalf("NewStreamingCounter() error = %v", err)
	}

	firstChunk := []byte("data: {\"choices\":[{\"delta\":{\"content\":\"One\"}}]}\n\n")
	secondChunkPart1 := []byte("data: {\"choices\":[{\"delta\":{\"content\":\"tw")
	secondChunkPart2 := []byte("o\"}}]}\n\n")
	doneChunk := []byte("data: [DONE]\n\n")

	wantFirst, err := service.Estimate(EstimateRequest{
		Protocol:     request.Protocol,
		RequestModel: request.RequestModel,
		RequestBody:  request.RequestBody,
		ResponseBody: firstChunk,
		IsStream:     true,
	})
	if err != nil {
		t.Fatalf("Estimate(firstChunk) error = %v", err)
	}

	fullBody := append([]byte(nil), firstChunk...)
	fullBody = append(fullBody, secondChunkPart1...)
	fullBody = append(fullBody, secondChunkPart2...)

	wantSecond, err := service.Estimate(EstimateRequest{
		Protocol:     request.Protocol,
		RequestModel: request.RequestModel,
		RequestBody:  request.RequestBody,
		ResponseBody: fullBody,
		IsStream:     true,
	})
	if err != nil {
		t.Fatalf("Estimate(fullBody) error = %v", err)
	}

	update, err := counter.AddChunk(firstChunk)
	if err != nil {
		t.Fatalf("AddChunk(firstChunk) error = %v", err)
	}
	if !update.Updated {
		t.Fatal("update.Updated = false, want true")
	}
	if update.Result != wantFirst {
		t.Fatalf("update.Result = %+v, want %+v", update.Result, wantFirst)
	}
	if update.Delta != NormalizeUsage(wantFirst.Usage) {
		t.Fatalf("update.Delta = %+v, want %+v", update.Delta, NormalizeUsage(wantFirst.Usage))
	}

	update, err = counter.AddChunk(secondChunkPart1)
	if err != nil {
		t.Fatalf("AddChunk(secondChunkPart1) error = %v", err)
	}
	if update.Updated {
		t.Fatal("update.Updated = true, want false")
	}
	if update.Result != wantFirst {
		t.Fatalf("update.Result = %+v, want %+v", update.Result, wantFirst)
	}
	if update.Delta.HasAny() {
		t.Fatalf("update.Delta = %+v, want zero delta", update.Delta)
	}

	update, err = counter.AddChunk(secondChunkPart2)
	if err != nil {
		t.Fatalf("AddChunk(secondChunkPart2) error = %v", err)
	}
	if !update.Updated {
		t.Fatal("update.Updated = false, want true")
	}
	if update.Result != wantSecond {
		t.Fatalf("update.Result = %+v, want %+v", update.Result, wantSecond)
	}
	if update.Delta != usageDiff(wantSecond.Usage, wantFirst.Usage) {
		t.Fatalf("update.Delta = %+v, want %+v", update.Delta, usageDiff(wantSecond.Usage, wantFirst.Usage))
	}

	update, err = counter.AddChunk(doneChunk)
	if err != nil {
		t.Fatalf("AddChunk(doneChunk) error = %v", err)
	}
	if update.Updated {
		t.Fatal("update.Updated = true, want false")
	}
	if update.Result != wantSecond {
		t.Fatalf("update.Result = %+v, want %+v", update.Result, wantSecond)
	}

	final, err := counter.FinalResult()
	if err != nil {
		t.Fatalf("FinalResult() error = %v", err)
	}
	if final.Updated {
		t.Fatal("final.Updated = true, want false")
	}
	if final.Result != wantSecond {
		t.Fatalf("final.Result = %+v, want %+v", final.Result, wantSecond)
	}

	wantBody := append(fullBody, doneChunk...)
	if got := string(counter.FinalBody()); got != string(wantBody) {
		t.Fatalf("FinalBody() = %q, want %q", got, string(wantBody))
	}
}

func TestStreamingCounterFinalResultReturnsIncompleteError(t *testing.T) {
	t.Parallel()

	counter, err := NewStreamingCounter(EstimateRequest{
		Protocol:     ProtocolOpenAIChat,
		RequestModel: "gpt-4o-mini",
	})
	if err != nil {
		t.Fatalf("NewStreamingCounter() error = %v", err)
	}

	update, err := counter.AddChunk([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"On"))
	if err != nil {
		t.Fatalf("AddChunk() error = %v, want nil for incomplete chunk", err)
	}
	if update.Updated {
		t.Fatal("update.Updated = true, want false")
	}

	_, err = counter.FinalResult()
	if err == nil {
		t.Fatal("FinalResult() error = nil, want non-nil")
	}
}

func TestStreamingCounterSwitchesToReportedUsageWhenStreamUsageArrives(t *testing.T) {
	t.Parallel()

	counter, err := NewStreamingCounter(EstimateRequest{
		Protocol:     ProtocolOpenAIChat,
		RequestModel: "gpt-4o-mini",
		RequestBody:  []byte(`{"messages":[{"role":"user","content":"Count to three."}]}`),
	})
	if err != nil {
		t.Fatalf("NewStreamingCounter() error = %v", err)
	}

	first, err := counter.AddChunk([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"One\"}}]}\n\n"))
	if err != nil {
		t.Fatalf("AddChunk(first) error = %v", err)
	}
	if first.Result.Source != SourceLocalEstimate {
		t.Fatalf("first.Result.Source = %s, want %s", first.Result.Source, SourceLocalEstimate)
	}
	if !first.Result.Usage.HasAny() {
		t.Fatalf("first.Result.Usage = %+v, want local estimate", first.Result.Usage)
	}

	second, err := counter.AddChunk([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"two\"}}]}\n\n"))
	if err != nil {
		t.Fatalf("AddChunk(second) error = %v", err)
	}
	if second.Result.Source != SourceLocalEstimate {
		t.Fatalf("second.Result.Source = %s, want %s", second.Result.Source, SourceLocalEstimate)
	}

	finalUsageChunk := []byte("data: {\"choices\":[{\"delta\":{\"content\":\"three\"}}],\"usage\":{\"prompt_tokens\":90,\"completion_tokens\":12,\"total_tokens\":102}}\n\n")
	third, err := counter.AddChunk(finalUsageChunk)
	if err != nil {
		t.Fatalf("AddChunk(third) error = %v", err)
	}
	if !third.Updated {
		t.Fatal("third.Updated = false, want true")
	}
	if third.Result.Source != SourceReportedUsage {
		t.Fatalf("third.Result.Source = %s, want %s", third.Result.Source, SourceReportedUsage)
	}

	want := Usage{PromptTokens: 90, CompletionTokens: 12, TotalTokens: 102}
	if third.Result.Usage != want {
		t.Fatalf("third.Result.Usage = %+v, want %+v", third.Result.Usage, want)
	}
	if third.Delta.TotalTokens == 0 {
		t.Fatalf("third.Delta = %+v, want usage sync delta", third.Delta)
	}
}

func TestStreamingCounterClearResetsAccumulatedState(t *testing.T) {
	t.Parallel()

	request := EstimateRequest{
		Protocol:     ProtocolOpenAIChat,
		RequestModel: "gpt-4o-mini",
		RequestBody:  []byte(`{"messages":[{"role":"user","content":"Count to three."}]}`),
	}

	counter, err := NewStreamingCounter(request)
	if err != nil {
		t.Fatalf("NewStreamingCounter() error = %v", err)
	}

	chunk := []byte("data: {\"choices\":[{\"delta\":{\"content\":\"One\"}}]}\n\n")

	first, err := counter.AddChunk(chunk)
	if err != nil {
		t.Fatalf("AddChunk(first) error = %v", err)
	}
	if !first.Updated {
		t.Fatal("first.Updated = false, want true")
	}

	if err := counter.Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	if got := string(counter.FinalBody()); got != "" {
		t.Fatalf("FinalBody() = %q, want empty after Clear()", got)
	}

	second, err := counter.AddChunk(chunk)
	if err != nil {
		t.Fatalf("AddChunk(second) error = %v", err)
	}
	if !second.Updated {
		t.Fatal("second.Updated = false, want true")
	}
	if second.Result != first.Result {
		t.Fatalf("second.Result = %+v, want %+v", second.Result, first.Result)
	}
	if second.Delta != first.Delta {
		t.Fatalf("second.Delta = %+v, want %+v", second.Delta, first.Delta)
	}
}

func TestStreamingCounterResetReplacesRequestContext(t *testing.T) {
	t.Parallel()

	service := New()
	initialRequest := EstimateRequest{
		Protocol:     ProtocolOpenAIChat,
		RequestModel: "gpt-4o-mini",
		RequestBody:  []byte(`{"messages":[{"role":"user","content":"Count to three."}]}`),
	}
	resetRequest := EstimateRequest{
		Protocol:     ProtocolOpenAIChat,
		RequestModel: "gpt-4o-mini",
		RequestBody:  []byte(`{"messages":[{"role":"user","content":"Say hello twice."}]}`),
	}

	counter, err := NewStreamingCounter(initialRequest)
	if err != nil {
		t.Fatalf("NewStreamingCounter() error = %v", err)
	}

	chunk := []byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n")

	if _, err := counter.AddChunk(chunk); err != nil {
		t.Fatalf("AddChunk(initial) error = %v", err)
	}

	if err := counter.Reset(resetRequest); err != nil {
		t.Fatalf("Reset() error = %v", err)
	}

	update, err := counter.AddChunk(chunk)
	if err != nil {
		t.Fatalf("AddChunk(after Reset) error = %v", err)
	}

	want, err := service.Estimate(EstimateRequest{
		Protocol:     resetRequest.Protocol,
		RequestModel: resetRequest.RequestModel,
		RequestBody:  resetRequest.RequestBody,
		ResponseBody: chunk,
		IsStream:     true,
	})
	if err != nil {
		t.Fatalf("Estimate(want) error = %v", err)
	}

	if update.Result != want {
		t.Fatalf("update.Result = %+v, want %+v", update.Result, want)
	}
	if update.Delta != NormalizeUsage(want.Usage) {
		t.Fatalf("update.Delta = %+v, want %+v", update.Delta, NormalizeUsage(want.Usage))
	}
}

func TestStreamingCounterResetRequestBodyReusesCurrentConfig(t *testing.T) {
	t.Parallel()

	service := New()
	counter, err := NewStreamingCounter(EstimateRequest{
		Protocol:     ProtocolOpenAIChat,
		RequestModel: "gpt-4o-mini",
		RequestBody:  []byte(`{"messages":[{"role":"user","content":"Count to three."}]}`),
	})
	if err != nil {
		t.Fatalf("NewStreamingCounter() error = %v", err)
	}

	if _, err := counter.AddChunk([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"One\"}}]}\n\n")); err != nil {
		t.Fatalf("AddChunk(initial) error = %v", err)
	}

	newBody := []byte(`{"messages":[{"role":"user","content":"Say hello twice."}]}`)
	if err := counter.ResetRequestBody(newBody); err != nil {
		t.Fatalf("ResetRequestBody() error = %v", err)
	}

	if got := string(counter.FinalBody()); got != "" {
		t.Fatalf("FinalBody() = %q, want empty after ResetRequestBody()", got)
	}

	chunk := []byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n")
	update, err := counter.AddChunk(chunk)
	if err != nil {
		t.Fatalf("AddChunk(after ResetRequestBody) error = %v", err)
	}

	want, err := service.Estimate(EstimateRequest{
		Protocol:     ProtocolOpenAIChat,
		RequestModel: "gpt-4o-mini",
		RequestBody:  newBody,
		ResponseBody: chunk,
		IsStream:     true,
	})
	if err != nil {
		t.Fatalf("Estimate(want) error = %v", err)
	}

	if update.Result != want {
		t.Fatalf("update.Result = %+v, want %+v", update.Result, want)
	}
	if update.Delta != NormalizeUsage(want.Usage) {
		t.Fatalf("update.Delta = %+v, want %+v", update.Delta, NormalizeUsage(want.Usage))
	}
}
