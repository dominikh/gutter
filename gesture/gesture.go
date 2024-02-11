package gesture

/*
   Problems with Flutter's gesture resolution

   Flutter uses an "arena" to choose a winning gesture recognizer. Recognizers can declare defeat or victory.
   A recognizer wins when it declares victory, when it is the only one left in the arena, or if it's the first
   item in the arena when the arena gets swept. The arena is swept on each pointer up event, unless a
   recognizer is explicitly "holding" the arena, to defer its decision until after a pointer up event.

   This approach naturally divides recognizers into two groups: passive recognizers, which can only win by
   default, and active recognizers, which win by active choice. An arena full of passive recognizers can only
   win due to sweeping. This implies that a held arena full of passive recognizers cannot make progress if
   recognizers don't release the hold.

   This negatively affects the design of some of the recognizers in Flutter. For example, single and double
   tap recognizers cannot both be passive, because otherwise they would have to wait for each other and
   deadlock. The single tap recognizer _could_ resolve the deadlock by declaring defeat after seeing the
   second pointer down event, but then it wouldn't be able to differentiate down + up + down + up (a double
   tap) from down + up + down + move (a single tap followed by a drag.) In fact, in current Flutter, combining
   the single and double tap recognizers. the second sequence will lead to no events being emitted at all,
   because the single tap recognizer gives up when seeing the move, instead of remembering that it had already
   finished one tap successfully.

   Double tap being active also means that double tap and triple tap cannot be used at the same time, as
   double tap's active nature prevents triple taps from ever being recognized.

   The obvious solution of combining all of these recognizers into an n-tap one only helps when all
   recognizers are being used on the same object. Once we consider parent-child relationships (e.g. a parent
   triple-tapper with a child double-tapper), we can no longer rely on combining the two. Flutter also has a
   "serial tap" recognizer, which recognizes arbitrarily long chains of taps, by emitting an event for each
   successive tap, where the event includes a tally. While this has other, valid applications, it doesn't help
   with this problem, as it merely shifts it to the user.

   In Gutter, we instead use multiple arenas, one per pointer down event. Each recognizer can only be in a
   single arena, and remains in its arena until it has lost. Recognizers that aren't in an arena yet are added
   to the newest arena. Only the oldest non-empty arena can be swept, the other arenas have to wait their
   turn. All recognizers are active and a decision is only reached once all members in the arena have declared
   victory or defeat. Multiple recognizers can declare victory, and the one consisting of the most pointer
   down events wins.

   Considering 3 recognizers single tap, double tap, drag:

   - A recognizer mustn't exist in two arenas at once. Otherwise, 3 taps would fire two double taps.

   - An arena of size 1 mustn't resolve immediately while previous arenas exist. Otherwise, down->up->down
     would immediately resolve a drag in the second arena.

   - When processing arenas, arena existence changes must be visible within the same frame. That is,
     down->up->down->move should immediately resolve to a single tap and a drag.
*/

import (
	"time"

	"honnef.co/go/gutter/debug"
	"honnef.co/go/gutter/f32"

	"gioui.org/app"
	"gioui.org/io/pointer"
)

type Recognizer interface {
	ArenaMember
	HandlePointerEvent(ev pointer.Event)
}

type ArenaMember interface {
	AcceptedGesture()
	RejectedGesture()
}

type Arena struct {
	members []arenaMember
	pending int
	downs   int
}

type arenaMember struct {
	member   ArenaMember
	win      int
	lost     bool
	rejected bool
}

type ArenaManager struct {
	Window *app.Window
	arenas []*Arena
}

func (am *ArenaManager) HandlePointerEvent(ev pointer.Event) {
	if ev.Kind == pointer.Press {
		for _, a := range am.arenas {
			a.downs++
		}
		am.grow()
	}
}

func (am *ArenaManager) Sweep() {
	swept := 0
	for swept < len(am.arenas) && am.arenas[swept].Sweep() {
		swept++
	}
	// fmt.Println("swept", swept, "of", len(am.arenas), "arenas")
	copy(am.arenas[:swept], am.arenas[swept:])
	am.arenas = am.arenas[:len(am.arenas)-swept]
	// fmt.Println(len(am.arenas), "arenas remain")
}

func (am *ArenaManager) grow() {
	if cap(am.arenas) > len(am.arenas) {
		am.arenas = am.arenas[:len(am.arenas)+1]
	} else {
		am.arenas = append(am.arenas, &Arena{})
	}
}

func (am *ArenaManager) Add(m ArenaMember) *Arena {
	if len(am.arenas) == 0 {
		am.grow()
	}
	a := am.arenas[len(am.arenas)-1]
	a.Add(m)
	// fmt.Printf("added member to arena %p\n", a)
	return a
}

func (a *Arena) Win(m ArenaMember) {
	for i := range a.members {
		om := &a.members[i]
		if om.member == m {
			debug.Assert(om.win == 0)
			om.win = a.downs
			a.pending--
			break
		}
	}
}

func (a *Arena) Lose(m ArenaMember) {
	debug.Assert(len(a.members) != 0)
	for i := range a.members {
		om := &a.members[i]
		if om.member == m {
			debug.Assert(!om.lost)
			om.lost = true
			a.pending--
			m.RejectedGesture()
			om.rejected = true
			break
		}
	}
}

func (a *Arena) Add(m ArenaMember) {
	// fmt.Printf("adding to arena %p\n", a)
	a.members = append(a.members, arenaMember{member: m})
	a.pending++
}

func (a *Arena) Sweep() bool {
	// fmt.Printf("sweeping arena %p\n", a)
	debug.Assert(a.pending >= 0)
	if a.pending != 0 {
		// fmt.Println("pending", a.pending)
		return false
	}

	// fmt.Println("sweeping", len(a.members), "members")
	if len(a.members) == 0 {
		return true
	}

	largest := &a.members[0]
	for i := range a.members {
		m := &a.members[i]
		if m.win > largest.win {
			largest = m
		}
	}
	if largest.lost {
		largest = nil
	} else {
		largest.member.AcceptedGesture()
	}
	for i := range a.members {
		m := &a.members[i]
		if m == largest || m.rejected {
			continue
		}
		m.member.RejectedGesture()
	}

	a.reset()
	return true
}

func (a *Arena) reset() {
	a.downs = 0
	a.pending = 0
	clear(a.members)
	a.members = a.members[:0]
}

type TapRecognizer struct {
	// XXX allow configuring which buttons are allowed
	Manager *ArenaManager

	OnTapStart func(ev pointer.Event)
	OnTap      func(ev pointer.Event)
	// XXX we aren't firing this event yet
	OnTapCancel func(ev pointer.Event)

	arena   *Arena
	start   pointer.Event
	active  bool
	waiting bool
}

func (tap *TapRecognizer) HandlePointerEvent(ev pointer.Event) {
	// fmt.Println(ev, tap.waiting, tap.active)
	if tap.waiting {
		// We've finished our tap and are waiting to be elected the winner. No event can change that.
		return
	}

	if ev.Kind != pointer.Press && !tap.active {
		return
	}

	// XXX use logical pixels instead of physical pixels for slack
	const slack = 15
	switch ev.Kind {
	case pointer.Press:
		tap.arena = tap.Manager.Add(tap)
		if tap.OnTapStart != nil {
			tap.OnTapStart(ev)
		}
		tap.start = ev
		tap.active = true
	case pointer.Release:
		// XXX compare buttons
		tap.arena.Win(tap)
		tap.waiting = true
	case pointer.Cancel:
		tap.lose()
	case pointer.Move:
		if ev.Source == pointer.Mouse {
			// Any amount of motion turns this from a tap into a drag. Most desktop UI elements that can get
			// clicked, like buttons, will actually want to listen to raw down and up events to recognize
			// clicks that move across the UI element, and rely on taps only for touch interfaces.
			tap.lose()
		} else {
			d := f32.Magnitude(ev.Position.Sub(tap.start.Position))
			if d > slack {
				tap.lose()
			}
		}
	}
}

func (tap *TapRecognizer) lose() {
	tap.arena.Lose(tap)
	tap.reset()
}

func (tap *TapRecognizer) AcceptedGesture() {
	if tap.OnTap != nil {
		tap.OnTap(tap.start)
	}
	tap.reset()
}

func (tap *TapRecognizer) RejectedGesture() {
	tap.reset()
}

func (tap *TapRecognizer) reset() {
	tap.active = false
	tap.arena = nil
	tap.start = pointer.Event{}
	tap.waiting = false
}

type DoubleTapRecognizer struct {
	Manager *ArenaManager

	OnTapStart  func(ev pointer.Event)
	OnDoubleTap func(ev pointer.Event)

	arena      *Arena
	active     bool
	waiting    bool
	second     bool
	start      pointer.Event
	timer      *time.Timer
	generation int
}

type CallbackEvent func()

func (CallbackEvent) ImplementsEvent() {}

func (tap *DoubleTapRecognizer) HandlePointerEvent(ev pointer.Event) {
	if tap.waiting {
		// We've finished our double tap and are waiting to be elected the winner. No event can change that.
		return
	}
	if ev.Kind != pointer.Press && !tap.active {
		return
	}
	// XXX use logical pixels instead of physical pixels for slack
	const slack = 15
	switch ev.Kind {
	case pointer.Press:
		if tap.active {
			tap.second = true
			tap.timer.Stop()
		} else {
			tap.arena = tap.Manager.Add(tap)
			if tap.OnTapStart != nil {
				tap.OnTapStart(ev)
			}
			tap.start = ev
			tap.active = true
			gen := tap.generation
			var timeout time.Duration
			if ev.Source == pointer.Mouse {
				timeout = 250 * time.Millisecond
			} else {
				timeout = 500 * time.Millisecond
			}
			tap.timer = time.AfterFunc(timeout, func() {
				tap.Manager.Window.EmitEvent(CallbackEvent(func() {
					if gen != tap.generation {
						return
					}
					tap.lose()
					tap.Manager.Sweep()
				}))
			})
		}
	case pointer.Release:
		// XXX compare buttons
		if tap.second {
			tap.arena.Win(tap)
			tap.waiting = true
		}
	case pointer.Cancel:
		tap.lose()
	case pointer.Move:
		if ev.Source == pointer.Mouse {
			// Any amount of motion turns this from a tap into a drag. Most desktop UI elements that can get
			// clicked, like buttons, will actually want to listen to raw down and up events to recognize
			// clicks that move across the UI element, and rely on taps only for touch interfaces.
			tap.lose()
		} else {
			d := f32.Magnitude(ev.Position.Sub(tap.start.Position))
			if d > slack {
				tap.lose()
			}
		}
	}
}

func (tap *DoubleTapRecognizer) lose() {
	tap.arena.Lose(tap)
	tap.reset()
}

func (tap *DoubleTapRecognizer) AcceptedGesture() {
	if tap.OnDoubleTap != nil {
		tap.OnDoubleTap(tap.start)
	}
	tap.reset()
}

func (tap *DoubleTapRecognizer) RejectedGesture() {
	tap.reset()
}

func (tap *DoubleTapRecognizer) reset() {
	tap.generation++
	tap.timer.Stop()
	tap.active = false
	tap.arena = nil
	tap.second = false
	tap.start = pointer.Event{}
	tap.waiting = false
}
