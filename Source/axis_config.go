package main

import (
	"encoding/json"
	"strings"
)

func readAxis(raw map[string]json.RawMessage, key string, dst *int) {
	value, ok := raw[key]
	if !ok {
		return
	}
	var index int
	if json.Unmarshal(value, &index) == nil {
		*dst = index
		return
	}
	var name string
	if json.Unmarshal(value, &name) != nil {
		return
	}
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "x":
		*dst = 0
	case "y":
		*dst = 1
	case "z":
		*dst = 2
	case "r", "rx", "x rotation (r)", "x rotation (rx)":
		*dst = 3
	case "u", "ry", "y rotation (u)", "y rotation (ry)":
		*dst = 4
	case "v", "rz", "z rotation (v)", "z rotation (rz)":
		*dst = 5
	case "slider 1":
		*dst = 6
	case "slider 2":
		*dst = 7
	}
}
