package models

const (
	ProviderDeepSeek ModelProvider = "deepseek"

	DeepSeekV4Flash ModelID = "deepseek-v4-flash"
	DeepSeekV4Pro   ModelID = "deepseek-v4-pro"
)

var DeepSeekModels = map[ModelID]Model{
	DeepSeekV4Flash: {
		ID:                  DeepSeekV4Flash,
		Name:                "DeepSeek V4 Flash",
		Provider:            ProviderDeepSeek,
		APIModel:            "deepseek-v4-flash",
		CostPer1MIn:         0,
		CostPer1MInCached:   0,
		CostPer1MOutCached:  0,
		CostPer1MOut:        0,
		ContextWindow:       128_000,
		DefaultMaxTokens:    20_000,
		SupportsAttachments: false,
	},
	DeepSeekV4Pro: {
		ID:                  DeepSeekV4Pro,
		Name:                "DeepSeek V4 Pro",
		Provider:            ProviderDeepSeek,
		APIModel:            "deepseek-v4-pro",
		CostPer1MIn:         0,
		CostPer1MInCached:   0,
		CostPer1MOutCached:  0,
		CostPer1MOut:        0,
		ContextWindow:       128_000,
		DefaultMaxTokens:    20_000,
		SupportsAttachments: false,
	},
}
