package platform

// mouseInput matches the Win32 MOUSEINPUT layout under natural Go alignment.
// On 64-bit, dwExtraInfo is 8-byte aligned, so four padding bytes follow time.
// On 32-bit, dwExtraInfo is 4-byte aligned and there is no padding.
type mouseInput struct {
	dx          int32
	dy          int32
	mouseData   uint32
	dwFlags     uint32
	time        uint32
	dwExtraInfo uintptr
}

// input matches the Win32 INPUT layout for a mouse event:
// a uint32 type field, padding to the union alignment, then MOUSEINPUT.
// The resulting size is 40 bytes on 64-bit and 28 bytes on 32-bit.
type input struct {
	typ uint32
	mi  mouseInput
}
