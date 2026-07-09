package nodes

// RSXNodeConfig is the typed config payload for RSXNode.OnConfigChange.
type RSXNodeConfig struct {
	Length       int
	SignalLength int
	Source       string
}
