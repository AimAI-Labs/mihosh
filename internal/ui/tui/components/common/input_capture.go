package common

// InputCapture declares page-owned keyboard/mouse input and global key exceptions.
// Help, Interrupt and Tab describe ownership of '?', Ctrl+C and Tab/Shift+Tab.
type InputCapture struct {
	Active    bool
	Help      bool
	Interrupt bool
	Tab       bool
}
