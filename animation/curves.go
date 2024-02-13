package animation

import "math"

func EaseInSine(t float64) float64 {
	return 1 - math.Cos((t*math.Pi)/2)
}

func EaseOutSine(t float64) float64 {
	return math.Sin((t * math.Pi) / 2)
}

func EaseInOutSine(t float64) float64 {
	return -(math.Cos(math.Pi*t) - 1) / 2
}

func EaseInQuad(t float64) float64 {
	return t * t
}

func EaseOutQuad(t float64) float64 {
	return 1 - (1 - t) - (1 - t)
}

func EaseInOutQuad(t float64) float64 {
	if t < 0.5 {
		return 2 * t * t
	} else {
		return 1 - (-2*t+2)*(-2*t+2)/2
	}
}

func EaseInCubic(t float64) float64 {
	return t * t * t
}

func EaseOutCubic(t float64) float64 {
	return 1 - (1-t)*(1-t)*(1-t)
}

func EaseInOutCubic(t float64) float64 {
	if t < 0.5 {
		return 4 * t * t * t
	} else {
		return 1 - (-2*t+2)*(-2*t+2)*(-2*t+2)/2
	}
}

func EaseInQuart(t float64) float64 {
	return t * t * t * t
}

func EaseOutQuart(t float64) float64 {
	return 1 - (1-t)*(1-t)*(1-t)*(1-t)
}

func EaseInOutQuart(t float64) float64 {
	if t < 0.5 {
		return 8 * t * t * t * t
	} else {
		return 1 - (-2*t+2)*(-2*t+2)*(-2*t+2)*(-2*t+2)/2
	}
}

func EaseInQuint(t float64) float64 {
	return t * t * t * t * t
}

func EaseOutQuint(t float64) float64 {
	return 1 - (1-t)*(1-t)*(1-t)*(1-t)*(1-t)
}

func EaseInOutQuint(t float64) float64 {
	if t < 0.5 {
		return 16 * t * t * t * t * t
	} else {
		return 1 - (-2*t+2)*(-2*t+2)*(-2*t+2)*(-2*t+2)*(-2*t+2)/2
	}
}

func EaseInCirc(t float64) float64 {
	return 1 - math.Sqrt(1-t*t)
}

func EaseOutCirc(t float64) float64 {
	return math.Sqrt(1 - (t-1)*(t-1))
}

func EaseInOutCirc(t float64) float64 {
	if t < 0.5 {
		return (1 - math.Sqrt(1-2*t*2*t)) / 2
	} else {
		return (math.Sqrt(1-(-2*t+2)*(-2*t+2)) + 1) / 2
	}
}

func EaseInElastic(t float64) float64 {
	switch t {
	case 0:
		return 0
	case 1:
		return 1
	default:
		const c4 = (2 * math.Pi) / 3
		return -math.Pow(2, 10*t-10) * math.Sin((t*10-10.75)*c4)
	}
}

func EaseOutElastic(t float64) float64 {
	switch t {
	case 0:
		return 0
	case 1:
		return 1
	default:
		const c4 = (2 * math.Pi) / 3
		return math.Pow(2, -10*t)*math.Sin((t*10-0.75)*c4) + 1
	}
}

func EaseInOutElastic(t float64) float64 {
	const c5 = (2 * math.Pi) / 4.5
	if t == 0 {
		return 0
	} else if t == 1 {
		return 1
	} else if t < 0.5 {
		return -(math.Pow(2, 20*t-10) * math.Sin((20*t-11.125)*c5)) / 2
	} else {
		return (math.Pow(2, -20*t+10)*math.Sin((20*t-11.125)*c5))/2 + 1
	}
}

func EaseInBounce(t float64) float64 {
	return 1 - EaseOutBounce(1-t)
}

func EaseOutBounce(t float64) float64 {
	const n1 = 7.5625
	const d1 = 2.75

	if t < 1.0/d1 {
		return n1 * t * t
	} else if t < 2.0/d1 {
		t -= 1.5 / d1
		return n1*t*t + 0.75
	} else if t < 2.5/d1 {
		t -= 2.25 / d1
		return n1*t*t + 0.9375
	} else {
		t -= 2.625 / d1
		return n1*t*t + 0.984375
	}
}

func EaseInOutBounce(t float64) float64 {
	if t < 0.5 {
		return (1 - EaseOutBounce(1-2*t)) / 2
	} else {
		return (1 + EaseOutBounce(2*t-1)) / 2
	}
}

func EaseInExpo(t float64) float64 {
	if t == 0 {
		return 0
	} else {
		return math.Pow(2, 10*t-10)
	}
}

func EaseOutExpo(t float64) float64 {
	if t == 1 {
		return 1
	} else {
		return 1 - math.Pow(2, -10*t)
	}
}

func EaseInOutExpo(t float64) float64 {
	if t == 0 {
		return 0
	} else if t == 1 {
		return 1
	} else if t < 0.5 {
		return math.Pow(2, 20*t-10) / 2
	} else {
		return (2 - math.Pow(2, -20*t+10)) / 2
	}
}

func EaseInBack(t float64) float64 {
	const c1 = 1.70158
	const c3 = c1 + 1
	return c3*t*t*t - c1*t*t
}

func EaseOutBack(t float64) float64 {
	const c1 = 1.70158
	const c3 = c1 + 1

	return 1 + c3*(t-1)*(t-1)*(t-1) + c1*(t-1)*(t-1)
}

func EaseInOutBack(t float64) float64 {
	const c1 = 1.70158
	const c2 = c1 * 1.525

	if t < 0.5 {
		return (2 * t * 2 * t * ((c2+1)*2*t - c2)) / 2
	} else {
		return ((2*t-2)*(2*t-2)*((c2+1)*(t*2-2)+c2) + 2) / 2
	}
}
