package main

import "strings"

func normalizeBindingKey(k string) string {
	s := strings.ToUpper(strings.TrimSpace(k))
	r := strings.NewReplacer(" ", "", "-", "", "_", "", "/", "")
	s = r.Replace(s)
	aliases := map[string]string{
		"XBOXA": "A", "XBOXB": "B", "XBOXX": "X", "XBOXY": "Y",
		"LEFTBUMPER": "LB", "RIGHTBUMPER": "RB", "L1": "LB", "R1": "RB",
		"LEFTTRIGGER": "LT", "RIGHTTRIGGER": "RT", "L2": "LT", "R2": "RT", "BRAKE": "LT",
		"BACKVIEW": "BACK", "VIEW": "BACK", "STARTMENU": "START", "MENU": "START",
		"LEFTSTICKCLICK": "LTHUMB", "RIGHTSTICKCLICK": "RTHUMB", "LTHUMB": "LTHUMB", "RTHUMB": "RTHUMB",
		"DPADUP": "DUP", "DPADDOWN": "DDOWN", "DPADLEFT": "DLEFT", "DPADRIGHT": "DRIGHT",
		"WASDW": "WASD_W", "WASDA": "WASD_A", "WASDS": "WASD_S", "WASDD": "WASD_D",
	}
	if alias, ok := aliases[s]; ok {
		return alias
	}
	return s
}
