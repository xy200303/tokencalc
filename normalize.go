package tokencalc

func NormalizeUsage(usage Usage) Usage {
	if usage.PromptTokens < 0 {
		usage.PromptTokens = 0
	}
	if usage.CompletionTokens < 0 {
		usage.CompletionTokens = 0
	}
	if usage.TotalTokens < 0 {
		usage.TotalTokens = 0
	}

	sum := usage.PromptTokens + usage.CompletionTokens
	if usage.TotalTokens == 0 && sum > 0 {
		usage.TotalTokens = sum
	}
	if usage.TotalTokens > 0 && usage.TotalTokens < sum {
		usage.TotalTokens = sum
	}

	return usage
}

func MergeUsage(reported Usage, estimated Usage) Usage {
	reported = NormalizeUsage(reported)
	estimated = NormalizeUsage(estimated)

	merged := reported
	if merged.PromptTokens == 0 {
		merged.PromptTokens = estimated.PromptTokens
	}
	if merged.CompletionTokens == 0 {
		merged.CompletionTokens = estimated.CompletionTokens
	}
	if merged.TotalTokens == 0 {
		merged.TotalTokens = merged.PromptTokens + merged.CompletionTokens
	}

	return NormalizeUsage(merged)
}

func normalizeReportedUsage(usage *Usage) Usage {
	if usage == nil {
		return Usage{}
	}
	return NormalizeUsage(*usage)
}

func usageNeedsMerge(usage Usage) bool {
	if !usage.HasAny() {
		return true
	}
	return usage.PromptTokens == 0 || usage.CompletionTokens == 0 || usage.TotalTokens == 0
}
