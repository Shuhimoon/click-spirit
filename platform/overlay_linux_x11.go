//go:build linux && cgo && !nox11

package platform

/*
#cgo pkg-config: x11 xext xft fontconfig
#include <stdlib.h>
#include <string.h>
#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/Xatom.h>
#include <X11/extensions/shape.h>
#include <X11/Xft/Xft.h>
#include "x11_common.h"

// csOverlay is a borderless, override-redirect (always on top, never managed
// or focused) window. Its bounding shape is a 1-bit mask of the ripple ring,
// the center dot and the label pill, so everything else is see-through even
// without a compositor. Its input shape is empty, so pointer events pass
// straight through to whatever is underneath.
typedef struct {
	Display *dpy;
	int screen;
	Window win;
	Visual *visual;
	Colormap cmap;
	int depth;
	GC gc;
	XftFont *font, *big;
	XftColor white;
	unsigned long blue, ink, halo;
	int sw, sh;
} csOverlay;

static unsigned long csRGB(csOverlay *o, unsigned short r, unsigned short g, unsigned short b) {
	XColor c;
	memset(&c, 0, sizeof c);
	c.red = r << 8;
	c.green = g << 8;
	c.blue = b << 8;
	c.flags = DoRed | DoGreen | DoBlue;
	if (!XAllocColor(o->dpy, o->cmap, &c)) {
		return BlackPixel(o->dpy, o->screen);
	}
	return c.pixel;
}

static csOverlay *csOverlayOpen(void) {
	int ev = 0, er = 0;
	Display *dpy = csXOpen();
	if (dpy == NULL) {
		return NULL;
	}
	csXftNoFontCache(dpy);
	if (!XShapeQueryExtension(dpy, &ev, &er)) {
		XCloseDisplay(dpy);
		return NULL;
	}
	csOverlay *o = calloc(1, sizeof *o);
	o->dpy = dpy;
	o->screen = DefaultScreen(dpy);
	o->visual = DefaultVisual(dpy, o->screen);
	o->cmap = DefaultColormap(dpy, o->screen);
	o->depth = DefaultDepth(dpy, o->screen);
	o->sw = DisplayWidth(dpy, o->screen);
	o->sh = DisplayHeight(dpy, o->screen);
	o->blue = csRGB(o, 0x0A, 0x84, 0xFF);
	o->ink = csRGB(o, 0x1C, 0x1C, 0x1E);
	o->halo = csRGB(o, 0xFF, 0xFF, 0xFF);

	XSetWindowAttributes a;
	memset(&a, 0, sizeof a);
	a.override_redirect = True;
	a.background_pixel = o->ink;
	a.border_pixel = 0;
	a.event_mask = 0;
	a.save_under = True;
	o->win = XCreateWindow(dpy, RootWindow(dpy, o->screen), 0, 0, 1, 1, 0, o->depth, InputOutput,
		o->visual, CWOverrideRedirect | CWBackPixel | CWBorderPixel | CWEventMask | CWSaveUnder, &a);
	XStoreName(dpy, o->win, "click-spirit-overlay");
	Atom type = XInternAtom(dpy, "_NET_WM_WINDOW_TYPE", False);
	Atom notif = XInternAtom(dpy, "_NET_WM_WINDOW_TYPE_NOTIFICATION", False);
	XChangeProperty(dpy, o->win, type, XA_ATOM, 32, PropModeReplace, (unsigned char *)&notif, 1);
	// Empty input region: the overlay never receives clicks or touches.
	XShapeCombineRectangles(dpy, o->win, ShapeInput, 0, 0, NULL, 0, ShapeSet, Unsorted);
	o->gc = XCreateGC(dpy, o->win, 0, NULL);

	o->font = XftFontOpenName(dpy, o->screen, "sans-serif:lang=zh-tw:weight=bold:pixelsize=15");
	if (o->font == NULL) {
		o->font = XftFontOpenName(dpy, o->screen, "sans-serif:pixelsize=15");
	}
	o->big = XftFontOpenName(dpy, o->screen, "sans-serif:weight=bold:pixelsize=84");
	XRenderColor w = {0xFFFF, 0xFFFF, 0xFFFF, 0xFFFF};
	XftColorAllocValue(dpy, o->visual, o->cmap, &w, &o->white);
	XSync(dpy, False);
	return o;
}

static void csOverlayClose(csOverlay *o) {
	if (o == NULL) {
		return;
	}
	if (o->font != NULL) {
		XftFontClose(o->dpy, o->font);
	}
	if (o->big != NULL) {
		XftFontClose(o->dpy, o->big);
	}
	XftColorFree(o->dpy, o->visual, o->cmap, &o->white);
	XFreeGC(o->dpy, o->gc);
	XDestroyWindow(o->dpy, o->win);
	XCloseDisplay(o->dpy);
	free(o);
}

static void csOverlayHide(csOverlay *o) {
	XUnmapWindow(o->dpy, o->win);
	XFlush(o->dpy);
}

static void csFillDisc(Display *d, Drawable p, GC gc, int cx, int cy, int r) {
	if (r <= 0) {
		return;
	}
	XFillArc(d, p, gc, cx - r, cy - r, 2 * r, 2 * r, 0, 360 * 64);
}

static void csFillPill(Display *d, Drawable p, GC gc, int x, int y, int w, int h) {
	int r = h / 2;
	XFillRectangle(d, p, gc, x + r, y, w - 2 * r, h);
	XFillArc(d, p, gc, x, y, h, h, 90 * 64, 180 * 64);
	XFillArc(d, p, gc, x + w - h, y, h, h, 270 * 64, 180 * 64);
}

#define CS_RMAX 34
#define CS_PILL_H 26

// csOverlayFrame draws one animation frame. (x, y) is the clicked screen
// pixel, ring is the current ripple radius, label is UTF-8.
static void csOverlayFrame(csOverlay *o, int x, int y, int ring, const char *label, int raise) {
	Display *d = o->dpy;
	int len = (int)strlen(label);
	XGlyphInfo ext;
	memset(&ext, 0, sizeof ext);
	if (o->font != NULL && len > 0) {
		XftTextExtentsUtf8(d, o->font, (const FcChar8 *)label, len, &ext);
	}
	int pillW = len > 0 ? ext.xOff + 24 : 0;
	int ringBox = 2 * CS_RMAX + 8;
	int w = ringBox > pillW ? ringBox : pillW;
	int h = ringBox + 6 + CS_PILL_H;
	int cx = w / 2, cy = CS_RMAX + 4;
	int pillY = ringBox + 6;
	// Flip the label above the ripple near the bottom edge.
	if (y - cy + h > o->sh) {
		cy = h - CS_RMAX - 4;
		pillY = 0;
	}
	int wx = x - cx, wy = y - cy;

	Pixmap mask = XCreatePixmap(d, o->win, w, h, 1);
	GC mgc = XCreateGC(d, mask, 0, NULL);
	XSetForeground(d, mgc, 0);
	XFillRectangle(d, mask, mgc, 0, 0, w, h);
	XSetForeground(d, mgc, 1);
	csFillDisc(d, mask, mgc, cx, cy, ring + 2);
	XSetForeground(d, mgc, 0);
	csFillDisc(d, mask, mgc, cx, cy, ring - 3);
	XSetForeground(d, mgc, 1);
	csFillDisc(d, mask, mgc, cx, cy, 6);
	if (pillW > 0) {
		csFillPill(d, mask, mgc, (w - pillW) / 2, pillY, pillW, CS_PILL_H);
	}

	Pixmap pix = XCreatePixmap(d, o->win, w, h, o->depth);
	XSetForeground(d, o->gc, o->ink);
	XFillRectangle(d, pix, o->gc, 0, 0, w, h);
	XSetForeground(d, o->gc, o->halo);
	csFillDisc(d, pix, o->gc, cx, cy, ring + 2);
	XSetForeground(d, o->gc, o->blue);
	csFillDisc(d, pix, o->gc, cx, cy, ring + 1);
	XSetForeground(d, o->gc, o->halo);
	csFillDisc(d, pix, o->gc, cx, cy, 6);
	XSetForeground(d, o->gc, o->blue);
	csFillDisc(d, pix, o->gc, cx, cy, 4);
	if (pillW > 0 && o->font != NULL) {
		XftDraw *xd = XftDrawCreate(d, pix, o->visual, o->cmap);
		int tx = (w - pillW) / 2 + 12;
		int ty = pillY + (CS_PILL_H + o->font->ascent - o->font->descent) / 2;
		XftDrawStringUtf8(xd, &o->white, o->font, tx, ty, (const FcChar8 *)label, len);
		XftDrawDestroy(xd);
	}

	XMoveResizeWindow(d, o->win, wx, wy, w, h);
	XShapeCombineMask(d, o->win, ShapeBounding, 0, 0, mask, ShapeSet);
	XShapeCombineRectangles(d, o->win, ShapeInput, 0, 0, NULL, 0, ShapeSet, Unsorted);
	if (raise) {
		XMapRaised(d, o->win);
	}
	XCopyArea(d, pix, o->win, o->gc, 0, 0, w, h, 0, 0);
	XFreeGC(d, mgc);
	XFreePixmap(d, mask);
	XFreePixmap(d, pix);
	while (XPending(d) > 0) {
		XEvent e;
		XNextEvent(d, &e);
	}
	XFlush(d);
}
static void csFillRound(Display *d, Drawable p, GC gc, int x, int y, int w, int h, int r) {
	XFillRectangle(d, p, gc, x + r, y, w - 2 * r, h);
	XFillRectangle(d, p, gc, x, y + r, w, h - 2 * r);
	XFillArc(d, p, gc, x, y, 2 * r, 2 * r, 90 * 64, 90 * 64);
	XFillArc(d, p, gc, x + w - 2 * r, y, 2 * r, 2 * r, 0, 90 * 64);
	XFillArc(d, p, gc, x, y + h - 2 * r, 2 * r, 2 * r, 180 * 64, 90 * 64);
	XFillArc(d, p, gc, x + w - 2 * r, y + h - 2 * r, 2 * r, 2 * r, 270 * 64, 90 * 64);
}

static int csTextW(csOverlay *o, XftFont *f, const char *s) {
	XGlyphInfo g;
	if (f == NULL) return 0;
	XftTextExtentsUtf8(o->dpy, f, (const FcChar8 *)s, (int)strlen(s), &g);
	return g.xOff;
}

// csOverlayCountdown shows the start-delay card near the top of the screen: a big
// number and a caption. Same window, same empty input shape (click-through).
static void csOverlayCountdown(csOverlay *o, const char *num, const char *sub) {
	Display *d = o->dpy;
	XftFont *bf = o->big != NULL ? o->big : o->font;
	int subW = csTextW(o, o->font, sub);
	int w = subW + 48;
	if (w < 220) w = 220;
	int h = 176, r = 24;
	// Upper part of the screen, so it does not cover the app's own countdown.
	int wx = (o->sw - w) / 2, wy = o->sh / 6;

	Pixmap mask = XCreatePixmap(d, o->win, w, h, 1);
	GC mgc = XCreateGC(d, mask, 0, NULL);
	XSetForeground(d, mgc, 0);
	XFillRectangle(d, mask, mgc, 0, 0, w, h);
	XSetForeground(d, mgc, 1);
	csFillRound(d, mask, mgc, 0, 0, w, h, r);

	Pixmap pix = XCreatePixmap(d, o->win, w, h, o->depth);
	XSetForeground(d, o->gc, o->blue);
	XFillRectangle(d, pix, o->gc, 0, 0, w, h);
	XSetForeground(d, o->gc, o->ink);
	csFillRound(d, pix, o->gc, 3, 3, w - 6, h - 6, r - 3);
	XftDraw *xd = XftDrawCreate(d, pix, o->visual, o->cmap);
	if (bf != NULL) {
		int nw = csTextW(o, bf, num);
		XftDrawStringUtf8(xd, &o->white, bf, (w - nw) / 2, 22 + bf->ascent, (const FcChar8 *)num, (int)strlen(num));
	}
	if (o->font != NULL) {
		XftDrawStringUtf8(xd, &o->white, o->font, (w - subW) / 2, h - 26, (const FcChar8 *)sub, (int)strlen(sub));
	}
	XftDrawDestroy(xd);

	XMoveResizeWindow(d, o->win, wx, wy, w, h);
	XShapeCombineMask(d, o->win, ShapeBounding, 0, 0, mask, ShapeSet);
	XShapeCombineRectangles(d, o->win, ShapeInput, 0, 0, NULL, 0, ShapeSet, Unsorted);
	XMapRaised(d, o->win);
	XCopyArea(d, pix, o->win, o->gc, 0, 0, w, h, 0, 0);
	XFreeGC(d, mgc);
	XFreePixmap(d, mask);
	XFreePixmap(d, pix);
	while (XPending(d) > 0) {
		XEvent e;
		XNextEvent(d, &e);
	}
	XFlush(d);
}

// csOverlayInputRects returns how many rectangles make up the input shape
// (0 means fully click-through) and whether the window is mapped.
static int csOverlayInputRects(csOverlay *o, int *mapped) {
	int count = 0, ordering = 0;
	XRectangle *r = XShapeGetRectangles(o->dpy, o->win, ShapeInput, &count, &ordering);
	if (r != NULL) {
		XFree(r);
	}
	XWindowAttributes wa;
	XGetWindowAttributes(o->dpy, o->win, &wa);
	*mapped = wa.map_state == IsViewable;
	return count;
}
*/
import "C"

import (
	"errors"
	"fmt"
	"runtime"
	"time"
	"unsafe"

	"github.com/click-spirit/click-spirit/core"
)

const (
	rippleFrames   = 12
	rippleFrameDur = 30 * time.Millisecond
	rippleHold     = 650 * time.Millisecond
	rippleRMin     = 8
	rippleRMax     = 34
)

// countdownHold keeps a countdown card up a little past one second so the
// next tick replaces it without flicker; a missed hide still clears it.
const countdownHold = 1150 * time.Millisecond

// countdownCaption is the small line under the big countdown number.
const countdownCaption = "秒後開始 · 按「停止」或緊急熱鍵取消"

type clickReq struct {
	x, y  int
	label string
	// countdown > 0 draws the countdown card; 0 hides a visible card;
	// -1 (clicks) draws a ripple.
	countdown int
}

// x11Overlay runs one goroutine that owns its own X connection. ShowClick
// never blocks: a newer click replaces one that has not been drawn yet.
type x11Overlay struct {
	reqs  chan clickReq
	quit  chan struct{}
	done  chan struct{} // closed after the window and connection are gone
	probe chan chan [2]int
}

func newX11ClickOverlay() (core.ClickOverlay, error) {
	ready := make(chan error, 1)
	o := &x11Overlay{reqs: make(chan clickReq, 1), quit: make(chan struct{}), done: make(chan struct{}), probe: make(chan chan [2]int)}
	go o.loop(ready)
	if err := <-ready; err != nil {
		return nil, err
	}
	return o, nil
}

func (o *x11Overlay) ShowClick(x, y int, label string) {
	o.send(clickReq{x: x, y: y, label: label, countdown: -1})
}

// ShowCountdown draws the start-delay card ("3"), or hides it when n <= 0.
func (o *x11Overlay) ShowCountdown(n int) {
	if n < 0 {
		n = 0
	}
	o.send(clickReq{countdown: n})
}

func (o *x11Overlay) send(r clickReq) {
	for {
		select {
		case o.reqs <- r:
			return
		default:
		}
		select {
		case <-o.reqs:
		default:
		}
	}
}

// Close destroys the window, ends the goroutine and waits for both, so no
// half-destroyed overlay window or connection outlives the call.
func (o *x11Overlay) Close() {
	select {
	case <-o.quit:
	default:
		close(o.quit)
	}
	<-o.done
}

func (o *x11Overlay) loop(ready chan<- error) {
	defer close(o.done)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ov := C.csOverlayOpen()
	if ov == nil {
		ready <- errors.New("cannot open X display with the SHAPE extension for the click overlay")
		return
	}
	defer C.csOverlayClose(ov)
	ready <- nil

	var cur *clickReq
	var label *C.char
	defer func() {
		if label != nil {
			C.free(unsafe.Pointer(label))
		}
	}()
	frame := 0
	var hideAt time.Time
	counting := false
	tick := time.NewTicker(rippleFrameDur)
	defer tick.Stop()
	for {
		select {
		case <-o.quit:
			return
		case reply := <-o.probe:
			var mapped C.int
			n := C.csOverlayInputRects(ov, &mapped)
			reply <- [2]int{int(n), int(mapped)}
		case r := <-o.reqs:
			if r.countdown == 0 {
				if counting {
					C.csOverlayHide(ov)
					counting = false
				}
				continue
			}
			if r.countdown > 0 {
				num := C.CString(fmt.Sprint(r.countdown))
				sub := C.CString(countdownCaption)
				C.csOverlayCountdown(ov, num, sub)
				C.free(unsafe.Pointer(num))
				C.free(unsafe.Pointer(sub))
				cur, counting = nil, true
				hideAt = time.Now().Add(countdownHold)
				continue
			}
			counting = false
			cur = &r
			if label != nil {
				C.free(unsafe.Pointer(label))
			}
			label = C.CString(r.label)
			frame = 0
			o.draw(ov, cur, label, frame, true)
			hideAt = time.Now().Add(rippleFrames*rippleFrameDur + rippleHold)
		case now := <-tick.C:
			if counting && !now.Before(hideAt) {
				C.csOverlayHide(ov)
				counting = false
			}
			if cur == nil {
				continue
			}
			if !now.Before(hideAt) {
				C.csOverlayHide(ov)
				cur = nil
				continue
			}
			if frame < rippleFrames {
				frame++
				o.draw(ov, cur, label, frame, false)
			}
		}
	}
}

func (o *x11Overlay) draw(ov *C.csOverlay, r *clickReq, label *C.char, frame int, raise bool) {
	t := float64(frame) / rippleFrames
	ease := 1 - (1-t)*(1-t)
	ring := rippleRMin + int(ease*float64(rippleRMax-rippleRMin))
	raiseFlag := C.int(0)
	if raise {
		raiseFlag = 1
	}
	C.csOverlayFrame(ov, C.int(r.x), C.int(r.y), C.int(ring), label, raiseFlag)
}

// NewClickOverlay returns the run-time click overlay for the active backend.
// X11 gets a click-through, always-on-top shaped window. uinput (Wayland)
// has no portable way to draw above other windows, so it returns
// ErrUnsupported and the app shows the last click inside its own window.
func NewClickOverlay() (core.ClickOverlay, error) {
	if Name() != "x11" {
		return nil, fmt.Errorf("click overlay needs the X11 backend: %w", ErrUnsupported)
	}
	return newX11ClickOverlay()
}

// state reports the input-shape rectangle count and whether the window is
// mapped. Tests use it to prove the overlay is click-through.
func (o *x11Overlay) state() (inputRects int, mapped bool) {
	reply := make(chan [2]int, 1)
	o.probe <- reply
	v := <-reply
	return v[0], v[1] == 1
}
