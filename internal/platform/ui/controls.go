package ui

func controlClasses(size, rounded Size, color Color, invalid bool, class string) string {
	base := "block w-full bg-white border transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-offset-1 disabled:opacity-50 disabled:cursor-not-allowed placeholder:text-slate-400"

	var sizeClasses string

	switch size {
	case SM:
		sizeClasses = "px-2 py-1 text-sm"
	case LG:
		sizeClasses = "px-4 py-2.5 text-base"
	default:
		sizeClasses = "px-3 py-2 text-sm"
	}

	var stateClasses string

	if invalid {
		stateClasses = "border-red-500 focus-visible:ring-red-500"
	} else {
		stateClasses = "border-slate-300 " + controlFocusRing(color)
	}

	return Merge(base, sizeClasses, roundedClass(rounded), stateClasses, class)
}

var controlFocusRings = map[Color]string{
	ColorBlue:   "focus-visible:ring-blue-500",
	ColorRed:    "focus-visible:ring-red-500",
	ColorYellow: "focus-visible:ring-yellow-500",
	ColorGreen:  "focus-visible:ring-green-500",
	ColorOrange: "focus-visible:ring-orange-500",
	ColorPurple: "focus-visible:ring-purple-500",
	ColorPink:   "focus-visible:ring-pink-500",
	ColorIndigo: "focus-visible:ring-indigo-500",
	ColorTeal:   "focus-visible:ring-teal-500",
	ColorZinc:   "focus-visible:ring-zinc-700",
	ColorStone:  "focus-visible:ring-stone-600",
}

func controlFocusRing(color Color) string {
	if ring, ok := controlFocusRings[color]; ok {
		return ring
	}

	return "focus-visible:ring-slate-400"
}

func roundedClass(rounded Size) string {
	switch rounded {
	case SM:
		return "rounded-sm"
	case MD:
		return "rounded-md"
	case LG:
		return "rounded-lg"
	case XL:
		return "rounded-xl"
	default:
		return ""
	}
}
