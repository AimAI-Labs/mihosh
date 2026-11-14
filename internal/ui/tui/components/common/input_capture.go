package common

// InputCapture declares page-owned keyboard/mouse input and global key exceptions.
// Help and Interrupt say whether the page also owns '?' and Ctrl+C.
type InputCapture struct {
	Active    bool
	Help      bool
	Interrupt bool
}
