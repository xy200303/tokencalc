package tokencalc

type PlaceholderPolicy struct {
	ImageTokenCost int
	AudioTokenCost int
	FileTokenCost  int
}

func DefaultPlaceholderPolicy() PlaceholderPolicy {
	return PlaceholderPolicy{
		ImageTokenCost: 256,
		AudioTokenCost: 128,
		FileTokenCost:  64,
	}
}
