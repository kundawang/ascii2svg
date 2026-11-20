package ascii2svg

import (
	"strings"
	"testing"
)

// findPath returns the path having want as one of its two endpoints.
func findPath(t *testing.T, paths []DirectionPath, want [2]int) *DirectionPath {
	t.Helper()
	for i := range paths {
		p := &paths[i]
		first, last := p.Points[0], p.Points[len(p.Points)-1]
		if first == want || last == want {
			return p
		}
	}
	return nil
}

// fixedDiagram contains a fork (degree-3 junction), text inside two boxes,
// bidirectional arrow heads and an arrow-less dangling branch that must
// stay out of the direction layer.
//
//	+-----------+
//	|   START   |
//	+-----+-----+
//	      |
//	      v
//	 <----+---->
//	      |
//	      |
//	------+------
//	      |
//	      v
//	+-----+-----+
//	|   END     |
//	+-----------+
func fixedDiagram() string {
	return strings.Join([]string{
		"+-----------+", // 0
		"|   START   |", // 1
		"+-----+-----+", // 2
		"      |",       // 3
		"      v",       // 4
		" <----+---->",  // 5
		"      |",       // 6
		"      |",       // 7
		"------+------", // 8
		"      |",       // 9
		"      v",       // 10
		"+-----+-----+", // 11
		"|   END     |", // 12
		"+-----------+", // 13
	}, "\n")
}

func TestExtractDirectionsFixedDiagram(t *testing.T) {
	paths, err := ExtractDirections(fixedDiagram(), Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Four arrow-headed paths: up stem, left & right fork arms, down stem.
	// The arrow-less dangling branch on row 8 produces no path.
	if got, want := len(paths), 4; got != want {
		t.Fatalf("path count = %d, want %d (paths: %+v)", got, want, paths)
	}

	// Fork junction at row 5, col 6.
	junction := [2]int{5, 6}

	// Dangling arrow above: (4,6) 'v' -> (3,6) '|' -> (2,6) box corner.
	up := findPath(t, paths, [2]int{4, 6})
	if up == nil {
		t.Fatal("missing path ending at dangling arrow (4,6)")
	}
	if !up.EndArrow && !up.StartArrow {
		t.Errorf("dangling-arrow path has no arrow marker: %+v", up)
	}
	if up.Points[len(up.Points)-1] != [2]int{2, 6} && up.Points[0] != [2]int{2, 6} {
		t.Errorf("dangling stem does not reach box corner (2,6): %+v", up.Points)
	}

	// Left arm: '<' at (5,1) reaches the junction.
	left := findPath(t, paths, [2]int{5, 1})
	if left == nil {
		t.Fatal("missing path starting at left arrow (5,1)")
	}
	if !left.StartArrow {
		t.Errorf("left arm missing StartArrow: %+v", left)
	}
	if left.Points[len(left.Points)-1] != junction && left.Points[0] != junction {
		t.Errorf("left arm does not end at fork %v: %+v", junction, left.Points)
	}

	// Right arm: '>' at (5,11) reaches the same junction.
	right := findPath(t, paths, [2]int{5, 11})
	if right == nil {
		t.Fatal("missing path ending at right arrow (5,11)")
	}
	if !right.EndArrow {
		t.Errorf("right arm missing EndArrow: %+v", right)
	}
	if right.Points[0] != junction && right.Points[len(right.Points)-1] != junction {
		t.Errorf("right arm does not start at fork %v: %+v", junction, right.Points)
	}

	// Down stem starts at the row-8 junction (the fork above splits the
	// network into separate arms by design) and ends at arrow (10,6).
	down := findPath(t, paths, [2]int{10, 6})
	if down == nil {
		t.Fatal("missing downward path ending at arrow (10,6)")
	}
	if !down.StartArrow && !down.EndArrow {
		t.Errorf("down stem lost its arrow marker: %+v", down)
	}
	if down.Points[0] != [2]int{8, 6} {
		t.Errorf("down stem must start at junction (8,6), got %v", down.Points[0])
	}
	foundSecondJunction := false
	for _, pt := range down.Points {
		if pt == [2]int{8, 6} {
			foundSecondJunction = true
		}
	}
	if !foundSecondJunction {
		t.Errorf("down stem lost the row-8 junction: %+v", down.Points)
	}

	// Box borders and the text "START"/"END" must not produce extra paths:
	// no path point may land inside either box's text rows.
	for _, p := range paths {
		for _, pt := range p.Points {
			if pt[0] == 1 || pt[0] == 12 {
				t.Errorf("box/text row leaked into direction layer: %v in %+v", pt, p)
			}
		}
	}
	// The arrow-less dangling branch cells (row 8, cols 0..5) never appear.
	for _, p := range paths {
		for _, pt := range p.Points {
			if pt[0] == 8 && pt[1] < 6 {
				t.Errorf("arrow-less dangling branch leaked into path: %+v", p.Points)
			}
		}
	}
}

func TestBidirectionalArrowSinglePath(t *testing.T) {
	// A lone double-headed arrow is one path carrying both markers.
	paths, err := ExtractDirections("<--->", Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("path count = %d, want 1: %+v", len(paths), paths)
	}
	p := paths[0]
	if !p.StartArrow || !p.EndArrow {
		t.Errorf("bidirectional arrow markers missing: %+v", p)
	}
	if p.Points[0] != [2]int{0, 0} || p.Points[len(p.Points)-1] != [2]int{0, 4} {
		t.Errorf("bidirectional endpoints wrong: %+v", p.Points)
	}
}

func TestCornerConnectsTwoArrows(t *testing.T) {
	input := strings.Join([]string{
		"+--->",
		"|",
		"v",
	}, "\n")
	paths, err := ExtractDirections(input, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("corner split the line into %d paths, want 1: %+v", len(paths), paths)
	}
	p := paths[0]
	// The degree-2 '+' corner stays inside the single polyline.
	found := false
	for _, pt := range p.Points {
		if pt == [2]int{0, 0} {
			found = true
		}
	}
	if !found {
		t.Errorf("corner (0,0) missing from polyline: %+v", p.Points)
	}
	if !p.StartArrow || !p.EndArrow {
		t.Errorf("both arrow ends expected on corner path: %+v", p)
	}
}

func TestIntersectionFork(t *testing.T) {
	// Three arrows around a degree-4 '+' crossing, plus one arrow-less
	// dangling arm that must be filtered out of the direction layer.
	input := strings.Join([]string{
		"    ^",
		"    |",
		"<---+--->",
		"    |",
	}, "\n")
	paths, err := ExtractDirections(input, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 3 {
		t.Fatalf("intersection produced %d paths, want 3: %+v", len(paths), paths)
	}
	junction := [2]int{2, 4}
	for _, p := range paths {
		first, last := p.Points[0], p.Points[len(p.Points)-1]
		if first != junction && last != junction {
			t.Errorf("path does not touch junction %v: %+v", junction, p.Points)
		}
		if !p.StartArrow && !p.EndArrow {
			t.Errorf("fork arm missing its arrow marker: %+v", p)
		}
	}
	// Endpoints must be exactly the three arrow cells (0,4), (2,0), (2,8).
	wantArrows := map[[2]int]bool{{0, 4}: true, {2, 0}: true, {2, 8}: true}
	gotArrows := map[[2]int]bool{}
	for _, p := range paths {
		first, last := p.Points[0], p.Points[len(p.Points)-1]
		if p.StartArrow {
			gotArrows[first] = true
		}
		if p.EndArrow {
			gotArrows[last] = true
		}
	}
	if len(gotArrows) != len(wantArrows) {
		t.Errorf("arrow endpoints = %v, want %v", gotArrows, wantArrows)
	}
	for arrow := range wantArrows {
		if !gotArrows[arrow] {
			t.Errorf("missing arrow endpoint %v in %v", arrow, gotArrows)
		}
	}
}

func TestPlainTextAndBoxesNotRecognized(t *testing.T) {
	cases := []string{
		"if a < b and c > d { return }",
		"-------",
		"|||||||",
		strings.Join([]string{
			"+-------+",
			"| hello |",
			"+-------+",
		}, "\n"),
		"",
		"plain text without any line glyphs",
	}
	for i, input := range cases {
		paths, err := ExtractDirections(input, Options{})
		if err != nil {
			t.Errorf("case %d: unexpected error: %v", i, err)
		}
		if len(paths) != 0 {
			t.Errorf("case %d (%q): expected no paths, got %+v", i, input, paths)
		}
	}
}

func TestDanglingArrowWithoutStem(t *testing.T) {
	// An arrow glyph with no matching stem is just text, not a path.
	for _, input := range []string{">", "v", "a > b", "  <  "} {
		paths, err := ExtractDirections(input, Options{})
		if err != nil {
			t.Errorf("input %q: unexpected error: %v", input, err)
		}
		if len(paths) != 0 {
			t.Errorf("input %q: dangling arrow produced paths: %+v", input, paths)
		}
	}
}

func TestClosedLoopWithArrow(t *testing.T) {
	// A single arrow injected into an otherwise closed frame produces one
	// arrow-headed polyline; the arrow-less frame edges on the other side of
	// the arrow are dropped as box borders.
	input := strings.Join([]string{
		"+--->+",
		"|    |",
		"+----+",
	}, "\n")
	paths, err := ExtractDirections(input, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("loop+arrow produced %d paths, want 1: %+v", len(paths), paths)
	}
	p := paths[0]
	arrow := [2]int{0, 4}
	if p.Points[0] != arrow {
		t.Errorf("path must start at injected arrow %v, got %v", arrow, p.Points[0])
	}
	if !p.StartArrow {
		t.Errorf("loop path missing arrow marker: %+v", p)
	}
	// The polyline walks the frame away from the arrow and stops at the cell
	// directly past it, never forming an arrow-less closed segment.
	if p.EndArrow {
		t.Errorf("single-arrow loop cannot carry an end marker: %+v", p)
	}
}

func TestWideUnicodeKeepsCoordinatesAligned(t *testing.T) {
	// '中' occupies visual columns 0-1; the line work starts at col 2.
	input := strings.Join([]string{
		"中+>",
		"A |",
	}, "\n")
	paths, err := ExtractDirections(input, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("wide-char diagram produced %d paths, want 1: %+v", len(paths), paths)
	}
	p := paths[0]
	want := map[[2]int]bool{
		{0, 3}: true, // '>' shifted past the two-column wide rune
		{0, 2}: true, // '+'
		{1, 2}: true, // '|'
	}
	if len(p.Points) != len(want) {
		t.Fatalf("point count = %d, want %d: %+v", len(p.Points), len(want), p.Points)
	}
	for _, pt := range p.Points {
		if !want[pt] {
			t.Errorf("unexpected point %v, want one of %v", pt, want)
		}
	}
	if !p.StartArrow && !p.EndArrow {
		t.Errorf("wide-char path lost its arrow: %+v", p)
	}
}

func TestEmptyInput(t *testing.T) {
	paths, err := ExtractDirections("", Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if paths != nil {
		t.Errorf("empty input paths = %+v, want nil", paths)
	}
	out, err := FromString("")
	if err != nil {
		t.Fatalf("FromString failed: %v", err)
	}
	if strings.Contains(out, LayerClass) {
		t.Errorf("empty input must not emit a direction layer:\n%s", out)
	}
	if !strings.Contains(out, "<pre></pre>") {
		t.Errorf("default rendering altered for empty input:\n%s", out)
	}
}

func TestHugeGridRejectedAndBoundedWork(t *testing.T) {
	// Explicitly tiny limits turn a normal input into a rejected huge grid.
	big := strings.Repeat(strings.Repeat("-", 50)+"\n", 20)
	if _, err := ExtractDirections(big, Options{MaxRows: 10, MaxCols: 10, MaxCells: 100}); err != ErrGridTooLarge {
		t.Fatalf("want ErrGridTooLarge, got %v", err)
	}

	// A large grid must still complete quickly with default limits.
	row := strings.Repeat("-", 299) + ">"
	var b strings.Builder
	for r := 0; r < 300; r++ {
		b.WriteString(row)
		b.WriteByte('\n')
	}
	paths, err := ExtractDirections(b.String(), Options{})
	if err != nil {
		t.Fatalf("unexpected error on large grid: %v", err)
	}
	if len(paths) == 0 {
		t.Error("large grid produced no paths")
	}
}

func TestInvalidOptions(t *testing.T) {
	input := "->\n"
	if _, err := ExtractDirections(input, Options{LineColor: "not a color"}); err == nil {
		t.Error("invalid color accepted")
	}
	if _, err := ExtractDirections(input, Options{LineWidth: -2}); err == nil {
		t.Error("negative line width accepted")
	}
	if _, err := ExtractDirections(input, Options{CellWidth: -1}); err == nil {
		t.Error("negative cell width accepted")
	}
}

func TestRenderDirectionLayer(t *testing.T) {
	paths, err := ExtractDirections("<--->", Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	opts := Options{LineColor: "#00ff00", LineWidth: 3}
	layer := RenderDirectionLayer(paths, opts)

	wantSubstrings := []string{
		`<g class="` + LayerClass + `"`,
		`stroke="#00ff00"`,
		`stroke-width="3"`,
		`marker-start="url(#` + MarkerStartID + `)"`,
		`marker-end="url(#` + MarkerEndID + `)"`,
		`<marker id="` + MarkerStartID + `"`,
		`<marker id="` + MarkerEndID + `"`,
		`<path d="M `,
		" L ",
		"</g>",
	}
	for _, s := range wantSubstrings {
		if !strings.Contains(layer, s) {
			t.Errorf("layer missing %q:\n%s", s, layer)
		}
	}

	// Independent layer coexists with, but never mutates, the default render.
	out, err := FromStringWithOptions("A -> B", opts)
	if err != nil {
		t.Fatalf("FromStringWithOptions failed: %v", err)
	}
	if !strings.Contains(out, "<foreignObject") || !strings.Contains(out, "<pre>A -&gt; B</pre>") {
		t.Errorf("default rendering missing or altered:\n%s", out)
	}
	if !strings.Contains(out, LayerClass) {
		t.Errorf("direction layer missing from full SVG:\n%s", out)
	}
}

func TestRenderMarkerGeometryFollowsGrid(t *testing.T) {
	paths, err := ExtractDirections("->", Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("simple arrow produced no paths")
	}
	opts := Options{LineColor: "blue", LineWidth: 2, CellWidth: 10, CellHeight: 10, OffsetX: 2, OffsetY: 4}
	layer := RenderDirectionLayer(paths, opts)
	// Cell centers: x = 2 + (col+0.5)*10, y = 4 + (row+0.5)*10.
	want := `d="M 7 9 L 17 9"`
	if !strings.Contains(layer, want) {
		t.Errorf("geometry %q not found in:\n%s", want, layer)
	}
}
