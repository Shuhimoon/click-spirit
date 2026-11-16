// Shared helpers for the cgo X11 overlay, picker and bubble files.
#ifndef CLICK_SPIRIT_X11_COMMON_H
#define CLICK_SPIRIT_X11_COMMON_H

#include <X11/Xlib.h>
#include <X11/Xft/Xft.h>
#include <fontconfig/fontconfig.h>
#include <unistd.h>

// csXOpen is XOpenDisplay with the same short retry as csOpen in
// clicker_linux_x11.go. An X server that resets when its last client leaves
// (Xvfb without -noreset) rejects connections made during the reset with
// ECONNRESET. A display that is actually down still returns NULL, after about 200 ms.
static inline Display *csXOpen(void) {
	int i;
	for (i = 0; i < 40; i++) {
		Display *d = XOpenDisplay(NULL);
		if (d != NULL) return d;
		usleep(5000);
	}
	return NULL;
}

// csXftNoFontCache must be called right after XOpenDisplay, before any Xft use.
//
// By default Xft keeps closed fonts in a per-Display cache and frees them only
// from its XCloseDisplay hook. XCloseDisplay runs hooks newest-first, so
// libXrender's hook has already dropped its per-Display info by then; freeing
// the cached glyph sets calls back into libXrender, which re-creates an info
// record keyed by the dying Display* and leaks it. When the next XOpenDisplay
// (e.g. the screen picker right after the countdown overlay) gets the same
// address, libXrender hands it that stale record, and XRenderQueryFormats
// sends a request with a garbage opcode -> random X errors (BadRequest,
// BadLength, BadColor, BadWindow) and the default handler exits the process.
//
// Disabling the cache makes XftFontClose free fonts immediately, while the
// Display is still fully alive, so nothing touches libXrender during close.
static inline void csXftNoFontCache(Display *dpy) {
	FcPattern *p = FcPatternCreate();
	if (p == NULL) return;
	FcPatternAddInteger(p, XFT_MAX_UNREF_FONTS, 0);
	XftDefaultSet(dpy, p); // takes ownership of p
}

#endif
