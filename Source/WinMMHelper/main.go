//go:build windows

// Galactic Racer WinMM Compatibility Helper
// Copyright (C) 2026 JaffaQuake
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

type joyCaps struct {
	Mid                  uint16
	Pid                  uint16
	Pname                [32]uint16
	Xmin, Xmax           uint32
	Ymin, Ymax           uint32
	Zmin, Zmax           uint32
	NumButtons           uint32
	PeriodMin, PeriodMax uint32
	Rmin, Rmax           uint32
	Umin, Umax           uint32
	Vmin, Vmax           uint32
	Caps                 uint32
	MaxAxes              uint32
	NumAxes              uint32
	MaxButtons           uint32
	RegKey               [32]uint16
	OEMVxD               [260]uint16
}

type countReply struct {
	Count int `json:"count"`
}

type capsReply struct {
	OK         bool   `json:"ok"`
	Result     uint32 `json:"result"`
	Name       string `json:"name"`
	Xmin       uint32 `json:"xmin"`
	Xmax       uint32 `json:"xmax"`
	Ymin       uint32 `json:"ymin"`
	Ymax       uint32 `json:"ymax"`
	Zmin       uint32 `json:"zmin"`
	Zmax       uint32 `json:"zmax"`
	Rmin       uint32 `json:"rmin"`
	Rmax       uint32 `json:"rmax"`
	Umin       uint32 `json:"umin"`
	Umax       uint32 `json:"umax"`
	Vmin       uint32 `json:"vmin"`
	Vmax       uint32 `json:"vmax"`
	NumButtons uint32 `json:"num_buttons"`
	Caps       uint32 `json:"caps"`
	MaxAxes    uint32 `json:"max_axes"`
	NumAxes    uint32 `json:"num_axes"`
	MaxButtons uint32 `json:"max_buttons"`
}

var (
	winmm              = syscall.NewLazyDLL("winmm.dll")
	procJoyGetNumDevs  = winmm.NewProc("joyGetNumDevs")
	procJoyGetDevCapsW = winmm.NewProc("joyGetDevCapsW")
)

func utf16SliceToString(a []uint16) string {
	n := 0
	for n < len(a) && a[n] != 0 {
		n++
	}
	return syscall.UTF16ToString(a[:n])
}

func writeJSON(v any) {
	_ = json.NewEncoder(os.Stdout).Encode(v)
}

func main() {
	if len(os.Args) < 2 {
		os.Exit(2)
	}
	switch strings.ToLower(os.Args[1]) {
	case "count":
		n, _, _ := procJoyGetNumDevs.Call()
		writeJSON(countReply{Count: int(n)})
	case "caps":
		if len(os.Args) < 3 {
			os.Exit(2)
		}
		slot, err := strconv.Atoi(os.Args[2])
		if err != nil || slot < 0 || slot > 255 {
			os.Exit(2)
		}
		var c joyCaps
		r, _, _ := procJoyGetDevCapsW.Call(uintptr(slot), uintptr(unsafe.Pointer(&c)), unsafe.Sizeof(c))
		rr := capsReply{Result: uint32(r)}
		if uint32(r) == 0 {
			rr.OK = true
			rr.Name = utf16SliceToString(c.Pname[:])
			rr.Xmin, rr.Xmax = c.Xmin, c.Xmax
			rr.Ymin, rr.Ymax = c.Ymin, c.Ymax
			rr.Zmin, rr.Zmax = c.Zmin, c.Zmax
			rr.Rmin, rr.Rmax = c.Rmin, c.Rmax
			rr.Umin, rr.Umax = c.Umin, c.Umax
			rr.Vmin, rr.Vmax = c.Vmin, c.Vmax
			rr.NumButtons = c.NumButtons
			rr.Caps = c.Caps
			rr.MaxAxes, rr.NumAxes, rr.MaxButtons = c.MaxAxes, c.NumAxes, c.MaxButtons
		}
		writeJSON(rr)
	default:
		os.Exit(2)
	}
}
