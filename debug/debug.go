package debug

// XXX move this behind a build tag
const debug = true

func Assert(b bool) {
	if !b {
		panic("failed assertion")
	}
}
