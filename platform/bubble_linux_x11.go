//go:build linux && cgo && !nox11

package platform

/*
#cgo pkg-config: x11 xext xft fontconfig
#include <poll.h>
#include <stdlib.h>
#include <string.h>
#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/Xatom.h>
#include <X11/extensions/shape.h>
#include <X11/Xft/Xft.h>
#include "x11_common.h"

// 小浮窗: a small override-redirect window that stays above other windows.
// Collapsed it is a 56 px round button; tapping it expands a pill with three
// buttons. Dragging moves it. Bounding shape = circle or pill, so the rest
// stays see-through even without a compositor.

#define BB_D 56
#define BB_BTN 84
#define BB_H 64

typedef struct {
	Display *dpy;
	int screen, depth, sw, sh;
	Window root, win;
	Visual *visual;
	Colormap cmap;
	GC gc;
	XftFont *font;
	XftColor white, dim;
	unsigned long ink, ring, knob, red, blue, sep;
	int x, y;          // bubble (collapsed) top-left on screen
	int expanded, left; // left: buttons open to the left of the bubble
	int recording, running, mapped;
} csBubble;

static unsigned long bbRGB(csBubble *b, int r, int g, int bl) {
	XColor c;
	memset(&c, 0, sizeof c);
	c.red = r << 8; c.green = g << 8; c.blue = bl << 8;
	c.flags = DoRed | DoGreen | DoBlue;
	if (!XAllocColor(b->dpy, b->cmap, &c)) return BlackPixel(b->dpy, b->screen);
	return c.pixel;
}

static csBubble *bbOpen(void) {
	int ev, er;
	Display *d = csXOpen();
	if (d == NULL) return NULL;
	csXftNoFontCache(d);
	if (!XShapeQueryExtension(d, &ev, &er)) { XCloseDisplay(d); return NULL; }
	csBubble *b = calloc(1, sizeof *b);
	b->dpy = d;
	b->screen = DefaultScreen(d);
	b->root = RootWindow(d, b->screen);
	b->visual = DefaultVisual(d, b->screen);
	b->cmap = DefaultColormap(d, b->screen);
	b->depth = DefaultDepth(d, b->screen);
	b->sw = DisplayWidth(d, b->screen);
	b->sh = DisplayHeight(d, b->screen);
	b->ink = bbRGB(b, 0x1C, 0x1C, 0x1E);
	b->ring = bbRGB(b, 0x63, 0x63, 0x66);
	b->knob = bbRGB(b, 0xF2, 0xF2, 0xF7);
	b->red = bbRGB(b, 0xFF, 0x3B, 0x30);
	b->blue = bbRGB(b, 0x0A, 0x84, 0xFF);
	b->sep = bbRGB(b, 0x3A, 0x3A, 0x3C);
	b->x = b->sw - BB_D - 16;
	b->y = b->sh / 2 - BB_D / 2;

	XSetWindowAttributes a;
	memset(&a, 0, sizeof a);
	a.override_redirect = True;
	a.background_pixel = b->ink;
	a.event_mask = ExposureMask | ButtonPressMask | ButtonReleaseMask | Button1MotionMask;
	b->win = XCreateWindow(d, b->root, b->x, b->y, BB_D, BB_D, 0, b->depth, InputOutput, b->visual,
		CWOverrideRedirect | CWBackPixel | CWEventMask, &a);
	XStoreName(d, b->win, "click-spirit-bubble");
	Atom type = XInternAtom(d, "_NET_WM_WINDOW_TYPE", False);
	Atom util = XInternAtom(d, "_NET_WM_WINDOW_TYPE_UTILITY", False);
	XChangeProperty(d, b->win, type, XA_ATOM, 32, PropModeReplace, (unsigned char *)&util, 1);
	b->gc = XCreateGC(d, b->win, 0, NULL);
	b->font = XftFontOpenName(d, b->screen, "sans-serif:lang=zh-tw:weight=bold:pixelsize=14");
	if (b->font == NULL) b->font = XftFontOpenName(d, b->screen, "sans-serif:pixelsize=14");
	XRenderColor w = {0xFFFF, 0xFFFF, 0xFFFF, 0xFFFF};
	XRenderColor dm = {0x8E8E, 0x8E8E, 0x9393, 0xFFFF};
	XftColorAllocValue(d, b->visual, b->cmap, &w, &b->white);
	XftColorAllocValue(d, b->visual, b->cmap, &dm, &b->dim);
	XSync(d, False);
	return b;
}

static void bbClose(csBubble *b) {
	if (b == NULL) return;
	if (b->font) XftFontClose(b->dpy, b->font);
	XFreeGC(b->dpy, b->gc);
	XDestroyWindow(b->dpy, b->win);
	XCloseDisplay(b->dpy);
	free(b);
}

// Window geometry for the current state.
static void bbGeom(csBubble *b, int *wx, int *wy, int *w, int *h, int *bubbleX) {
	if (!b->expanded) {
		*wx = b->x; *wy = b->y; *w = BB_D; *h = BB_D; *bubbleX = 0;
		return;
	}
	*w = BB_D + 3 * BB_BTN + 12;
	*h = BB_H;
	*wy = b->y - (BB_H - BB_D) / 2;
	b->left = b->x + BB_D / 2 > b->sw / 2;
	if (b->left) { *wx = b->x - (*w - BB_D) + 4; *bubbleX = *w - BB_D - 4; }
	else { *wx = b->x - 4; *bubbleX = 4; }
}

static void bbFillRound(Display *d, Drawable p, GC gc, int x, int y, int w, int h, int r) {
	XFillRectangle(d, p, gc, x + r, y, w - 2 * r, h);
	XFillRectangle(d, p, gc, x, y + r, w, h - 2 * r);
	XFillArc(d, p, gc, x, y, 2 * r, 2 * r, 90 * 64, 90 * 64);
	XFillArc(d, p, gc, x + w - 2 * r, y, 2 * r, 2 * r, 0, 90 * 64);
	XFillArc(d, p, gc, x, y + h - 2 * r, 2 * r, 2 * r, 180 * 64, 90 * 64);
	XFillArc(d, p, gc, x + w - 2 * r, y + h - 2 * r, 2 * r, 2 * r, 270 * 64, 90 * 64);
}

static const char *bbLabel(csBubble *b, int i) {
	switch (i) {
	case 0: return b->recording ? "■ 停止錄製" : "● 開始錄製";
	case 1: return "▶ 開始腳本";
	default: return "■ 停止腳本";
	}
}

// Button i rect inside the expanded window.
static void bbButton(csBubble *b, int w, int i, int *bx) {
	int start = b->left ? 6 : BB_D + 8;
	(void)w;
	*bx = start + i * BB_BTN;
}

static void bbDraw(csBubble *b) {
	Display *d = b->dpy;
	int wx, wy, w, h, bubbleX;
	bbGeom(b, &wx, &wy, &w, &h, &bubbleX);
	Pixmap mask = XCreatePixmap(d, b->win, w, h, 1);
	GC mgc = XCreateGC(d, mask, 0, NULL);
	XSetForeground(d, mgc, 0);
	XFillRectangle(d, mask, mgc, 0, 0, w, h);
	XSetForeground(d, mgc, 1);
	if (b->expanded) bbFillRound(d, mask, mgc, 0, 0, w, h, h / 2);
	else XFillArc(d, mask, mgc, 0, 0, BB_D, BB_D, 0, 360 * 64);

	Pixmap pix = XCreatePixmap(d, b->win, w, h, b->depth);
	XSetForeground(d, b->gc, b->ink);
	XFillRectangle(d, pix, b->gc, 0, 0, w, h);
	int by = (h - BB_D) / 2;
	// AssistiveTouch-style knob: dark disc, grey ring, light centre.
	XSetForeground(d, b->gc, b->ring);
	XFillArc(d, pix, b->gc, bubbleX + 8, by + 8, BB_D - 16, BB_D - 16, 0, 360 * 64);
	XSetForeground(d, b->gc, b->ink);
	XFillArc(d, pix, b->gc, bubbleX + 13, by + 13, BB_D - 26, BB_D - 26, 0, 360 * 64);
	XSetForeground(d, b->gc, b->recording ? b->red : (b->running ? b->blue : b->knob));
	XFillArc(d, pix, b->gc, bubbleX + 18, by + 18, BB_D - 36, BB_D - 36, 0, 360 * 64);
	if (b->expanded && b->font != NULL) {
		XftDraw *xd = XftDrawCreate(d, pix, b->visual, b->cmap);
		for (int i = 0; i < 3; i++) {
			int bx;
			bbButton(b, w, i, &bx);
			if (i > 0) {
				XSetForeground(d, b->gc, b->sep);
				XFillRectangle(d, pix, b->gc, bx, 16, 1, h - 32);
			}
			const char *s = bbLabel(b, i);
			XGlyphInfo g;
			XftTextExtentsUtf8(d, b->font, (const FcChar8 *)s, (int)strlen(s), &g);
			XftColor *col = &b->white;
			if (i == 1 && b->running) col = &b->dim;
			if (i == 2 && !b->running) col = &b->dim;
			int ty = (h + b->font->ascent - b->font->descent) / 2;
			XftDrawStringUtf8(xd, col, b->font, bx + (BB_BTN - g.xOff) / 2, ty, (const FcChar8 *)s, (int)strlen(s));
		}
		XftDrawDestroy(xd);
	}
	XMoveResizeWindow(d, b->win, wx, wy, w, h);
	XShapeCombineMask(d, b->win, ShapeBounding, 0, 0, mask, ShapeSet);
	XCopyArea(d, pix, b->win, b->gc, 0, 0, w, h, 0, 0);
	XFreeGC(d, mgc);
	XFreePixmap(d, mask);
	XFreePixmap(d, pix);
	XFlush(d);
}

static void bbSetVisible(csBubble *b, int on) {
	if (on && !b->mapped) { bbDraw(b); XMapRaised(b->dpy, b->win); bbDraw(b); b->mapped = 1; }
	if (!on && b->mapped) { XUnmapWindow(b->dpy, b->win); b->mapped = 0; b->expanded = 0; }
	XFlush(b->dpy);
}

static void bbRaise(csBubble *b) {
	if (b->mapped) { XRaiseWindow(b->dpy, b->win); XFlush(b->dpy); }
}

// Current window rect: out[0..3] = x, y, w, h; returns mapped.
static int bbRect(csBubble *b, int *out) {
	int wx, wy, w, h, bx;
	bbGeom(b, &wx, &wy, &w, &h, &bx);
	out[0] = wx; out[1] = wy; out[2] = w; out[3] = h;
	return b->mapped;
}

// bbStep handles pending X events. It returns an action: -1 none, 0..2 a
// button, 10 = the rect changed (moved, expanded or collapsed).
static int bbPending(csBubble *b) { return XPending(b->dpy); }

static int bbFd(csBubble *b) { return ConnectionNumber(b->dpy); }

static int bbHandle(csBubble *b, int *pressX, int *pressY, int *startX, int *startY, int *pressed, int *dragged) {
	XEvent ev;
	XNextEvent(b->dpy, &ev);
	switch (ev.type) {
	case Expose:
		if (ev.xexpose.count == 0) bbDraw(b);
		return -1;
	case ButtonPress:
		if (ev.xbutton.button != 1) return -1;
		*pressed = 1; *dragged = 0;
		*pressX = ev.xbutton.x_root; *pressY = ev.xbutton.y_root;
		*startX = b->x; *startY = b->y;
		return -1;
	case MotionNotify: {
		if (!*pressed) return -1;
		while (XCheckTypedWindowEvent(b->dpy, b->win, MotionNotify, &ev)) {}
		int dx = ev.xmotion.x_root - *pressX, dy = ev.xmotion.y_root - *pressY;
		if (!*dragged && dx * dx + dy * dy < 36) return -1;
		*dragged = 1;
		b->expanded = 0;
		b->x = *startX + dx; b->y = *startY + dy;
		if (b->x < 0) b->x = 0;
		if (b->y < 0) b->y = 0;
		if (b->x > b->sw - BB_D) b->x = b->sw - BB_D;
		if (b->y > b->sh - BB_D) b->y = b->sh - BB_D;
		bbDraw(b);
		return 10;
	}
	case ButtonRelease: {
		if (ev.xbutton.button != 1 || !*pressed) return -1;
		*pressed = 0;
		if (*dragged) return 10;
		int wx, wy, w, h, bubbleX;
		bbGeom(b, &wx, &wy, &w, &h, &bubbleX);
		int lx = ev.xbutton.x;
		if (b->expanded) {
			for (int i = 0; i < 3; i++) {
				int bx;
				bbButton(b, w, i, &bx);
				if (lx >= bx && lx < bx + BB_BTN) {
					b->expanded = 0;
					bbDraw(b);
					return i;
				}
			}
		}
		b->expanded = !b->expanded;
		bbDraw(b);
		return 10;
	}
	}
	return -1;
}
*/
import "C"

import (
	"errors"
	"runtime"
	"sync"
	"time"

	"github.com/click-spirit/click-spirit/core"
)

type bubbleReq struct {
	visible            *bool
	recording, running bool
	state              bool
}

// x11Bubble owns its own X connection on one locked goroutine.
type x11Bubble struct {
	reqs chan bubbleReq
	quit chan struct{}
	done chan struct{} // closed after the window and connection are gone

	mu       sync.Mutex
	rect     core.Rect
	visible  bool
	onAction func(string)
}

var bubbleActions = [3]string{core.BubbleRecord, core.BubbleStart, core.BubbleStop}

// NewBubble returns the floating bubble for the active backend (X11 only).
func NewBubble() (core.Bubble, error) {
	if Name() != "x11" {
		return nil, errors.New("floating bubble needs the X11 backend: " + ErrUnsupported.Error())
	}
	b := &x11Bubble{reqs: make(chan bubbleReq, 8), quit: make(chan struct{}), done: make(chan struct{})}
	ready := make(chan error, 1)
	go b.loop(ready)
	if err := <-ready; err != nil {
		return nil, err
	}
	return b, nil
}

func (b *x11Bubble) SetVisible(on bool) { b.reqs <- bubbleReq{visible: &on} }

func (b *x11Bubble) SetState(recording, running bool) {
	select {
	case b.reqs <- bubbleReq{state: true, recording: recording, running: running}:
	default:
	}
}

func (b *x11Bubble) SetOnAction(fn func(string)) {
	b.mu.Lock()
	b.onAction = fn
	b.mu.Unlock()
}

func (b *x11Bubble) Rect() (core.Rect, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.rect, b.visible
}

// Close destroys the window and waits until it and the connection are gone.
func (b *x11Bubble) Close() {
	select {
	case <-b.quit:
	default:
		close(b.quit)
	}
	<-b.done
}

func (b *x11Bubble) publish(cb *C.csBubble) {
	var out [4]C.int
	mapped := C.bbRect(cb, &out[0])
	b.mu.Lock()
	b.rect = core.Rect{X: int(out[0]), Y: int(out[1]), W: int(out[2]), H: int(out[3])}
	b.visible = mapped != 0
	b.mu.Unlock()
}

func (b *x11Bubble) loop(ready chan<- error) {
	defer close(b.done)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	cb := C.bbOpen()
	if cb == nil {
		ready <- errors.New("cannot open X display with the SHAPE extension for the bubble")
		return
	}
	defer C.bbClose(cb)
	b.publish(cb)
	ready <- nil

	var pressX, pressY, startX, startY, pressed, dragged C.int
	raise := time.NewTicker(time.Second)
	defer raise.Stop()
	for {
		select {
		case <-b.quit:
			return
		case r := <-b.reqs:
			if r.visible != nil {
				on := C.int(0)
				if *r.visible {
					on = 1
				}
				C.bbSetVisible(cb, on)
			}
			if r.state {
				cb.recording = boolC(r.recording)
				cb.running = boolC(r.running)
				if cb.mapped != 0 {
					C.bbDraw(cb)
				}
			}
			b.publish(cb)
			continue
		case <-raise.C:
			C.bbRaise(cb)
		default:
		}
		if C.bbPending(cb) == 0 {
			fds := []C.struct_pollfd{{fd: C.bbFd(cb), events: C.POLLIN}}
			C.poll(&fds[0], 1, 50)
			continue
		}
		act := C.bbHandle(cb, &pressX, &pressY, &startX, &startY, &pressed, &dragged)
		if act >= 0 {
			b.publish(cb)
		}
		if act >= 0 && act <= 2 {
			b.mu.Lock()
			fn := b.onAction
			b.mu.Unlock()
			if fn != nil {
				go fn(bubbleActions[act])
			}
		}
	}
}

func boolC(v bool) C.int {
	if v {
		return 1
	}
	return 0
}
