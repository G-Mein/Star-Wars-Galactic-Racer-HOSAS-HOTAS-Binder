//go:build windows

package main

import "testing"

func TestScrollThumbBoundsVertical(t *testing.T) {
	track := rect{Left: 0, Top: 0, Right: 10, Bottom: 400}
	top := scrollThumbBounds(track, false, 0, 400, 400, 0)
	if top.Top != 0 || top.Bottom != 200 {
		t.Fatalf("thumb at start = %+v, want 0..200", top)
	}
	bottom := scrollThumbBounds(track, false, 0, 400, 400, 400)
	if bottom.Top != 200 || bottom.Bottom != 400 {
		t.Fatalf("thumb at end = %+v, want 200..400", bottom)
	}
	clamped := scrollThumbBounds(track, false, 0, 400, 400, 9999)
	if clamped != bottom {
		t.Fatalf("out-of-range pos = %+v, want %+v", clamped, bottom)
	}
}

func TestScrollThumbBoundsMinimumAndHorizontal(t *testing.T) {
	track := rect{Left: 10, Top: 0, Right: 210, Bottom: 10}
	thumb := scrollThumbBounds(track, true, 0, 100000, 10, 0)
	if got := thumb.Right - thumb.Left; got != scrollBarMinThumb {
		t.Fatalf("thumb length = %d, want minimum %d", got, scrollBarMinThumb)
	}
	if thumb.Left != 10 || thumb.Top != 0 || thumb.Bottom != 10 {
		t.Fatalf("horizontal thumb = %+v", thumb)
	}
	full := scrollThumbBounds(track, true, 0, 0, 500, 0)
	if full != track {
		t.Fatalf("thumb without range = %+v, want full track %+v", full, track)
	}
}

func TestScrollPosFromThumbOffsetRoundTrip(t *testing.T) {
	track := rect{Top: 0, Bottom: 400, Right: 10}
	for _, pos := range []int{50, 120, 250, 400, 450} {
		thumb := scrollThumbBounds(track, false, 50, 450, 300, pos)
		got := scrollPosFromThumbOffset(int(thumb.Top), 400, int(thumb.Bottom-thumb.Top), 50, 450)
		if diff := got - pos; diff < -2 || diff > 2 {
			t.Errorf("pos %d -> thumb %+v -> %d", pos, thumb, got)
		}
	}
	if got := scrollPosFromThumbOffset(-50, 400, 100, 50, 450); got != 50 {
		t.Fatalf("negative offset = %d, want 50", got)
	}
	if got := scrollPosFromThumbOffset(900, 400, 100, 50, 450); got != 450 {
		t.Fatalf("offset past end = %d, want 450", got)
	}
	if got := scrollPosFromThumbOffset(10, 100, 100, 50, 450); got != 50 {
		t.Fatalf("no travel = %d, want 50", got)
	}
}

func TestContentViewportExcludesChromeAndScrollbars(t *testing.T) {
	client := rect{Right: 1380, Bottom: 616}
	plain := contentViewport(client, false, false)
	if plain != (rect{Left: sidebarWidth, Top: fixedProfileBottom, Right: 1380, Bottom: 616 - statusBarHeight}) {
		t.Fatalf("viewport without bars = %+v", plain)
	}
	status := statusBarBounds(client)
	if status != (rect{Left: 0, Top: 616 - statusBarHeight, Right: 1380, Bottom: 616}) {
		t.Fatalf("status bar = %+v", status)
	}
	if verticalScrollBarBounds(client).Bottom > status.Top || horizontalScrollBarBounds(client, true).Bottom > status.Top {
		t.Fatal("scrollbars must end above the status bar")
	}
	if tiny := statusBarBounds(rect{Right: 50, Bottom: 10}); tiny.Top != 0 || tiny.Bottom != 10 {
		t.Fatalf("tiny client status bar = %+v, want clamped to client", tiny)
	}
	both := contentViewport(client, true, true)
	if both.Right != verticalScrollBarBounds(client).Left {
		t.Fatalf("viewport right = %d, want vertical bar left", both.Right)
	}
	if both.Bottom != horizontalScrollBarBounds(client, true).Top {
		t.Fatalf("viewport bottom = %d, want horizontal bar top", both.Bottom)
	}
	h := horizontalScrollBarBounds(client, true)
	if h.Right != verticalScrollBarBounds(client).Left || h.Bottom-h.Top != scrollBarWidth {
		t.Fatalf("horizontal bar = %+v overlaps vertical bar or has wrong height", h)
	}
}

func TestChildClipRect(t *testing.T) {
	view := rect{Left: 200, Top: 82, Right: 1000, Bottom: 600}
	if _, full := childClipRect(rect{Left: 300, Top: 100, Right: 400, Bottom: 130}, view); !full {
		t.Fatal("child inside viewport should not be clipped")
	}
	clip, full := childClipRect(rect{Left: 300, Top: 70, Right: 400, Bottom: 100}, view)
	if full || clip != (rect{Left: 0, Top: 12, Right: 100, Bottom: 30}) {
		t.Fatalf("partial clip = %+v full=%v", clip, full)
	}
	hidden, full := childClipRect(rect{Left: 300, Top: 20, Right: 400, Bottom: 60}, view)
	if full || hidden != (rect{}) {
		t.Fatalf("hidden child clip = %+v full=%v, want empty region", hidden, full)
	}
}

func TestHorizontalScrollBarBoundsTinyClient(t *testing.T) {
	tiny := horizontalScrollBarBounds(rect{Left: 0, Top: 0, Right: 10, Bottom: 5}, false)
	if tiny.Right < tiny.Left || tiny.Bottom < tiny.Top {
		t.Fatalf("tiny client bounds inverted: %+v", tiny)
	}
}

func TestClampScrollPos(t *testing.T) {
	cases := []struct {
		name                string
		pos, min, max, want int
	}{
		{"within range", 50, 0, 100, 50},
		{"below min", -10, 0, 100, 0},
		{"above max", 150, 0, 100, 100},
		{"inverted range", 50, 100, 40, 100},
	}
	for _, tc := range cases {
		if got := clampScrollPos(tc.pos, tc.min, tc.max); got != tc.want {
			t.Fatalf("%s: clampScrollPos(%d, %d, %d) = %d, want %d", tc.name, tc.pos, tc.min, tc.max, got, tc.want)
		}
	}
}

func TestChildClipRectBottomAndSides(t *testing.T) {
	view := rect{Left: 200, Top: 82, Right: 1000, Bottom: 600}
	if clip, full := childClipRect(rect{Left: 300, Top: 580, Right: 400, Bottom: 650}, view); full || clip != (rect{Left: 0, Top: 0, Right: 100, Bottom: 20}) {
		t.Fatalf("bottom clip = %+v full=%v", clip, full)
	}
	if clip, full := childClipRect(rect{Left: 300, Top: 620, Right: 400, Bottom: 680}, view); full || clip != (rect{}) {
		t.Fatalf("hidden below = %+v full=%v", clip, full)
	}
	if clip, full := childClipRect(rect{Left: 950, Top: 100, Right: 1050, Bottom: 140}, view); full || clip != (rect{Left: 0, Top: 0, Right: 50, Bottom: 40}) {
		t.Fatalf("right clip = %+v full=%v", clip, full)
	}
	if clip, full := childClipRect(rect{Left: 150, Top: 100, Right: 250, Bottom: 140}, view); full || clip != (rect{Left: 50, Top: 0, Right: 100, Bottom: 40}) {
		t.Fatalf("left clip = %+v full=%v", clip, full)
	}
}

func TestStatusBarBoundsOffsetAndInvertedClient(t *testing.T) {
	client := rect{Left: 10, Top: 20, Right: 500, Bottom: 300}
	if got, want := statusBarBounds(client), (rect{Left: 10, Top: 300 - statusBarHeight, Right: 500, Bottom: 300}); got != want {
		t.Fatalf("statusBarBounds(%+v) = %+v, want %+v", client, got, want)
	}
	if got := statusBarBounds(rect{Left: 0, Top: 50, Right: 100, Bottom: 20}); got.Bottom < got.Top {
		t.Fatalf("statusBarBounds returned inverted rect %+v", got)
	}
}

func TestTinyClientScrollBarAndViewportNotInverted(t *testing.T) {
	tiny := rect{Right: 50, Bottom: 10}
	for _, vertical := range []bool{false, true} {
		if h := horizontalScrollBarBounds(tiny, vertical); h.Right < h.Left || h.Bottom < h.Top {
			t.Fatalf("horizontalScrollBarBounds(vertical=%v) inverted: %+v", vertical, h)
		}
		if vp := contentViewport(tiny, vertical, true); vp.Right < vp.Left || vp.Bottom < vp.Top {
			t.Fatalf("contentViewport(vertical=%v) inverted: %+v", vertical, vp)
		}
	}
}
