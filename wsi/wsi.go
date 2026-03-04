// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package wsi

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"slices"
	"sync"
	"time"
	"unsafe"

	"honnef.co/go/jello/mem"
	wl "honnef.co/go/libwayland"

	"golang.org/x/sys/unix"
)

type System struct {
	app Application
	wl  *waylandDsp

	userEvents     chan userEvent
	requestedFrame chan struct{}
	eventArena     mem.Arena
	windows        []*WaylandWindow

	mu              sync.Mutex
	requestedFrames map[*WaylandWindow]struct{}
}

type userEvent struct {
	win Window
	ev  Event
}

type Context struct {
	Window     Window
	userEvents []userEvent
	sys        *System
}

func (ctx *Context) EmitEvent(win Window, ev Event) {
	ctx.userEvents = append(ctx.userEvents, userEvent{win: win, ev: ev})
}

func (ctx *Context) CreateWindow() Window {
	return ctx.sys.CreateWindow()
}

type Event any

type Application interface {
	WindowEvent(*Context, Event)
}

func NewSystem(app Application) *System {
	return &System{
		app:             app,
		userEvents:      make(chan userEvent, 1),
		requestedFrame:  make(chan struct{}, 1),
		requestedFrames: make(map[*WaylandWindow]struct{}),
	}
}

type waylandDsp struct {
	dsp     *wl.Display
	reg     *wl.Registry
	comp    *wl.Compositor
	xdgBase *wl.XdgWmBase
	dcm     *wl.XdgDecorationManager
	pres    *wl.WpPresentation
	porter  *wl.WpViewporter
	shm     *wl.Shm

	// seat waylandSeat
}

// type waylandSeat struct {
// 	seat    *wl.Seat
// 	pointer *wl.Pointer
// }

func clampVersion(minDesired, maxDesired, supported uint32) uint32 {
	if minDesired > supported {
		// The compositor doesn't support our minimum version; return the
		// minimum version, which will lead to our request failing.
		return minDesired
	}
	return min(supported, maxDesired)
}

func newWaylandDsp() (*waylandDsp, error) {
	// XXX properly disconnect when we return an error

	dsp, err := wl.Connect()
	if err != nil {
		return nil, err
	}

	reg := dsp.Registry()

	var comp *wl.Compositor
	var shm *wl.Shm
	var xdg *wl.XdgWmBase
	var dcm *wl.XdgDecorationManager
	var pres *wl.WpPresentation
	var porter *wl.WpViewporter
	reg.OnGlobal = func(name uint32, iface string, version uint32) {
		switch iface {
		case "wl_compositor":
			// Version 6 adds wl_surface.preferred_buffer_scale event
			// Version 5 adds wl_surface.offset
			// Version 4 adds wl_surface.damage_buffer
			// Version 3 adds surface scaling
			// Version 2 adds buffer transformations
			//
			// Except for Mir, everyone supports at least version 5. We support up to version 6.
			comp = reg.BindCompositor(name, clampVersion(5, 6, version))
		case "wl_shm":
			shm = reg.BindShm(name, 1)
		case "xdg_wm_base":
			// Version 6 adds the suspended state
			// Version 5 adds wm_capabilities event
			// Version 4 adds configure_bounds event
			// Version 3 adds implicit popup repositioning
			// Version 2 adds tiling states
			//
			// We only need version 1, but support up to version 6.
			xdg = reg.BindXdgWmBase(name, clampVersion(1, 6, version))
			xdg.OnPing = func(serial uint32) {
				xdg.Pong(serial)
			}
		case "zxdg_decoration_manager_v1":
			dcm = reg.BindZxdgDecorationManagerV1(name, 1)
		case "wp_presentation":
			pres = reg.BindWpPresentation(name, 1)
		case "wp_viewporter":
			porter = reg.BindWpViewporter(name, 1)
		}
	}

	// Roundtrip calls dsp.Sync and waits for the callback to fire, so at this point we
	// know that we've seen all of the globals that existed when we created the registry.
	dsp.Roundtrip()

	// We no longer care about events for globals
	reg.OnGlobal = func(uint32, string, uint32) {}

	if comp == nil {
		return nil, errors.New("Wayland server has no compositor global")
	}
	if shm == nil {
		return nil, errors.New("Wayland server has no shm global")
	}
	if xdg == nil {
		return nil, errors.New("Wayland server has no xdg_wm_base global")
	}
	// if dcm == nil {
	// 	// XXX in a real application we'd also want to support the KDE one, and fall back to client side decorations
	// 	// otherwise.
	// 	return nil, errors.New("Wayland server has no zxdg_decoration_manager_v1 global")
	// }

	return &waylandDsp{
		dsp:     dsp,
		reg:     reg,
		comp:    comp,
		xdgBase: xdg,
		dcm:     dcm,
		pres:    pres,
		porter:  porter,
		shm:     shm,
	}, nil
}

type Buffer struct {
	wl   *wl.Buffer
	busy bool
	size PhysicalSize
	Data []byte
}

func (buf *Buffer) destroy() {
	buf.wl.Destroy()
	unix.Munmap(buf.Data)
	buf.Data = nil
}

func (dsp *waylandDsp) makeBuffer(sz PhysicalSize, format wl.ShmFormat, size, stride int) (*Buffer, error) {
	// TODO(dh): compute size and stride from width, height, and format

	fd, err := unix.MemfdCreate("window", unix.MFD_CLOEXEC)
	if err != nil {
		return nil, fmt.Errorf("couldn't make buffer: %w", err)
	}
	defer unix.Close(fd)
	if err := unix.Ftruncate(fd, int64(size)); err != nil {
		return nil, fmt.Errorf("couldn't resize buffer: %w", err)
	}
	data, err := unix.Mmap(fd, 0, size, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		return nil, fmt.Errorf("couldn't mmap buffer: %w", err)
	}
	pool := dsp.shm.CreatePool(int32(fd), int32(size))
	defer pool.Destroy()

	buf := &Buffer{
		wl:   pool.CreateBuffer(0, int32(sz.Width), int32(sz.Height), int32(stride), format),
		size: sz,
		Data: data,
	}
	buf.wl.OnRelease = func() {
		buf.busy = false
	}

	return buf, nil
}

var errPollHUP = errors.New("remote hung up")

func (sys *System) poll(ctx context.Context, dsp *wl.Display, results chan pollResult, trigger chan struct{}) (err error) {
	quitR, quitW, err := os.Pipe()
	if err != nil {
		return err
	}
	defer quitR.Close()
	fd := dsp.Fd()

	go func() {
		<-ctx.Done()
		quitW.Close()
	}()

	// XXX we need a way to trigger flushing externally, e.g. to request a
	// new frame from outside the event loop. wl_display_flush is reentrant,
	// but because it may not write all data, we'd want to start polling
	// again. we need another fd that we can signal to start flushing again.
	fds := []unix.PollFd{
		{Fd: int32(fd), Events: unix.POLLIN | unix.POLLOUT},
		{Fd: int32(quitR.Fd()), Events: unix.POLLIN},
	}

	for {
		for {
			_, err := unix.Poll(fds, -1)
			if err == unix.EINTR {
				continue
			} else if err != nil {
				return err
			} else {
				break
			}
		}
		// Context was cancelled while we were polling.
		if fds[1].Revents != 0 {
			return ctx.Err()
		}

		select {
		case results <- pollResult{fds: fds}:
		case <-ctx.Done():
			return ctx.Err()
		}
		select {
		case <-trigger:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

type pollResult struct {
	fds []unix.PollFd
}

func (sys *System) Run(ctx context.Context) (err error) {
	dsp, err := newWaylandDsp()
	if err != nil {
		return fmt.Errorf("couldn't connect to Wayland: %w", err)
	}
	sys.wl = dsp
	defer sys.cleanup()

	// Call PrepareRead so we're ready to start polling.
	for dsp.dsp.PrepareRead() != 0 {
		dsp.dsp.DispatchPending()
	}
	defer dsp.dsp.CancelReadIfPrepared()

	pollResults := make(chan pollResult)
	pollTrigger := make(chan struct{})
	pollErr := make(chan error, 1)
	pollCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	defer cancel()
	go func() {
		err := sys.poll(pollCtx, dsp.dsp, pollResults, pollTrigger)
		switch err {
		case context.Canceled, context.DeadlineExceeded:
			// Do nothing. We're the reason the context got cancelled.
		case nil:
			// Do nothing. We can only get here via context cancellation or via
			// an error in the event loop.
		default:
			// poll itself failed, which is catastrophic.
			pollErr <- err
		}
	}()

	sys.app.WindowEvent(&Context{sys: sys}, &EventInitialized{})
	const quitTimeout = 10 * time.Second
	var timeout <-chan time.Time
	var stopWriting, quitting, synced bool
	// Our event loop keeps running after ctx.Done has been signaled, to read
	// outstanding messages and quit gracefully. done is set to nil after
	// receiving from it to avoid reacting to context cancellation more than
	// once. All other checks for cancellation should continue to use ctx.Done()
	done := ctx.Done()
	for !synced {
		sys.eventArena.Reset()
		select {
		case err := <-pollErr:
			// The poll syscall failed, which is catastrophic. We'll try our
			// best to clean up, but any pending reads and writes will get
			// discarded.
			return fmt.Errorf("poll failed: %w", err)
		case res := <-pollResults:
			fds := res.fds
			if fds[0].Revents&unix.POLLIN != 0 {
				// Read all available data.
				err := dsp.dsp.ReadEvents()
				// libwayland will only return a non-nil error from ReadEvents
				// when it read no data or on fatal errors, such as not being
				// able to queue events. That means we don't have to dispatch
				// before handling the error.
				if err != nil {
					// We've read all there is to read and the connection is
					// useless now. Best case scenario, the compositor closed
					// the connection. But maybe it crashed, or it misbehaved
					// and only closed half of the connection. In any case we
					// have no reason to write more data to it.
					if err == unix.EPIPE && quitting {
						// We've been expecting this error.
						return ctx.Err()
					} else {
						return fmt.Errorf("couldn't read events: %w", err)
					}
				}
				// Dispatch all events and prepare us for the next iteration of
				// polling.
				for dsp.dsp.PrepareRead() != 0 {
					dsp.dsp.DispatchPending()
				}
			}

			if !stopWriting && fds[0].Revents&(unix.POLLIN|unix.POLLOUT) != 0 {
				// Either continue flushing because a previous flush didn't
				// complete, or try to flush data written while we handled
				// events.

				_, err := dsp.dsp.Flush()
				switch err {
				case unix.EAGAIN:
					// We need to write more, make sure we're polling for
					// writability
					fds[0].Events |= unix.POLLOUT
				case unix.EPIPE:
					// The compositor has closed the connection. We want to read
					// the remaining data, in particular because there might be
					// a protocol error. We'll just keep polling for readability
					// and read remaining data until that fails, too.
					fds[0].Events &^= unix.POLLOUT
					stopWriting = true
				case nil:
					// We've flushed everything, stop polling for writability.
					fds[0].Events &^= unix.POLLOUT

					if quitting {
						// If we're quitting because the context was cancelled
						// and we've gotten here it means we've written
						// everything up to that point, including the sync request.
						// Prevent any further writes.
						stopWriting = true
					}
				default:
					// Writing failed in an unexpected way. Tear everything down
					// and don't try to make sense of any remaining data.
					return err
				}
			}

			// Start next round of polling.
			pollTrigger <- struct{}{}

		case <-done:
			// Context was cancelled. Flush any remaining writes, read and
			// dispatch remaining events while preventing new writes from being
			// made, then return.
			done = nil
			quitting = true
			timeout = time.After(quitTimeout)
			dsp.dsp.Sync(func(data uint32) {
				synced = true
			})

		case <-timeout:
			// We've been trying long enough to shut down cleanly. At this point
			// we must assume that the compositor isn't cooperating and give up.
			return errors.New("timed out trying to shut down")

		case <-sys.requestedFrame:
			sys.mu.Lock()
			for win := range sys.requestedFrames {
				win.requestFrameNow()
			}
			clear(sys.requestedFrames)
			sys.mu.Unlock()
			dsp.dsp.Flush()

		case ev := <-sys.userEvents:
			sys.app.WindowEvent(&Context{Window: ev.win}, ev.ev)
		}
	}
	return ctx.Err()
}

func (sys *System) cleanup() {
	// Cancel pending prepared read so any deferred call of CancelReadIfPrepared
	// further up the call stack doesn't try to dereference the display we're
	// about to destroy.
	sys.wl.dsp.CancelReadIfPrepared()

	for _, win := range sys.windows {
		if win.port != nil {
			win.port.Destroy()
		}
		if win.deco != nil {
			win.deco.Destroy()
		}
		win.top.Destroy()
		win.xdgSurf.Destroy()
		win.surf.Destroy()
	}
	if sys.wl.porter != nil {
		sys.wl.porter.Destroy()
	}
	if sys.wl.pres != nil {
		sys.wl.pres.Destroy()
	}
	if sys.wl.dcm != nil {
		sys.wl.dcm.Destroy()
	}
	sys.wl.xdgBase.Destroy()
	sys.wl.comp.Destroy()
	sys.wl.reg.Destroy()
	sys.wl.dsp.Disconnect()
}

func (win *WaylandWindow) RequestFrame() {
	win.sys.mu.Lock()
	defer win.sys.mu.Unlock()
	win.sys.requestedFrames[win] = struct{}{}
	select {
	case win.sys.requestedFrame <- struct{}{}:
	default:
	}
}

func (win *WaylandWindow) redraw(d time.Duration) {
	if win.sys.wl.pres != nil {
		// XXX we have to remember fb and free it
		fb := win.sys.wl.pres.Feedback(win.surf)
		fb.OnPresented = func(tvSecHi, tvSecLo, tvNsec, refresh, seqHi, seqLo, flags uint32) {
			// fmt.Println("A", tvSecHi, tvSecLo, tvNsec, refresh, seqHi, seqLo, flags)
		}
		fb.OnDiscarded = func() {
			// fmt.Println("discarded")
		}
		fb.OnSyncOutput = func(o *wl.Output) {}
	}
	win.sys.app.WindowEvent(
		mem.Make(&win.sys.eventArena, Context{Window: win}),
		mem.Make(&win.sys.eventArena, RedrawRequested{d}),
	)
}

func (win *WaylandWindow) Present(buf *Buffer, x0, y0, x1, y1 int) {
	buf.busy = true
	win.surf.Attach(buf.wl)
	// TODO(dh): we should probably support multiple damage regions per buffer
	win.surf.DamageBuffer(int32(x0), int32(y0), int32(x1), int32(y1))
	win.surf.Commit()
}

func (win *WaylandWindow) onFrame(ms uint32) {
	// TODO(dh): libwayland should be responsible for giving us the right
	// argument type.
	// fmt.Println(ms)
	d := time.Duration(ms) * time.Millisecond
	win.frameScheduled = false
	win.redraw(d)
}

func (win *WaylandWindow) requestFrameNow() {
	if win.frameScheduled {
		return
	}
	win.frameScheduled = true
	win.surf.Frame(win.onFrame)
	if win.sys.wl.pres != nil {
		fb := win.sys.wl.pres.Feedback(win.surf)
		fb.OnPresented = func(tvSecHi, tvSecLo, tvNsec, refresh, seqHi, seqLo, flags uint32) {
			// fmt.Println("B", tvSecHi, tvSecLo, tvNsec, refresh, seqHi, seqLo, flags)
		}
		fb.OnDiscarded = func() {
			// fmt.Println("discarded")
		}
		fb.OnSyncOutput = func(o *wl.Output) {}
	}
	win.surf.Commit()
}

// SetSize sets the logical size of the window. To ensure proper cross-platform
// support, particularly for Wayland, it is recommended to only use sizes
// reported by [Resized] events, as different compositors handle unexpected
// sizes differently.
func (win *WaylandWindow) SetSize(sz LogicalSize) {
	win.calledSetSize = true
	win.size = sz
	win.needNewBuffers = true
	if win.port != nil {
		win.port.SetDestination(sz.Width, sz.Height)
	} else {
		// Without a wp_viewport, the window size will be determined by the size
		// of the committed buffer, scaled by the inverse of the scale set by
		// SetScale.
	}
}

// SetScale informs the windowing system of the scale with which you are
// rendering the window contents.
//
// If the scale doesn't match the scale reported
// by a [Resized] event, the windowing system will have to scale the contents,
// which may result in a blurry image.
//
// If [Resized.ScaleIsMandatory] is true
// then it is an error to call SetScale with a value that deviates from it.
//
// Support for fractional scales is only guaranteed when [Resized.Scale] is
// fractional. When fractional scaling isn't supported, values will be rounded
// to the nearest integer, but at least 1. SetScale returns the resulting scale,
// which allows checking for this condition.
func (win *WaylandWindow) SetScale(scale float64) float64 {
	win.calledSetScale = true
	if win.port == nil {
		scale = max(1, math.Round(scale))
		win.surf.SetBufferScale(int(scale))
		return scale
	} else {
		// With a wp_viewport, we set the logical size of the destination, and
		// the buffer will automatically be scaled up or down to match that
		// size.
		return scale
	}
}

func (win *WaylandWindow) PhysicalSize() PhysicalSize {
	// XXX ensure Wayland and us agree on rounding behavior
	// XXX is win.scale affected by SetScale? it doesn't seem to be
	return PhysicalSize{
		int(float64(win.size.Width) * win.scale),
		int(float64(win.size.Height) * win.scale),
	}
}

func emitResizedEvent(win *WaylandWindow, ev Resized) {
	win.calledSetSize = false
	win.calledSetScale = false
	win.sys.app.WindowEvent(
		mem.Make(&win.sys.eventArena, Context{Window: win}),
		mem.Make(&win.sys.eventArena, ev),
	)
	if !win.calledSetSize {
		panic("didn't call Window.SetSize while handling Resized event")
	}
	if !win.calledSetScale {
		panic("didn't call Window.SetScale while handling Resized event")
	}
}

func (win *WaylandWindow) setup() {
	var newWidth, newHeight int32
	first := true
	// XXX support older ways of figuring out DPI scaling
	win.surf.OnPreferred_buffer_scale = func(scale int) {
		win.scale = float64(scale)
		emitResizedEvent(
			win,
			Resized{
				Size:  win.size,
				Scale: float64(scale),
			},
		)
	}
	win.xdgSurf.OnConfigure = func(serial uint32) {
		changedSize := win.size.Width != int(newWidth) || win.size.Height != int(newHeight) || first

		if !first && changedSize {
			// Requesting a new frame requires committing the surface. We don't
			// want to commit the viewport updates that result from changing the
			// size, so we request the frame before we emit the resized event.
			win.requestFrameNow()
			win.dsp.Flush()
		}

		if changedSize {
			lsz := LogicalSize{
				Width:  int(newWidth),
				Height: int(newHeight),
			}
			emitResizedEvent(
				win,
				Resized{
					Size:  lsz,
					Scale: win.scale,
				},
			)
		}

		win.xdgSurf.AckConfigure(serial)
		if first {
			first = false
			// XXX 0 isn't great, e.g. if the app wants to fade in when starting
			win.redraw(0)
		}
	}

	win.top.OnConfigure = func(width, height int32, states []uint32) {
		newWidth, newHeight = width, height
		// XXX handle states
	}
	win.top.OnClose = func() {
		win.sys.app.WindowEvent(
			mem.Make(&win.sys.eventArena, Context{Window: win}),
			mem.Make(&win.sys.eventArena, CloseRequested{}),
		)
	}
}

func (sys *System) CreateWindow() Window {
	surf := sys.wl.comp.CreateSurface()

	xdgSurf := sys.wl.xdgBase.XdgSurface(surf)
	top := xdgSurf.Toplevel()
	var deco *wl.XdgToplevelDecoration
	var port *wl.WpViewport
	if sys.wl.dcm != nil {
		deco = sys.wl.dcm.ToplevelDecoration(top)
		deco.SetMode(wl.XdgToplevelDecorationModeServerSide)
	}
	if sys.wl.porter != nil {
		port = sys.wl.porter.Viewport(surf)
	}
	surf.Commit()

	win := &WaylandWindow{
		sys:     sys,
		dsp:     sys.wl.dsp,
		surf:    surf,
		xdgSurf: xdgSurf,
		top:     top,
		deco:    deco,
		port:    port,
		scale:   1,
	}

	sys.windows = append(sys.windows, win)

	win.setup()
	return win
}

type WaylandWindow struct {
	sys     *System
	dsp     *wl.Display
	surf    *wl.Surface
	xdgSurf *wl.XdgSurface
	top     *wl.XdgToplevel
	deco    *wl.XdgToplevelDecoration
	port    *wl.WpViewport
	size    LogicalSize
	scale   float64

	needNewBuffers bool
	buffers        []*Buffer

	// temporary state used by emitResizedEvent
	calledSetSize  bool
	calledSetScale bool

	// set to true by call to WaylandWindow.requestFrameNow and reset to false
	// by WaylandWindow.onFrame. Used to ensure that we request a frame at most
	// once per rendered frame.
	frameScheduled bool
}

func (w *WaylandWindow) NextBuffer() (*Buffer, error) {
	sz := w.PhysicalSize()
	if w.needNewBuffers {
		w.needNewBuffers = false
		for i, buf := range w.buffers {
			// XXX can we destroy a busy buffer and wayland will take care of it?
			if !buf.busy && buf.size != sz {
				buf.destroy()
				w.buffers[i] = nil
			}
		}
		w.buffers = slices.DeleteFunc(w.buffers, func(buf *Buffer) bool { return buf == nil })
	}

	for i, buf := range w.buffers {
		if !buf.busy {
			if buf.size == sz {
				return buf, nil
			} else {
				buf.destroy()
				w.buffers[i] = nil
			}
		}
	}
	w.buffers = slices.DeleteFunc(w.buffers, func(buf *Buffer) bool { return buf == nil })

	// XXX make format configurable
	buf, err := w.sys.wl.makeBuffer(sz, wl.ShmFormatAbgr8888, sz.Width*sz.Height*1*4, sz.Width*1*4)
	if err != nil {
		return nil, err
	}
	w.buffers = append(w.buffers, buf)
	return buf, nil
}

func (w *WaylandWindow) Display() unsafe.Pointer {
	return w.dsp.Handle()
}

func (w *WaylandWindow) Surface() unsafe.Pointer {
	return w.surf.Handle()
}

func (w *WaylandWindow) Attach(buf *wl.Buffer) {
	w.surf.Attach(buf)
}

func (sys *System) EmitEvent(win Window, ev any) {
	sys.userEvents <- userEvent{win: win, ev: ev}
}

type Window interface {
	RequestFrame()
	SetSize(LogicalSize)
	SetScale(float64) float64
	PhysicalSize() PhysicalSize
}

type EventInitialized struct{}

// A Resized event is emitted whenever the window size or the scaling factor
// change. [Window.SetSize] and [Window.SetScale] must be called before
// returning from the event handler.
type Resized struct {
	// The new logical size of the window. If it is (0, 0), you must choose any
	// size with non-zero dimensions.
	Size LogicalSize
	// Scale indicates the scale with which the windowing system will map from
	// logical pixels to physical pixels. For example, for a logical size of
	// (200, 200) and a scale of 2, the window will occupy (400, 400) physical
	// pixels on the output device.
	Scale float64
	// If ScaleIsMandatory is true, [Window.SetScale] must be called with the
	// value of Scale, and the window contents must be rendered at that scale.
	//
	// If ScaleIsMandatory is false, [Window.SetScale] must still be called, but
	// you're free to choose any scale.
	ScaleIsMandatory bool
}

type RedrawRequested struct {
	When time.Duration
}

type CloseRequested struct{}

type LogicalSize struct {
	Width, Height int
}

type PhysicalSize struct {
	Width, Height int
}

func (sz LogicalSize) Scale(f float64) PhysicalSize {
	return PhysicalSize{
		int(math.Round(float64(sz.Width) * f)),
		int(math.Round(float64(sz.Height) * f)),
	}
}
