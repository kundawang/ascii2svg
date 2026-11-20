package ascii2svg

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Connection side bits used by line and arrow glyphs.
const (
	sideN byte = 1 << iota
	sideE
	sideS
	sideW
)

const (
	// LayerClass is the CSS class attached to the direction-layer <g>.
	LayerClass = "a2s-directions"
	// MarkerEndID is the SVG id of the arrow-head marker for path ends.
	MarkerEndID = "a2s-arrow"
	// MarkerStartID is the SVG id of the reversed marker for path starts.
	MarkerStartID = "a2s-arrow-start"

	defaultMaxRows  = 5000
	defaultMaxCols  = 2000
	defaultMaxCells = 4000000
)

// ErrGridTooLarge is returned when the input grid exceeds the configured
// row, column or cell limits.
var ErrGridTooLarge = errors.New("ascii2svg: input grid exceeds configured size limits")

var (
	// lineGlyphs maps a body glyph to the sides it connects.
	lineGlyphs = map[rune]byte{
		'-': sideE | sideW,
		'|': sideN | sideS,
		'+': sideN | sideE | sideS | sideW,
	}
	// arrowGlyphs maps an arrow glyph to the side its tip points to.
	// An arrow is a terminal cell: it only connects through its back side.
	arrowGlyphs = map[rune]byte{
		'<': sideW,
		'>': sideE,
		'^': sideN,
		'v': sideS,
		'V': sideS,
	}
	oppositeSide = map[byte]byte{
		sideN: sideS,
		sideS: sideN,
		sideE: sideW,
		sideW: sideE,
	}
	colorPattern = regexp.MustCompile(`^(?:#[0-9a-fA-F]{3,8}|[a-zA-Z]{2,20})$`)
)

type cellKind uint8

const (
	cellText cellKind = iota
	cellLine
	cellArrow
)

// gridCell describes one visual column of the input. Wide runes occupy a
// leading cell holding the rune plus one or more continuation cells.
type gridCell struct {
	r    rune
	kind cellKind
	mask byte
}

// DirectionPath is one extracted direction line: Points are the ordered
// grid coordinates (row, column) of the cells making the polyline.
// StartArrow/EndArrow tell whether an arrow head sits at the matching end.
type DirectionPath struct {
	Points     [][2]int
	StartArrow bool
	EndArrow   bool
}

// Options configures direction-line extraction and rendering.
type Options struct {
	// LineColor is the stroke and arrow-head color (named color or #rgb hex).
	LineColor string
	// LineWidth is the stroke width in SVG user units.
	LineWidth float64
	// CellWidth/CellHeight are the monospace glyph cell dimensions.
	CellWidth  float64
	CellHeight float64
	// OffsetX/OffsetY position the top-left cell on the SVG canvas.
	OffsetX float64
	OffsetY float64
	// MaxRows, MaxCols and MaxCells bound extraction so pathological input
	// cannot exhaust memory; zero means use the built-in defaults.
	MaxRows  int
	MaxCols  int
	MaxCells int
}

// DefaultOptions returns the default direction-layer configuration.
func DefaultOptions() Options {
	return Options{
		LineColor:  "red",
		LineWidth:  1.5,
		CellWidth:  9.6,
		CellHeight: 17,
		OffsetX:    24,
		OffsetY:    22,
		MaxRows:    defaultMaxRows,
		MaxCols:    defaultMaxCols,
		MaxCells:   defaultMaxCells,
	}
}

func (o Options) withDefaults() Options {
	d := DefaultOptions()
	if o.LineColor == "" {
		o.LineColor = d.LineColor
	}
	if o.LineWidth == 0 {
		o.LineWidth = d.LineWidth
	}
	if o.CellWidth == 0 {
		o.CellWidth = d.CellWidth
	}
	if o.CellHeight == 0 {
		o.CellHeight = d.CellHeight
	}
	if o.OffsetX == 0 {
		o.OffsetX = d.OffsetX
	}
	if o.OffsetY == 0 {
		o.OffsetY = d.OffsetY
	}
	if o.MaxRows == 0 {
		o.MaxRows = d.MaxRows
	}
	if o.MaxCols == 0 {
		o.MaxCols = d.MaxCols
	}
	if o.MaxCells == 0 {
		o.MaxCells = d.MaxCells
	}
	return o
}

func (o Options) validate() error {
	if !colorPattern.MatchString(o.LineColor) {
		return fmt.Errorf("ascii2svg: invalid line color %q", o.LineColor)
	}
	if o.LineWidth < 0 {
		return fmt.Errorf("ascii2svg: line width must be positive, got %v", o.LineWidth)
	}
	if o.CellWidth < 0 || o.CellHeight < 0 {
		return errors.New("ascii2svg: cell dimensions must be positive")
	}
	if o.MaxRows < 0 || o.MaxCols < 0 || o.MaxCells < 0 {
		return errors.New("ascii2svg: grid limits must be positive")
	}
	return nil
}

// withDefaults fills unset (zero-valued) fields with the built-in defaults
// and validates the resulting configuration.
func (o Options) withDefaultsAndValidate() (Options, error) {
	o = o.withDefaults()
	if err := o.validate(); err != nil {
		return Options{}, err
	}
	return o, nil
}

// runeWidth returns the number of visual columns occupied by r. Control and
// zero-width runes occupy 0 columns; East-Asian wide and full-width runes 2.
func runeWidth(r rune) int {
	if r == 0 || r < 0x20 || (r >= 0x7f && r < 0xA0) {
		return 0
	}
	switch {
	case r >= 0x1100 && r <= 0x115F, // Hangul Jamo
		r == 0x2329 || r == 0x232A,
		r >= 0x2E80 && r <= 0x303E,   // CJK radicals, Kangxi
		r >= 0x3041 && r <= 0x33FF,   // kana, CJK punctuation
		r >= 0x3400 && r <= 0x4DBF,   // CJK extension A
		r >= 0x4E00 && r <= 0x9FFF,   // CJK unified ideographs
		r >= 0xA000 && r <= 0xA4CF,   // Yi
		r >= 0xAC00 && r <= 0xD7A3,   // Hangul syllables
		r >= 0xF900 && r <= 0xFAFF,   // CJK compatibility ideographs
		r >= 0xFE30 && r <= 0xFE4F,   // CJK compatibility forms
		r >= 0xFF00 && r <= 0xFF60,   // full-width ASCII
		r >= 0xFFE0 && r <= 0xFFE6,   // full-width signs
		r >= 0x1F300 && r <= 0x1F64F, // emoji and symbols
		r >= 0x1F900 && r <= 0x1F9FF,
		r >= 0x20000 && r <= 0x3FFFD: // CJK extensions B+
		return 2
	}
	return 1
}

func classify(r rune) (cellKind, byte) {
	if mask, ok := lineGlyphs[r]; ok {
		return cellLine, mask
	}
	if tip, ok := arrowGlyphs[r]; ok {
		// The arrow only accepts a stem on the side opposite its tip.
		return cellArrow, oppositeSide[tip]
	}
	return cellText, 0
}

// buildGrid turns the textual input into a row-major grid indexed by visual
// columns, so Unicode wide runes cannot shift line coordinates.
func buildGrid(input string, opts Options) ([][]gridCell, int, error) {
	lines := strings.Split(input, "\n")
	if len(lines) > opts.MaxRows {
		return nil, 0, ErrGridTooLarge
	}
	grid := make([][]gridCell, len(lines))
	cols := 0
	for r, line := range lines {
		row := make([]gridCell, 0, len(line))
		c := 0
		for _, ch := range line {
			w := runeWidth(ch)
			if w == 0 {
				continue
			}
			kind, mask := classify(ch)
			row = append(row, gridCell{r: ch, kind: kind, mask: mask})
			c++
			for i := 1; i < w; i++ {
				// Wide-rune continuation columns are never line cells.
				row = append(row, gridCell{kind: cellText})
				c++
			}
		}
		grid[r] = row
		if c > cols {
			cols = c
		}
	}
	if cols > opts.MaxCols || len(lines)*cols > opts.MaxCells {
		return nil, 0, ErrGridTooLarge
	}
	for r := range grid {
		for len(grid[r]) < cols {
			grid[r] = append(grid[r], gridCell{kind: cellText})
		}
	}
	return grid, cols, nil
}

type neighbor struct{ to, edgeID int }

// extractGraph builds the undirected graph of line/arrow cells. An edge
// between two orthogonally adjacent cells exists only when both glyphs
// advertise the matching connection sides, so text and box decoration
// cannot create phantom connections.
func extractGraph(grid [][]gridCell, cols int) (adj [][]neighbor, edges [][2]int, isArrow []bool, positions [][2]int) {
	rows := len(grid)
	nodeID := make([][]int, rows)
	for r := range nodeID {
		nodeID[r] = make([]int, cols)
		for c := range nodeID[r] {
			nodeID[r][c] = -1
		}
	}
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			if grid[r][c].kind == cellText {
				continue
			}
			id := len(positions)
			nodeID[r][c] = id
			positions = append(positions, [2]int{r, c})
			isArrow = append(isArrow, grid[r][c].kind == cellArrow)
			adj = append(adj, nil)
		}
	}
	addEdge := func(a, b int) {
		id := len(edges)
		edges = append(edges, [2]int{a, b})
		adj[a] = append(adj[a], neighbor{to: b, edgeID: id})
		adj[b] = append(adj[b], neighbor{to: a, edgeID: id})
	}
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			cell := grid[r][c]
			if cell.kind == cellText {
				continue
			}
			if c+1 < cols && grid[r][c+1].kind != cellText &&
				cell.mask&sideE != 0 && grid[r][c+1].mask&sideW != 0 {
				addEdge(nodeID[r][c], nodeID[r][c+1])
			}
			if r+1 < rows && grid[r+1][c].kind != cellText &&
				cell.mask&sideS != 0 && grid[r+1][c].mask&sideN != 0 {
				addEdge(nodeID[r][c], nodeID[r+1][c])
			}
		}
	}
	return adj, edges, isArrow, positions
}

// findBridges runs iterative Tarjan's algorithm on the given adjacency and
// reports bridge edge IDs. Iteration (instead of recursion) keeps
// arbitrarily long snake lines from growing the goroutine stack.
func findBridges(adj [][]neighbor, edgeCount int) []bool {
	disc := make([]int, len(adj))
	low := make([]int, len(adj))
	for i := range disc {
		disc[i] = -1
	}
	isBridge := make([]bool, edgeCount)
	timer := 0
	type frame struct {
		v, parent, parentEdge, idx int
	}
	for root := range adj {
		if disc[root] != -1 {
			continue
		}
		disc[root] = timer
		low[root] = timer
		timer++
		stack := []frame{{v: root, parent: -1, parentEdge: -1}}
		for len(stack) > 0 {
			f := &stack[len(stack)-1]
			if f.idx < len(adj[f.v]) {
				nb := adj[f.v][f.idx]
				f.idx++
				if nb.to == f.parent {
					continue
				}
				if disc[nb.to] == -1 {
					disc[nb.to] = timer
					low[nb.to] = timer
					timer++
					stack = append(stack, frame{v: nb.to, parent: f.v, parentEdge: nb.edgeID})
				} else if disc[nb.to] < low[f.v] {
					low[f.v] = disc[nb.to]
				}
				continue
			}
			v := f.v
			parent := f.parent
			parentEdge := f.parentEdge
			stack = stack[:len(stack)-1]
			if parent == -1 {
				continue
			}
			if low[v] < low[parent] {
				low[parent] = low[v]
			}
			if low[v] > disc[parent] {
				isBridge[parentEdge] = true
			}
		}
	}
	return isBridge
}

// boxEdges marks edges belonging to arrow-less frames. Arrow nodes are
// removed before the bridge analysis: an edge that lies on a cycle in the
// arrow-free view is part of a box border and must not reach the direction
// layer, while an arrow injected into a frame keeps its two incident edges
// out of the stripped graph so they stay visible.
func boxEdges(adj [][]neighbor, edgeCount int, isArrow []bool) []bool {
	stripped := make([][]neighbor, len(adj))
	edgeLive := make([]bool, edgeCount)
	for v := range adj {
		if isArrow[v] {
			continue
		}
		for _, nb := range adj[v] {
			if isArrow[nb.to] {
				continue
			}
			stripped[v] = append(stripped[v], nb)
			edgeLive[nb.edgeID] = true
		}
	}
	bridges := findBridges(stripped, edgeCount)
	isBox := make([]bool, edgeCount)
	for edgeID := 0; edgeID < edgeCount; edgeID++ {
		// Edges touching an arrow (absent from the stripped graph) can never
		// be box borders; a live edge is a box edge only when not a bridge.
		isBox[edgeID] = edgeLive[edgeID] && !bridges[edgeID]
	}
	return isBox
}

// extractPaths extracts the direction polylines.
//
// First, box-border edges (cycles with no arrow) are removed. The remaining
// graph is a forest of stems, forks and arrows. It is split at every node
// whose degree is not 2 (arrow heads, dangling ends, forks and crossings);
// degree-2 nodes, including corners, stay inside one polyline so turns and
// intersections remain geometrically connected. Only polylines carrying at
// least one arrow head are returned, so arrow-less dangling branches and
// plain line work never reach the direction layer.
func extractPaths(adj [][]neighbor, edges [][2]int, isArrow []bool, positions [][2]int) []DirectionPath {
	isBox := boxEdges(adj, len(edges), isArrow)
	kept := make([][]neighbor, len(adj))
	for edgeID, endpoints := range edges {
		if isBox[edgeID] {
			continue
		}
		a, b := endpoints[0], endpoints[1]
		kept[a] = append(kept[a], neighbor{to: b, edgeID: edgeID})
		kept[b] = append(kept[b], neighbor{to: a, edgeID: edgeID})
	}
	visited := make([]bool, len(edges))
	var paths []DirectionPath
	// Arrow glyphs are always terminals, even when their surviving degree is
	// 1: a walk entering an arrow ends there. Degree != 2 additionally covers
	// forks, crossings and dangling ends; degree-2 cells, including corners,
	// stay inside one polyline so turns and intersections stay connected.
	isTerminal := func(node int) bool {
		return isArrow[node] || len(kept[node]) != 2
	}
	walk := func(start, firstTo, edgeID int) DirectionPath {
		nodeSeq := []int{start}
		prev, cur := start, firstTo
		visited[edgeID] = true
		for {
			nodeSeq = append(nodeSeq, cur)
			if isTerminal(cur) {
				break
			}
			next := kept[cur][0]
			if next.to == prev {
				next = kept[cur][1]
			}
			visited[next.edgeID] = true
			prev, cur = cur, next.to
		}
		points := make([][2]int, len(nodeSeq))
		for i, node := range nodeSeq {
			points[i] = positions[node]
		}
		return DirectionPath{
			Points:     points,
			StartArrow: isArrow[nodeSeq[0]],
			EndArrow:   isArrow[nodeSeq[len(nodeSeq)-1]],
		}
	}
	// Terminals own every incident edge and start all walks; degree-2-only
	// cycles (arrow-less loops) were already removed as box edges.
	for start := 0; start < len(kept); start++ {
		if !isTerminal(start) {
			continue
		}
		for _, nb := range kept[start] {
			if visited[nb.edgeID] {
				continue
			}
			path := walk(start, nb.to, nb.edgeID)
			if path.StartArrow || path.EndArrow {
				paths = append(paths, path)
			}
		}
	}
	return paths
}

// ExtractDirections recognizes the horizontal/vertical direction lines and
// arrow ends in input and returns them as grid-coordinate polylines.
func ExtractDirections(input string, opts Options) ([]DirectionPath, error) {
	var err error
	if opts, err = opts.withDefaultsAndValidate(); err != nil {
		return nil, err
	}
	grid, cols, err := buildGrid(input, opts)
	if err != nil {
		return nil, err
	}
	adj, edges, isArrow, positions := extractGraph(grid, cols)
	if len(edges) == 0 {
		// No connected line work at all: nothing to render.
		return nil, nil
	}
	return extractPaths(adj, edges, isArrow, positions), nil
}

func formatFloat(v float64) string {
	s := strconv.FormatFloat(v, 'f', 2, 64)
	s = strings.TrimRight(s, "0")
	return strings.TrimRight(s, ".")
}

func (o Options) pointX(col int) string {
	return formatFloat(o.OffsetX + (float64(col)+0.5)*o.CellWidth)
}

func (o Options) pointY(row int) string {
	return formatFloat(o.OffsetY + (float64(row)+0.5)*o.CellHeight)
}

// RenderDirectionLayer renders the extracted paths as an independent,
// separately styleable SVG <g> fragment with its own marker definitions.
func RenderDirectionLayer(paths []DirectionPath, opts Options) string {
	opts = opts.withDefaults()
	var b strings.Builder
	put := func(s string) { _, _ = b.WriteString(s) }

	fmt.Fprintf(&b, `  <g class="%s" stroke="%s" stroke-width="%s" fill="none" stroke-linecap="round" stroke-linejoin="round">`+"\n",
		LayerClass, opts.LineColor, formatFloat(opts.LineWidth))

	needStart, needEnd := false, false
	for _, p := range paths {
		needStart = needStart || p.StartArrow
		needEnd = needEnd || p.EndArrow
	}
	if needStart || needEnd {
		put("    <defs>\n")
		if needEnd {
			fmt.Fprintf(&b, "      <marker id=\"%s\" viewBox=\"0 0 10 10\" refX=\"9\" refY=\"5\" markerWidth=\"4\" markerHeight=\"4\" orient=\"auto\">\n", MarkerEndID)
			fmt.Fprintf(&b, "        <path d=\"M 0 0 L 10 5 L 0 10 z\" fill=\"%s\"></path>\n", opts.LineColor)
			put("      </marker>\n")
		}
		if needStart {
			fmt.Fprintf(&b, "      <marker id=\"%s\" viewBox=\"0 0 10 10\" refX=\"9\" refY=\"5\" markerWidth=\"4\" markerHeight=\"4\" orient=\"auto\">\n", MarkerStartID)
			fmt.Fprintf(&b, "        <path d=\"M 10 0 L 0 5 L 10 10 z\" fill=\"%s\"></path>\n", opts.LineColor)
			put("      </marker>\n")
		}
		put("    </defs>\n")
	}

	for _, p := range paths {
		var d strings.Builder
		for i, pt := range p.Points {
			if i == 0 {
				_, _ = d.WriteString("M ")
			} else {
				_, _ = d.WriteString(" L ")
			}
			_, _ = d.WriteString(opts.pointX(pt[1]))
			_, _ = d.WriteString(" ")
			_, _ = d.WriteString(opts.pointY(pt[0]))
		}
		put(`    <path d="`)
		put(d.String())
		put(`"`)
		if p.StartArrow {
			fmt.Fprintf(&b, " marker-start=\"url(#%s)\"", MarkerStartID)
		}
		if p.EndArrow {
			fmt.Fprintf(&b, " marker-end=\"url(#%s)\"", MarkerEndID)
		}
		put("></path>\n")
	}
	put("  </g>\n")
	return b.String()
}
