package core

// Node is a stateful DAG vertex with O(1) snapshot/restore for open-bar immunity.
type Node interface {
	Name() string
	Init(bus *Bus)
	Update()
	SaveState()
	RestoreState()
	// OnConfigChange applies hot config; nodes that do not support config return nil.
	OnConfigChange(cfg any) error
}
