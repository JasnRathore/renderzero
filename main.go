// ============================================================
//  VOID VIEWER — 3D Model Viewer
//  Go + raylib-go | Windows only
//  UI: Blender-inspired | Aesthetic: Zhenya Rynzhuk dark
// ============================================================
//
//  CONTROLS
//  --------
//  Middle Mouse Drag       Orbit
//  Shift + MMB Drag        Pan
//  Scroll Wheel            Zoom
//  O / Ctrl+O              Open file dialog
//  Drag & Drop             Load OBJ / FBX / GLTF
//  Numpad 1/3/7            Front / Right / Top view
//  Ctrl + Numpad 1/3/7     Back / Left / Bottom
//  Numpad 0                Perspective reset
//  Numpad 5                Toggle ortho / persp
//  . (Period)              Focus on model
//  G / W / A               Grid / Wire overlay / Axes
//  Z                       Cycle shading mode
//  F11                     Fullscreen
// ============================================================

package main

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"unsafe"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// ── Layout constants ──────────────────────────────────────────
const (
	topbarH     = int32(36)
	bottombarH  = int32(22)
	panelRW     = int32(270)
	fontSize    = int32(13)
	fontSizeSm  = int32(11)
	fontSizeLg  = int32(15)
	pad         = int32(8)
	btnH        = int32(26)
	gizmoSize   = float32(70)
	gizmoMargin = float32(14)
)

// ── Palette — Zhenya Rynzhuk dark ─────────────────────────────
var (
	cBG        = rl.Color{R: 8, G: 8, B: 13, A: 255}
	cPanel     = rl.Color{R: 14, G: 14, B: 20, A: 255}
	cBorder    = rl.Color{R: 30, G: 30, B: 46, A: 255}
	cBorderLo  = rl.Color{R: 20, G: 20, B: 32, A: 255}
	cAccent    = rl.Color{R: 0, G: 204, B: 255, A: 255}
	cAccentDim = rl.Color{R: 0, G: 140, B: 178, A: 255}
	cPurple    = rl.Color{R: 120, G: 40, B: 255, A: 255}
	cPurpleDim = rl.Color{R: 70, G: 20, B: 180, A: 255}
	cText      = rl.Color{R: 210, G: 210, B: 228, A: 255}
	cTextDim   = rl.Color{R: 90, G: 90, B: 120, A: 255}
	cTextBrt   = rl.Color{R: 240, G: 240, B: 255, A: 255}
	cHover     = rl.Color{R: 22, G: 22, B: 34, A: 255}
	cGridMin   = rl.Color{R: 22, G: 22, B: 35, A: 255}
	cGridMaj   = rl.Color{R: 38, G: 38, B: 60, A: 255}
	cAxisX     = rl.Color{R: 180, G: 50, B: 50, A: 180}
	cAxisY     = rl.Color{R: 50, G: 180, B: 50, A: 180}
	cAxisZ     = rl.Color{R: 50, G: 50, B: 180, A: 180}
)

// ── Enums ─────────────────────────────────────────────────────
type ShadingMode int

const (
	ShadeSolid ShadingMode = iota
	ShadeWire
	ShadeMaterial
	ShadeCount
)

type PresetView int

const (
	ViewPersp PresetView = iota
	ViewFront
	ViewBack
	ViewRight
	ViewLeft
	ViewTop
	ViewBottom
)

// ── State structs ─────────────────────────────────────────────
type OrbitCam struct {
	target     rl.Vector3
	azimuth    float32
	elevation  float32
	distance   float32
	fovy       float32
	ortho      bool
	dragStart  rl.Vector2
	panOrigin  rl.Vector3
	dragging   bool
	panning    bool
}

type ModelState struct {
	model         rl.Model
	loaded        bool
	name          string
	ext           string
	bounds        rl.BoundingBox
	center        rl.Vector3
	scaleFactor   float32
	meshCount     int
	vertexCount   int
	triCount      int
}

type UIState struct {
	shading     ShadingMode
	view        PresetView
	grid        bool
	axes        bool
	wireOver    bool
	stats       bool
	rightPanel  bool
	lightAz     float32
	lightEl     float32
	ambient     float32
	secObject   bool
	secTransform bool
	secDisplay  bool
	secLighting bool
	statusMsg   string
	statusTimer float32
}

// ── Shaders ───────────────────────────────────────────────────
const vsSource = `#version 330 core
in vec3 vertexPosition;
in vec3 vertexNormal;
in vec2 vertexTexCoord;
in vec4 vertexColor;
uniform mat4 mvp;
uniform mat4 matModel;
out vec3 vPos;
out vec3 vNorm;
out vec2 vUV;
void main() {
    vPos  = vec3(matModel * vec4(vertexPosition, 1.0));
    mat3  nm  = transpose(inverse(mat3(matModel)));
    vNorm = normalize(nm * vertexNormal);
    vUV   = vertexTexCoord;
    gl_Position = mvp * vec4(vertexPosition, 1.0);
}`

const fsSource = `#version 330 core
in vec3 vPos;
in vec3 vNorm;
in vec2 vUV;
uniform sampler2D texture0;
uniform vec4  colDiffuse;
uniform vec3  uLightDir;
uniform vec3  uLightColor;
uniform float uAmbient;
uniform vec3  uViewPos;
out vec4 fragColor;
void main() {
    vec4  tex  = texture(texture0, vUV) * colDiffuse;
    vec3  N    = normalize(vNorm);
    vec3  L    = normalize(-uLightDir);
    float diff = max(dot(N, L), 0.0);
    vec3  V    = normalize(uViewPos - vPos);
    vec3  H    = normalize(L + V);
    float spec = pow(max(dot(N, H), 0.0), 48.0) * 0.25;
    vec3  lit  = (uAmbient + diff + spec) * uLightColor;
    fragColor  = vec4(clamp(lit, 0.0, 1.0) * tex.rgb, tex.a);
}`

// ── Globals ───────────────────────────────────────────────────
var (
	cam           OrbitCam
	obj           ModelState
	ui            UIState
	shd           rl.Shader
	locLightDir   int32
	locLightColor int32
	locAmbient    int32
	locViewPos    int32
	locMatModel   int32
)

// ── Helpers ───────────────────────────────────────────────────
func setStatus(f string, a ...any) {
	ui.statusMsg = fmt.Sprintf(f, a...)
	ui.statusTimer = 4.0
}

func vpRect(sw, sh int32) rl.Rectangle {
	pw := int32(0)
	if ui.rightPanel {
		pw = panelRW
	}
	return rl.Rectangle{X: 0, Y: float32(topbarH), Width: float32(sw - pw), Height: float32(sh - topbarH - bottombarH)}
}

func panelRect(sw, sh int32) rl.Rectangle {
	return rl.Rectangle{X: float32(sw - panelRW), Y: float32(topbarH), Width: float32(panelRW), Height: float32(sh - topbarH - bottombarH)}
}

// ── Camera ────────────────────────────────────────────────────
func camInit() {
	cam = OrbitCam{azimuth: 0.785, elevation: 0.524, distance: 6, fovy: 60}
}

func camGet() rl.Camera3D {
	x := float32(math.Sin(float64(cam.azimuth))) * float32(math.Cos(float64(cam.elevation)))
	y := float32(math.Sin(float64(cam.elevation)))
	z := float32(math.Cos(float64(cam.azimuth))) * float32(math.Cos(float64(cam.elevation)))
	dir := rl.Vector3{X: x, Y: y, Z: z}
	proj := rl.CameraProjection(rl.CameraPerspective)
	if cam.ortho {
		proj = rl.CameraProjection(rl.CameraOrthographic)
	}
	return rl.Camera3D{
		Position:   rl.Vector3Add(cam.target, rl.Vector3Scale(dir, cam.distance)),
		Target:     cam.target,
		Up:         rl.Vector3{X: 0, Y: 1, Z: 0},
		Fovy:       cam.fovy,
		Projection: proj,
	}
}

func camFocus() {
	if !obj.loaded {
		return
	}
	cam.target = obj.center
	sz := rl.Vector3Subtract(obj.bounds.Max, obj.bounds.Min)
	cam.distance = float32(math.Max(float64(rl.Vector3Length(sz)*1.4), 0.5))
}

func camPreset(v PresetView) {
	ui.view = v
	switch v {
	case ViewFront:
		cam.azimuth, cam.elevation, cam.ortho = 0, 0, true
	case ViewBack:
		cam.azimuth, cam.elevation, cam.ortho = math.Pi, 0, true
	case ViewRight:
		cam.azimuth, cam.elevation, cam.ortho = math.Pi/2, 0, true
	case ViewLeft:
		cam.azimuth, cam.elevation, cam.ortho = -math.Pi/2, 0, true
	case ViewTop:
		cam.azimuth, cam.elevation, cam.ortho = 0, 1.55, true
	case ViewBottom:
		cam.azimuth, cam.elevation, cam.ortho = 0, -1.55, true
	default:
		cam.ortho = false
	}
}

func camUpdate(vp rl.Rectangle) {
	mouse := rl.GetMousePosition()
	scroll := rl.GetMouseWheelMove()
	mmb := rl.IsMouseButtonDown(rl.MouseButtonMiddle)
	shift := rl.IsKeyDown(rl.KeyLeftShift) || rl.IsKeyDown(rl.KeyRightShift)
	inVP := rl.CheckCollisionPointRec(mouse, vp)

	if inVP && scroll != 0 {
		cam.distance -= scroll * cam.distance * 0.08
		if cam.distance < 0.05 {
			cam.distance = 0.05
		}
	}

	if mmb && inVP {
		if !cam.dragging && !cam.panning {
			cam.dragStart = mouse
			cam.panOrigin = cam.target
			cam.dragging = !shift
			cam.panning = shift
		}
	} else {
		cam.dragging = false
		cam.panning = false
	}

	if cam.dragging {
		d := rl.Vector2Subtract(mouse, cam.dragStart)
		cam.azimuth -= d.X * 0.005
		cam.elevation = clamp32(cam.elevation+d.Y*0.005, -1.55, 1.55)
		cam.dragStart = mouse
		cam.ortho = false
		ui.view = ViewPersp
	}

	if cam.panning {
		c := camGet()
		fwd := rl.Vector3Normalize(rl.Vector3Subtract(c.Target, c.Position))
		right := rl.Vector3CrossProduct(fwd, c.Up)
		up := rl.Vector3CrossProduct(right, fwd)
		d := rl.Vector2Subtract(mouse, cam.dragStart)
		s := cam.distance * 0.002
		cam.target = rl.Vector3Subtract(cam.panOrigin,
			rl.Vector3Add(rl.Vector3Scale(right, d.X*s), rl.Vector3Scale(up, -d.Y*s)))
	}
}

func clamp32(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ── Shader ────────────────────────────────────────────────────
func shaderInit() {
	shd = rl.LoadShaderFromMemory(vsSource, fsSource)
	locLightDir = rl.GetShaderLocation(shd, "uLightDir")
	locLightColor = rl.GetShaderLocation(shd, "uLightColor")
	locAmbient = rl.GetShaderLocation(shd, "uAmbient")
	locViewPos = rl.GetShaderLocation(shd, "uViewPos")
	locMatModel = rl.GetShaderLocation(shd, "matModel")

	// Tell raylib which shader slot carries the model matrix and view pos
	setShaderLoc(&shd, rl.ShaderLocMatrixModel, locMatModel)
	setShaderLoc(&shd, rl.ShaderLocVectorView, locViewPos)
}

// setShaderLoc writes into the C-backed Locs array of the shader.
func setShaderLoc(s *rl.Shader, slot, value int32) {
	if s.Locs == nil {
		return
	}
	locPtr := (*int32)(unsafe.Pointer(
		uintptr(unsafe.Pointer(s.Locs)) + uintptr(slot)*unsafe.Sizeof(int32(0))))
	*locPtr = value
}

func shaderUpdate(c rl.Camera3D) {
	lx := float32(math.Sin(float64(ui.lightAz))) * float32(math.Cos(float64(ui.lightEl)))
	ly := float32(math.Sin(float64(ui.lightEl)))
	lz := float32(math.Cos(float64(ui.lightAz))) * float32(math.Cos(float64(ui.lightEl)))

	ld := [3]float32{lx, ly, lz}
	lc := [3]float32{1, 1, 1}
	vp := [3]float32{c.Position.X, c.Position.Y, c.Position.Z}

	rl.SetShaderValue(shd, locLightDir, ld[:], rl.ShaderUniformVec3)
	rl.SetShaderValue(shd, locLightColor, lc[:], rl.ShaderUniformVec3)
	rl.SetShaderValue(shd, locAmbient, []float32{ui.ambient}, rl.ShaderUniformFloat)
	rl.SetShaderValue(shd, locViewPos, vp[:], rl.ShaderUniformVec3)
}

// ── Model ─────────────────────────────────────────────────────
func loadModel(path string) bool {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))

	switch ext {
	case "obj", "fbx", "gltf", "glb", "iqm", "m3d":
		// supported
	default:
		setStatus("Unsupported format — use OBJ, FBX, GLTF, IQM, M3D")
		return false
	}

	if obj.loaded {
		rl.UnloadModel(obj.model)
		obj.loaded = false
	}

	mdl := rl.LoadModel(path)
	if mdl.MeshCount == 0 {
		setStatus("Failed to load: %s", filepath.Base(path))
		return false
	}

	obj.model = mdl
	obj.loaded = true
	obj.ext = ext

	base := filepath.Base(path)
	if dot := strings.LastIndex(base, "."); dot >= 0 {
		obj.name = base[:dot]
	} else {
		obj.name = base
	}

	// Count verts / tris via unsafe pointer arithmetic
	obj.meshCount = int(mdl.MeshCount)
	obj.vertexCount = 0
	obj.triCount = 0
	for i := 0; i < obj.meshCount; i++ {
		m := modelMesh(mdl, i)
		obj.vertexCount += int(m.VertexCount)
		obj.triCount += int(m.TriangleCount)
	}

	// Bounds and scale
	obj.bounds = rl.GetModelBoundingBox(mdl)
	obj.center = rl.Vector3Scale(rl.Vector3Add(obj.bounds.Min, obj.bounds.Max), 0.5)
	diag := rl.Vector3Length(rl.Vector3Subtract(obj.bounds.Max, obj.bounds.Min))
	if diag > 0.001 {
		obj.scaleFactor = 2.0 / diag
	} else {
		obj.scaleFactor = 1.0
	}

	// Assign shader
	for i := int32(0); i < mdl.MaterialCount; i++ {
		mat := modelMaterial(mdl, int(i))
		mat.Shader = shd
	}

	camFocus()
	setStatus("Loaded %s  (%d meshes, %d verts, %d tris)",
		filepath.Base(path), obj.meshCount, obj.vertexCount, obj.triCount)
	return true
}

// Unsafe helpers to index C-backed arrays in rl.Model ─────────
func modelMesh(m rl.Model, i int) *rl.Mesh {
	return (*rl.Mesh)(unsafe.Pointer(
		uintptr(unsafe.Pointer(m.Meshes)) + uintptr(i)*unsafe.Sizeof(rl.Mesh{})))
}

func modelMaterial(m rl.Model, i int) *rl.Material {
	return (*rl.Material)(unsafe.Pointer(
		uintptr(unsafe.Pointer(m.Materials)) + uintptr(i)*unsafe.Sizeof(rl.Material{})))
}

// ── Grid ──────────────────────────────────────────────────────
func drawGrid(halfSteps int, step float32) {
	span := float32(halfSteps) * step
	for i := -halfSteps; i <= halfSteps; i++ {
		t := float32(i) * step
		if i == 0 {
			rl.DrawLine3D(rl.Vector3{X: -span}, rl.Vector3{X: span}, cAxisX)
			rl.DrawLine3D(rl.Vector3{Z: -span}, rl.Vector3{Z: span}, cAxisZ)
			continue
		}
		c := cGridMin
		if i%5 == 0 {
			c = cGridMaj
		}
		rl.DrawLine3D(rl.Vector3{X: -span, Z: t}, rl.Vector3{X: span, Z: t}, c)
		rl.DrawLine3D(rl.Vector3{X: t, Z: -span}, rl.Vector3{X: t, Z: span}, c)
	}
}

// ── Navigation Gizmo ─────────────────────────────────────────
func drawGizmo(sw, sh int32) {
	c := camGet()
	fwd := rl.Vector3Normalize(rl.Vector3Subtract(c.Target, c.Position))
	right := rl.Vector3Normalize(rl.Vector3CrossProduct(fwd, rl.Vector3{X: 0, Y: 1, Z: 0}))
	up := rl.Vector3CrossProduct(right, fwd)

	pw := int32(0)
	if ui.rightPanel {
		pw = panelRW
	}
	cx := float32(sw-pw) - gizmoSize - gizmoMargin
	cy := float32(topbarH) + gizmoSize + gizmoMargin

	rl.DrawCircle(int32(cx), int32(cy), gizmoSize, rl.Color{R: 14, G: 14, B: 22, A: 200})
	rl.DrawCircleLines(int32(cx), int32(cy), gizmoSize, cBorder)

	type axis struct {
		dir   rl.Vector3
		col   rl.Color
		label string
	}
	axes := []axis{
		{rl.Vector3{X: 1}, rl.Color{R: 220, G: 60, B: 60, A: 255}, "X"},
		{rl.Vector3{X: -1}, rl.Color{R: 100, G: 30, B: 30, A: 255}, "-X"},
		{rl.Vector3{Y: 1}, rl.Color{R: 60, G: 200, B: 60, A: 255}, "Y"},
		{rl.Vector3{Y: -1}, rl.Color{R: 30, G: 80, B: 30, A: 255}, "-Y"},
		{rl.Vector3{Z: 1}, rl.Color{R: 60, G: 60, B: 220, A: 255}, "Z"},
		{rl.Vector3{Z: -1}, rl.Color{R: 30, G: 30, B: 80, A: 255}, "-Z"},
	}

	// depth sort back-to-front
	depths := make([]float32, 6)
	order := []int{0, 1, 2, 3, 4, 5}
	for i, a := range axes {
		depths[i] = rl.Vector3DotProduct(a.dir, fwd)
	}
	for i := 1; i < 6; i++ {
		k := order[i]
		d := depths[k]
		j := i - 1
		for j >= 0 && depths[order[j]] > d {
			order[j+1] = order[j]
			j--
		}
		order[j+1] = k
	}

	arm := gizmoSize * 0.78
	for _, idx := range order {
		a := axes[idx]
		px := rl.Vector3DotProduct(a.dir, right)
		py := -rl.Vector3DotProduct(a.dir, up)
		ex := int32(cx + px*arm)
		ey := int32(cy + py*arm)
		if idx%2 == 0 { // positive half
			rl.DrawLine(int32(cx), int32(cy), ex, ey, a.col)
			rl.DrawCircle(ex, ey, 6.5, a.col)
			tw := rl.MeasureText(a.label, fontSizeSm)
			rl.DrawText(a.label, ex-tw/2, ey-fontSizeSm/2, fontSizeSm, rl.White)
		} else {
			rl.DrawCircle(ex, ey, 4.0, a.col)
		}
	}
}

// ── Scene ─────────────────────────────────────────────────────
func drawScene() {
	if ui.grid {
		drawGrid(20, 0.5)
	}
	if ui.axes {
		rl.DrawLine3D(rl.Vector3{X: -5}, rl.Vector3{X: 5}, cAxisX)
		rl.DrawLine3D(rl.Vector3{Y: -5}, rl.Vector3{Y: 5}, cAxisY)
		rl.DrawLine3D(rl.Vector3{Z: -5}, rl.Vector3{Z: 5}, cAxisZ)
	}
	if !obj.loaded {
		return
	}

	sf := obj.scaleFactor
	pos := rl.Vector3Scale(obj.center, -sf)
	axis := rl.Vector3{Y: 1}
	scale := rl.Vector3{X: sf, Y: sf, Z: sf}

	switch ui.shading {
	case ShadeSolid, ShadeMaterial:
		rl.DrawModelEx(obj.model, pos, axis, 0, scale, rl.White)
	case ShadeWire:
		rl.DrawModelWiresEx(obj.model, pos, axis, 0, scale, cAccent)
	}

	if ui.wireOver && ui.shading != ShadeWire {
		rl.DrawModelWiresEx(obj.model, pos, axis, 0, scale,
			rl.Color{R: 0, G: 180, B: 230, A: 40})
	}
}

// ── UI helpers ────────────────────────────────────────────────
func btn(x, y, w, h int32, label string, active bool, accent rl.Color) bool {
	mp := rl.GetMousePosition()
	r := rl.Rectangle{X: float32(x), Y: float32(y), Width: float32(w), Height: float32(h)}
	hov := rl.CheckCollisionPointRec(mp, r)

	fill := cPanel
	if active {
		fill = rl.Color{R: accent.R, G: accent.G, B: accent.B, A: 40}
	} else if hov {
		fill = cHover
	}
	rl.DrawRectangle(x, y, w, h, fill)
	if active {
		rl.DrawRectangle(x, y+h-2, w, 2, accent)
	} else if hov {
		rl.DrawRectangle(x, y+h-1, w, 1, cBorder)
	}
	tc := cText
	if active {
		tc = accent
	} else if hov {
		tc = cTextBrt
	}
	tw := rl.MeasureText(label, fontSizeSm)
	rl.DrawText(label, x+(w-tw)/2, y+(h-fontSizeSm)/2, fontSizeSm, tc)
	return hov && rl.IsMouseButtonPressed(rl.MouseButtonLeft)
}

func sectionHeader(x, y, w int32, title string, open *bool) {
	h := btnH
	mp := rl.GetMousePosition()
	hov := rl.CheckCollisionPointRec(mp, rl.Rectangle{X: float32(x), Y: float32(y), Width: float32(w), Height: float32(h)})
	if hov {
		rl.DrawRectangle(x, y, w, h, cHover)
	} else {
		rl.DrawRectangle(x, y, w, h, cPanel)
	}
	rl.DrawRectangle(x, y, 3, h, cAccent)
	rl.DrawText(title, x+12, y+(h-fontSize)/2, fontSize, cTextBrt)
	arrow := ">"
	if *open {
		arrow = "v"
	}
	tw := rl.MeasureText(arrow, fontSize)
	rl.DrawText(arrow, x+w-tw-8, y+(h-fontSize)/2, fontSize, cAccent)
	rl.DrawLine(x, y+h, x+w, y+h, cBorderLo)
	if hov && rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
		*open = !*open
	}
}

func labelRow(x, y int32, key, val string, valColor rl.Color) {
	rl.DrawText(key, x+pad, y, fontSizeSm, cTextDim)
	rl.DrawText(val, x+pad+90, y, fontSizeSm, valColor)
}

func slider(x, y, w int32, label string, val *float32, lo, hi float32) {
	rl.DrawText(label, x+pad, y, fontSizeSm, cTextDim)
	y += fontSizeSm + 3
	slH := int32(6)
	rl.DrawRectangle(x, y, w, slH, cBorder)
	t := (*val - lo) / (hi - lo)
	rl.DrawRectangle(x, y, int32(t*float32(w)), slH, cAccentDim)
	hx := x + int32(t*float32(w-10))
	rl.DrawRectangle(hx, y-3, 10, slH+6, cAccent)

	mp := rl.GetMousePosition()
	hr := rl.Rectangle{X: float32(x), Y: float32(y - 3), Width: float32(w), Height: float32(slH + 6)}
	if rl.CheckCollisionPointRec(mp, hr) && rl.IsMouseButtonDown(rl.MouseButtonLeft) {
		*val = lo + clamp32((mp.X-float32(x))/float32(w), 0, 1)*(hi-lo)
	}
}

// ── Top bar ───────────────────────────────────────────────────
func drawTopBar(sw int32) {
	rl.DrawRectangle(0, 0, sw, topbarH, cPanel)
	rl.DrawLine(0, topbarH-1, sw, topbarH-1, cBorder)
	rl.DrawRectangle(0, 0, sw, 2, cAccent)

	rl.DrawText("VOID", pad, (topbarH-fontSizeLg)/2, fontSizeLg, cAccent)
	rl.DrawText("VIEWER", pad+36, (topbarH-fontSizeSm)/2+2, fontSizeSm, cTextDim)

	x := int32(110)

	if btn(x, 4, 68, topbarH-8, "OPEN", false, cAccent) {
		if p := openFileDialog(); p != "" {
			loadModel(p)
		}
	}
	x += 72

	rl.DrawLine(x, 8, x, topbarH-8, cBorder)
	x += 8

	for i, lbl := range []string{"SOLID", "WIRE", "MAT"} {
		if btn(x, 4, 48, topbarH-8, lbl, ui.shading == ShadingMode(i), cAccent) {
			ui.shading = ShadingMode(i)
		}
		x += 50
	}

	rl.DrawLine(x, 8, x, topbarH-8, cBorder)
	x += 8

	if btn(x, 4, 40, topbarH-8, "GRID", ui.grid, cPurple) {
		ui.grid = !ui.grid
	}
	x += 42
	if btn(x, 4, 40, topbarH-8, "AXES", ui.axes, cPurple) {
		ui.axes = !ui.axes
	}
	x += 42
	if btn(x, 4, 40, topbarH-8, "+WIRE", ui.wireOver, cPurple) {
		ui.wireOver = !ui.wireOver
	}
	x += 46

	rl.DrawLine(x, 8, x, topbarH-8, cBorder)
	x += 8

	viewLabels := []string{"PERSP", "FRONT", "RIGHT", "TOP"}
	viewVals := []PresetView{ViewPersp, ViewFront, ViewRight, ViewTop}
	for i, lbl := range viewLabels {
		if btn(x, 4, 48, topbarH-8, lbl, ui.view == viewVals[i], cAccentDim) {
			camPreset(viewVals[i])
		}
		x += 50
	}

	// Panel toggle (right side)
	if btn(sw-40, 4, 34, topbarH-8, ">>", ui.rightPanel, cAccent) {
		ui.rightPanel = !ui.rightPanel
	}

	// Center: filename
	if obj.loaded {
		info := obj.name + "." + obj.ext
		tw := rl.MeasureText(info, fontSizeSm)
		rl.DrawText(info, sw/2-tw/2, (topbarH-fontSizeSm)/2, fontSizeSm, cTextDim)
	}
}

// ── Right panel ───────────────────────────────────────────────
func drawRightPanel(sw, sh int32) {
	pr := panelRect(sw, sh)
	px, py := int32(pr.X), int32(pr.Y)
	pw, ph := int32(pr.Width), int32(pr.Height)

	rl.DrawRectangle(px, py, pw, ph, cPanel)
	rl.DrawLine(px, py, px, py+ph, cBorder)

	y := py + 6

	// Object section
	sectionHeader(px, y, pw, "OBJECT", &ui.secObject)
	y += btnH + 2
	if ui.secObject {
		if obj.loaded {
			labelRow(px, y, "File", obj.name, cText); y += fontSizeSm + 5
			labelRow(px, y, "Format", obj.ext, cText); y += fontSizeSm + 5
			labelRow(px, y, "Meshes", fmt.Sprintf("%d", obj.meshCount), cAccent); y += fontSizeSm + 5
			labelRow(px, y, "Vertices", fmt.Sprintf("%d", obj.vertexCount), cAccent); y += fontSizeSm + 5
			labelRow(px, y, "Triangles", fmt.Sprintf("%d", obj.triCount), cAccent); y += fontSizeSm + 5
		} else {
			rl.DrawText("No model loaded", px+pad, y, fontSizeSm, cTextDim)
			y += fontSizeSm + 5
		}
		y += 4
	}
	rl.DrawLine(px, y, px+pw, y, cBorderLo); y += 4

	// Transform section
	sectionHeader(px, y, pw, "TRANSFORM", &ui.secTransform)
	y += btnH + 2
	if ui.secTransform {
		labelRow(px, y, "Azimuth", fmt.Sprintf("%.1f°", cam.azimuth*180/math.Pi), cAccent); y += fontSizeSm + 5
		labelRow(px, y, "Elevation", fmt.Sprintf("%.1f°", cam.elevation*180/math.Pi), cAccent); y += fontSizeSm + 5
		labelRow(px, y, "Distance", fmt.Sprintf("%.3f", cam.distance), cAccent); y += fontSizeSm + 5
		mode := "PERSP"
		if cam.ortho {
			mode = "ORTHO"
		}
		labelRow(px, y, "Projection", mode, cText); y += fontSizeSm + 5
		if obj.loaded {
			if btn(px+pad, y, pw-pad*2, btnH, "FOCUS OBJECT (.)", false, cAccent) {
				camFocus()
			}
			y += btnH + 6
		}
		y += 4
	}
	rl.DrawLine(px, y, px+pw, y, cBorderLo); y += 4

	// Display section
	sectionHeader(px, y, pw, "DISPLAY", &ui.secDisplay)
	y += btnH + 2
	if ui.secDisplay {
		sname := [...]string{"Solid", "Wireframe", "Material"}[ui.shading]
		labelRow(px, y, "Shading", sname, cText); y += fontSizeSm + 5

		rl.DrawText("Overlays", px+pad, y, fontSizeSm, cTextDim); y += fontSizeSm + 4
		bw := (pw - pad*3) / 2
		bx := px + pad
		if btn(bx, y, bw, btnH, "Grid", ui.grid, cPurple) {
			ui.grid = !ui.grid
		}
		if btn(bx+bw+pad, y, bw, btnH, "Axes", ui.axes, cPurple) {
			ui.axes = !ui.axes
		}
		y += btnH + 4
		if btn(bx, y, bw, btnH, "+Wire", ui.wireOver, cPurple) {
			ui.wireOver = !ui.wireOver
		}
		if btn(bx+bw+pad, y, bw, btnH, "Stats", ui.stats, cPurple) {
			ui.stats = !ui.stats
		}
		y += btnH + 8
	}
	rl.DrawLine(px, y, px+pw, y, cBorderLo); y += 4

	// Lighting section
	sectionHeader(px, y, pw, "LIGHTING", &ui.secLighting)
	y += btnH + 2
	if ui.secLighting {
		slw := pw - pad*2

		slider(px+pad, y, slw, "Light Azimuth", &ui.lightAz, -math.Pi, math.Pi); y += fontSizeSm + 3 + 6 + 8
		slider(px+pad, y, slw, "Elevation", &ui.lightEl, -math.Pi/2, math.Pi/2); y += fontSizeSm + 3 + 6 + 8

		rl.DrawText(fmt.Sprintf("Ambient  %.2f", ui.ambient), px+pad, y, fontSizeSm, cTextDim); y += fontSizeSm + 3
		slider(px+pad, y, slw, "", &ui.ambient, 0, 1); y += fontSizeSm + 3 + 6 + 8
	}

	// Shortcuts pinned to bottom
	bot := py + ph - 126
	rl.DrawLine(px, bot, px+pw, bot, cBorderLo); bot += 6
	rl.DrawText("SHORTCUTS", px+pad, bot, fontSizeSm, cTextDim); bot += fontSizeSm + 4
	rl.DrawText("O  Open  |  .  Focus", px+pad, bot, fontSizeSm, cTextDim); bot += fontSizeSm + 3
	rl.DrawText("G Grid  Z Shade  W Wire  A Axes", px+pad, bot, fontSizeSm, cTextDim); bot += fontSizeSm + 3
	rl.DrawText("Numpad 1/3/7  Views  |  5  Ortho", px+pad, bot, fontSizeSm, cTextDim); bot += fontSizeSm + 3
	rl.DrawText("MMB Orbit  |  Shift+MMB  Pan", px+pad, bot, fontSizeSm, cTextDim)
}

// ── Bottom bar ────────────────────────────────────────────────
func drawBottomBar(sw, sh int32) {
	y := sh - bottombarH
	rl.DrawRectangle(0, y, sw, bottombarH, cPanel)
	rl.DrawLine(0, y, sw, y, cBorder)
	rl.DrawRectangle(0, sh-1, sw, 1, cPurpleDim)

	if ui.statusTimer > 0 {
		tc := cTextDim
		if ui.statusTimer < 1.0 {
			tc.A = uint8(ui.statusTimer * 255)
		}
		rl.DrawText(ui.statusMsg, pad, y+(bottombarH-fontSizeSm)/2, fontSizeSm, tc)
	} else {
		hint := "No model loaded — press O to open"
		if obj.loaded {
			hint = "Drag & drop OBJ / FBX / GLTF to reload"
		}
		rl.DrawText(hint, pad, y+(bottombarH-fontSizeSm)/2, fontSizeSm, cTextDim)
	}

	fps := fmt.Sprintf("%d FPS", rl.GetFPS())
	tw := rl.MeasureText(fps, fontSizeSm)
	rl.DrawText(fps, sw-tw-pad, y+(bottombarH-fontSizeSm)/2, fontSizeSm, cTextDim)

	if obj.loaded {
		s := fmt.Sprintf("%dv  %dt", obj.vertexCount, obj.triCount)
		tw2 := rl.MeasureText(s, fontSizeSm)
		rl.DrawText(s, sw-tw2-tw-pad*4, y+(bottombarH-fontSizeSm)/2, fontSizeSm, cTextDim)
	}
}

// ── Stats overlay ─────────────────────────────────────────────
func drawStats(vp rl.Rectangle) {
	if !ui.stats {
		return
	}
	names := [...]string{"PERSP", "FRONT", "BACK", "RIGHT", "LEFT", "TOP", "BOTTOM"}
	s := fmt.Sprintf("%s  |  dist %.2f  |  az %.0f°  el %.0f°",
		names[ui.view], cam.distance,
		cam.azimuth*180/math.Pi, cam.elevation*180/math.Pi)
	tw := rl.MeasureText(s, fontSizeSm)
	bx, by := int32(vp.X)+pad-3, int32(vp.Y)+pad-2
	rl.DrawRectangle(bx, by, tw+6, fontSizeSm+4, rl.Color{R: 8, G: 8, B: 14, A: 190})
	rl.DrawText(s, bx+3, by+2, fontSizeSm, cTextDim)
}

// ── No-model hint ─────────────────────────────────────────────
func drawNoModelHint(sw, sh int32) {
	if obj.loaded {
		return
	}
	vp := vpRect(sw, sh)
	hint := "DROP A FILE   OR   PRESS  O"
	tw := rl.MeasureText(hint, fontSizeLg)
	rl.DrawText(hint,
		int32(vp.X)+(int32(vp.Width)-tw)/2,
		int32(vp.Y)+int32(vp.Height)/2-fontSizeLg/2,
		fontSizeLg, rl.Color{R: 45, G: 45, B: 65, A: 200})
}

// ── Input ─────────────────────────────────────────────────────
func handleInput() {
	ctrl := rl.IsKeyDown(rl.KeyLeftControl) || rl.IsKeyDown(rl.KeyRightControl)

	if rl.IsKeyPressed(rl.KeyO) {
		if p := openFileDialog(); p != "" {
			loadModel(p)
		}
	}
	if rl.IsKeyPressed(rl.KeyPeriod) {
		camFocus()
	}
	if rl.IsKeyPressed(rl.KeyZ) {
		ui.shading = (ui.shading + 1) % ShadeCount
	}
	if rl.IsKeyPressed(rl.KeyG) {
		ui.grid = !ui.grid
	}
	if rl.IsKeyPressed(rl.KeyW) {
		ui.wireOver = !ui.wireOver
	}
	if rl.IsKeyPressed(rl.KeyA) {
		ui.axes = !ui.axes
	}
	if rl.IsKeyPressed(rl.KeyKp1) {
		if ctrl {
			camPreset(ViewBack)
		} else {
			camPreset(ViewFront)
		}
	}
	if rl.IsKeyPressed(rl.KeyKp3) {
		if ctrl {
			camPreset(ViewLeft)
		} else {
			camPreset(ViewRight)
		}
	}
	if rl.IsKeyPressed(rl.KeyKp7) {
		if ctrl {
			camPreset(ViewBottom)
		} else {
			camPreset(ViewTop)
		}
	}
	if rl.IsKeyPressed(rl.KeyKp0) {
		camPreset(ViewPersp)
	}
	if rl.IsKeyPressed(rl.KeyKp5) {
		cam.ortho = !cam.ortho
	}
	if rl.IsKeyPressed(rl.KeyF11) {
		rl.ToggleFullscreen()
	}

	// Drag & drop
	if rl.IsFileDropped() {
		files := rl.LoadDroppedFiles()
		if len(files) > 0 {
			loadModel(files[0])
		}
		rl.UnloadDroppedFiles()
	}

	if ui.statusTimer > 0 {
		ui.statusTimer -= rl.GetFrameTime()
		if ui.statusTimer < 0 {
			ui.statusTimer = 0
		}
	}
}

// ── Main ──────────────────────────────────────────────────────
func main() {
	rl.SetConfigFlags(rl.FlagWindowResizable | rl.FlagMsaa4xHint | rl.FlagVsyncHint)
	rl.InitWindow(1400, 860, "VOID VIEWER")
	rl.SetTargetFPS(144)
	rl.SetExitKey(0)

	camInit()
	shaderInit()

	ui = UIState{
		shading:      ShadeSolid,
		view:         ViewPersp,
		grid:         true,
		axes:         true,
		stats:        true,
		rightPanel:   true,
		lightAz:      -0.8,
		lightEl:      0.9,
		ambient:      0.18,
		secObject:    true,
		secTransform: true,
		secDisplay:   true,
		secLighting:  true,
	}
	setStatus("Welcome — drag & drop a model or press O to open")

	for !rl.WindowShouldClose() {
		sw := int32(rl.GetScreenWidth())
		sh := int32(rl.GetScreenHeight())
		vp := vpRect(sw, sh)
		c := camGet()

		handleInput()
		camUpdate(vp)
		shaderUpdate(c)

		rl.BeginDrawing()
		rl.ClearBackground(cBG)

		rl.BeginMode3D(c)
		drawScene()
		rl.EndMode3D()

		drawNoModelHint(sw, sh)
		drawStats(vp)
		drawGizmo(sw, sh)

		drawTopBar(sw)
		if ui.rightPanel {
			drawRightPanel(sw, sh)
		}
		drawBottomBar(sw, sh)

		rl.EndDrawing()
	}

	if obj.loaded {
		rl.UnloadModel(obj.model)
	}
	rl.UnloadShader(shd)
	rl.CloseWindow()
}
