package common

// InputCapture declares page-owned input and the two global key exceptions.
// Help and Interrupt say whether the page also owns '?' and Ctrl+C.
type InputCapture struct {
	Active    bool
	Help      bool
	Interrupt bool
}
