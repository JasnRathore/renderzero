// ============================================================
//  RENDER ZERO — 3D Model Viewer
//  Go + raylib-go | Windows only
//  UI: Void Precision — Zhenya Rynzhuk × Jony Ive
// ============================================================
//
//  KEYBOARD CONTROLS
//  ─────────────────
//  Middle Mouse Drag        Orbit
//  Shift + MMB Drag         Pan
//  Scroll Wheel             Zoom
//  O                        Open file dialog
//  Drag & Drop              Load OBJ / FBX / GLTF
//  Numpad 1/3/7             Front / Right / Top view
//  Ctrl+Numpad 1/3/7        Back / Left / Bottom
//  Numpad 0                 Perspective reset
//  Numpad 5                 Toggle ortho / persp
//  . (Period)               Focus on model
//  G / W / A                Grid / Wire overlays / Axes
//  Z                        Cycle shading mode
//  F11                      Fullscreen
//
//  ON-SCREEN NAV TOOLBAR  (floating, bottom of viewport)
//  ───────────────────────────────────────────────────────
//  Orbit   left-click drag = orbit camera
//  Pan     left-click drag = pan camera
//  +  −    zoom in / out
//  Focus   frame the loaded model
//  Ortho   toggle perspective ↔ orthographic
//  Views   Front / Right / Top presets
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
	topbarH     = int32(42)
	bottombarH  = int32(26)
	panelRW     = int32(276)
	fontSize    = int32(16)
	fontSizeSm  = int32(14)
	fontSizeLg  = int32(20)
	fontSizeXL  = int32(24)
	pad         = int32(10)
	btnH        = int32(28)
	gizmoSize   = float32(68)
	gizmoMargin = float32(18)
	navBtnW     = int32(62)
	navBtnH     = int32(32)
)

// ── Palette — Render Zero / Blender DNA ───────────────────────
// Warm charcoal neutrals, Blender orange accent, steel teal secondary.
// Zero blue-tinting, zero purple.
var (
	cBG        = rl.Color{R: 28, G: 28, B: 28, A: 255}   // viewport bg
	cPanel     = rl.Color{R: 36, G: 36, B: 36, A: 255}   // panel / header bg
	cBorder    = rl.Color{R: 18, G: 18, B: 18, A: 255}   // hard separator
	cBorderLo  = rl.Color{R: 26, G: 26, B: 26, A: 255}   // soft separator
	cAccent    = rl.Color{R: 232, G: 125, B: 13, A: 255} // Blender orange
	cAccentDim = rl.Color{R: 160, G: 84, B: 8, A: 255}   // orange dim
	cPurple    = rl.Color{R: 78, G: 142, B: 166, A: 255} // steel teal (secondary)
	cPurpleDim = rl.Color{R: 42, G: 88, B: 106, A: 255}  // steel teal dim
	cText      = rl.Color{R: 210, G: 210, B: 210, A: 255} // primary text
	cTextDim   = rl.Color{R: 120, G: 120, B: 120, A: 255} // muted text
	cTextBrt   = rl.Color{R: 240, G: 240, B: 240, A: 255} // bright text
	cHover     = rl.Color{R: 50, G: 50, B: 50, A: 255}   // hover fill
	cGridMin   = rl.Color{R: 38, G: 38, B: 38, A: 255}   // minor grid line
	cGridMaj   = rl.Color{R: 52, G: 52, B: 52, A: 255}   // major grid line
	cAxisX     = rl.Color{R: 186, G: 60, B: 60, A: 200}
	cAxisY     = rl.Color{R: 60, G: 186, B: 60, A: 200}
	cAxisZ     = rl.Color{R: 60, G: 100, B: 186, A: 200}
	cNavBg     = rl.Color{R: 30, G: 30, B: 30, A: 236}
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

type NavMode int

const (
	NavDefault NavMode = iota // MMB-only
	NavOrbit                  // LMB drag → orbit
	NavPan                    // LMB drag → pan
)

// ── State structs ─────────────────────────────────────────────
type OrbitCam struct {
	target    rl.Vector3
	azimuth   float32
	elevation float32
	distance  float32
	fovy      float32
	ortho     bool
	dragStart rl.Vector2
	panOrigin rl.Vector3
	dragging  bool
	panning   bool
}

type ModelState struct {
	model       rl.Model
	loaded      bool
	name        string
	ext         string
	bounds      rl.BoundingBox
	center      rl.Vector3
	scaleFactor float32
	meshCount   int
	vertexCount int
	triCount    int
}

type UIState struct {
	shading      ShadingMode
	view         PresetView
	navMode      NavMode
	grid         bool
	axes         bool
	wireOver     bool
	stats        bool
	rightPanel   bool
	lightAz      float32
	lightEl      float32
	ambient      float32
	secObject    bool
	secTransform bool
	secDisplay   bool
	secLighting  bool
	statusMsg    string
	statusTimer  float32
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

// ── Font helpers ──────────────────────────────────────────────
// We load one font atlas per render size so DrawTextEx always draws 1:1 —
// no scaling, no bilinear blur.
var (
	gFont       rl.Font
	gFontLoaded bool
	gFontSm     rl.Font // fontSizeSm
	gFontMd     rl.Font // fontSize
	gFontLg     rl.Font // fontSizeLg
	gFontXL     rl.Font // fontSizeXL
)

func loadFontAt(path string, size int32) (rl.Font, bool) {
	f := rl.LoadFontEx(path, size, nil, 0)
	if f.Texture.ID != 0 {
		rl.SetTextureFilter(f.Texture, rl.FilterPoint)
		return f, true
	}
	return f, false
}

func initFont() {
	candidates := []string{
		"C:/Windows/Fonts/seguivar.ttf",
		"C:/Windows/Fonts/segoeui.ttf",
		"C:/Windows/Fonts/calibri.ttf",
		"C:/Windows/Fonts/tahoma.ttf",
	}
	for _, p := range candidates {
		f, ok := loadFontAt(p, fontSizeSm)
		if !ok {
			continue
		}
		gFontSm = f
		gFontMd, _ = loadFontAt(p, fontSize)
		gFontLg, _ = loadFontAt(p, fontSizeLg)
		gFontXL, _ = loadFontAt(p, fontSizeXL)
		gFont = gFontMd
		gFontLoaded = true
		return
	}
}

// fontFor returns the pre-baked atlas that exactly matches sz.
func fontFor(sz int32) rl.Font {
	switch sz {
	case fontSizeSm:
		return gFontSm
	case fontSizeLg:
		return gFontLg
	case fontSizeXL:
		return gFontXL
	default:
		return gFontMd
	}
}

// txt draws text at pixel-exact size — no scaling artefacts.
func txt(s string, x, y, sz int32, c rl.Color) {
	if gFontLoaded {
		f := fontFor(sz)
		rl.DrawTextEx(f, s, rl.Vector2{X: float32(x), Y: float32(y)},
			float32(sz), 1.0, c)
	} else {
		rl.DrawText(s, x, y, sz, c)
	}
}

// txtC draws text horizontally centered at cx.
func txtC(s string, cx, y, sz int32, c rl.Color) {
	w := measure(s, sz)
	txt(s, cx-w/2, y, sz, c)
}

// measure returns the pixel width of a string at the given size.
func measure(s string, sz int32) int32 {
	if gFontLoaded {
		f := fontFor(sz)
		v := rl.MeasureTextEx(f, s, float32(sz), 1.0)
		return int32(v.X)
	}
	return rl.MeasureText(s, sz)
}

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
	return rl.Rectangle{
		X: 0, Y: float32(topbarH),
		Width: float32(sw - pw), Height: float32(sh - topbarH - bottombarH),
	}
}

func panelRect(sw, sh int32) rl.Rectangle {
	return rl.Rectangle{
		X: float32(sw - panelRW), Y: float32(topbarH),
		Width: float32(panelRW), Height: float32(sh - topbarH - bottombarH),
	}
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
	lmb := rl.IsMouseButtonDown(rl.MouseButtonLeft)
	shift := rl.IsKeyDown(rl.KeyLeftShift) || rl.IsKeyDown(rl.KeyRightShift)
	inVP := rl.CheckCollisionPointRec(mouse, vp)

	// Scroll zoom
	if inVP && scroll != 0 {
		cam.distance -= scroll * cam.distance * 0.08
		if cam.distance < 0.05 {
			cam.distance = 0.05
		}
	}

	// Determine active orbit/pan from MMB or on-screen nav mode
	activeOrbit := (mmb && !shift && inVP) ||
		(lmb && ui.navMode == NavOrbit && inVP)
	activePan := (mmb && shift && inVP) ||
		(lmb && ui.navMode == NavPan && inVP)

	if activeOrbit || activePan {
		if !cam.dragging && !cam.panning {
			cam.dragStart = mouse
			cam.panOrigin = cam.target
			cam.dragging = activeOrbit
			cam.panning = activePan
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

	setShaderLoc(&shd, rl.ShaderLocMatrixModel, locMatModel)
	setShaderLoc(&shd, rl.ShaderLocVectorView, locViewPos)
}

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

	obj.meshCount = int(mdl.MeshCount)
	obj.vertexCount = 0
	obj.triCount = 0
	for i := 0; i < obj.meshCount; i++ {
		m := modelMesh(mdl, i)
		obj.vertexCount += int(m.VertexCount)
		obj.triCount += int(m.TriangleCount)
	}

	obj.bounds = rl.GetModelBoundingBox(mdl)
	obj.center = rl.Vector3Scale(rl.Vector3Add(obj.bounds.Min, obj.bounds.Max), 0.5)
	diag := rl.Vector3Length(rl.Vector3Subtract(obj.bounds.Max, obj.bounds.Min))
	if diag > 0.001 {
		obj.scaleFactor = 2.0 / diag
	} else {
		obj.scaleFactor = 1.0
	}

	for i := int32(0); i < mdl.MaterialCount; i++ {
		mat := modelMaterial(mdl, int(i))
		mat.Shader = shd
	}

	camFocus()
	setStatus("Loaded %s  (%d meshes, %d verts, %d tris)",
		filepath.Base(path), obj.meshCount, obj.vertexCount, obj.triCount)
	return true
}

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

	// Soft glass background
	rl.DrawCircle(int32(cx), int32(cy), gizmoSize,
		rl.Color{R: 28, G: 28, B: 28, A: 180})
	rl.DrawCircleLines(int32(cx), int32(cy), gizmoSize,
		rl.Color{R: 50, G: 50, B: 50, A: 200})
	rl.DrawCircleLines(int32(cx), int32(cy), gizmoSize-1,
		rl.Color{R: 22, G: 22, B: 22, A: 100})

	type axis struct {
		dir   rl.Vector3
		col   rl.Color
		label string
	}
	axes := []axis{
		{rl.Vector3{X: 1}, rl.Color{R: 218, G: 58, B: 58, A: 255}, "X"},
		{rl.Vector3{X: -1}, rl.Color{R: 90, G: 26, B: 26, A: 255}, "-X"},
		{rl.Vector3{Y: 1}, rl.Color{R: 58, G: 198, B: 58, A: 255}, "Y"},
		{rl.Vector3{Y: -1}, rl.Color{R: 26, G: 80, B: 26, A: 255}, "-Y"},
		{rl.Vector3{Z: 1}, rl.Color{R: 58, G: 58, B: 218, A: 255}, "Z"},
		{rl.Vector3{Z: -1}, rl.Color{R: 26, G: 26, B: 80, A: 255}, "-Z"},
	}

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

	arm := gizmoSize * 0.76
	for _, idx := range order {
		a := axes[idx]
		px2 := rl.Vector3DotProduct(a.dir, right)
		py2 := -rl.Vector3DotProduct(a.dir, up)
		ex := int32(cx + px2*arm)
		ey := int32(cy + py2*arm)
		if idx%2 == 0 {
			rl.DrawLine(int32(cx), int32(cy), ex, ey,
				rl.Color{R: a.col.R, G: a.col.G, B: a.col.B, A: 140})
			rl.DrawCircle(ex, ey, 6.0, a.col)
			// Inner highlight dot
			rl.DrawCircle(ex-1, ey-1, 1.5, rl.Color{R: 255, G: 255, B: 255, A: 60})
			tw := measure(a.label, fontSizeSm)
			txt(a.label, ex-tw/2, ey-fontSizeSm/2, fontSizeSm, rl.White)
		} else {
			rl.DrawCircle(ex, ey, 3.5, a.col)
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
		rl.DrawModelWiresEx(obj.model, pos, axis, 0, scale,
			rl.Color{R: 78, G: 142, B: 166, A: 255})
	}

	if ui.wireOver && ui.shading != ShadeWire {
		rl.DrawModelWiresEx(obj.model, pos, axis, 0, scale,
			rl.Color{R: 200, G: 130, B: 30, A: 36})
	}
}

// ── UI primitives ─────────────────────────────────────────────

// roundRect draws a filled rounded rectangle.
func roundRect(x, y, w, h int32, r float32, c rl.Color) {
	rl.DrawRectangleRounded(
		rl.Rectangle{X: float32(x), Y: float32(y), Width: float32(w), Height: float32(h)},
		r, 6, c)
}

// roundRectLines draws a rounded rectangle outline.
func roundRectLines(x, y, w, h int32, r float32, c rl.Color) {
	rl.DrawRectangleRoundedLines(
		rl.Rectangle{X: float32(x), Y: float32(y), Width: float32(w), Height: float32(h)},
		r, 6, c)
}

// btn renders a styled button and returns true if clicked.
func btn(x, y, w, h int32, label string, active bool, accent rl.Color) bool {
	mp := rl.GetMousePosition()
	r := rl.Rectangle{X: float32(x), Y: float32(y), Width: float32(w), Height: float32(h)}
	hov := rl.CheckCollisionPointRec(mp, r)

	// Fill
	fill := cPanel
	if active {
		fill = rl.Color{R: accent.R / 10, G: accent.G / 10, B: accent.B / 10, A: 255}
		// Stronger tint
		fill = rl.Color{
			R: uint8(clamp32(float32(accent.R)*0.16, 0, 255)),
			G: uint8(clamp32(float32(accent.G)*0.16, 0, 255)),
			B: uint8(clamp32(float32(accent.B)*0.16, 0, 255)),
			A: 255,
		}
	} else if hov {
		fill = cHover
	}

	roundRect(x, y, w, h, 0.35, fill)

	// Active: bottom accent bar + border glow
	if active {
		roundRectLines(x, y, w, h, 0.35,
			rl.Color{R: accent.R, G: accent.G, B: accent.B, A: 80})
		// Bottom accent line
		rl.DrawRectangle(x+4, y+h-2, w-8, 2,
			rl.Color{R: accent.R, G: accent.G, B: accent.B, A: 200})
	} else if hov {
		roundRectLines(x, y, w, h, 0.35, cBorder)
	}

	// Label
	tc := cTextDim
	if active {
		tc = accent
	} else if hov {
		tc = cTextBrt
	}
	tw := measure(label, fontSizeSm)
	txt(label, x+(w-tw)/2, y+(h-fontSizeSm)/2, fontSizeSm, tc)

	return hov && rl.IsMouseButtonPressed(rl.MouseButtonLeft)
}

// navBtn is a nav-toolbar specific button with a slightly larger hit area style.
func navBtn(x, y, w, h int32, label string, active bool, accent rl.Color) bool {
	mp := rl.GetMousePosition()
	r := rl.Rectangle{X: float32(x), Y: float32(y), Width: float32(w), Height: float32(h)}
	hov := rl.CheckCollisionPointRec(mp, r)

	fill := rl.Color{R: 0, G: 0, B: 0, A: 0} // transparent by default
	if active {
		fill = rl.Color{
			R: uint8(clamp32(float32(accent.R)*0.18, 0, 255)),
			G: uint8(clamp32(float32(accent.G)*0.18, 0, 255)),
			B: uint8(clamp32(float32(accent.B)*0.18, 0, 255)),
			A: 255,
		}
		roundRect(x, y, w, h, 0.40, fill)
		roundRectLines(x, y, w, h, 0.40,
			rl.Color{R: accent.R, G: accent.G, B: accent.B, A: 100})
		// Bottom glow line
		rl.DrawRectangle(x+5, y+h-2, w-10, 2,
			rl.Color{R: accent.R, G: accent.G, B: accent.B, A: 220})
	} else if hov {
		roundRect(x, y, w, h, 0.40, rl.Color{R: 50, G: 50, B: 50, A: 255})
		roundRectLines(x, y, w, h, 0.40, cBorder)
	}

	tc := cTextDim
	if active {
		tc = accent
	} else if hov {
		tc = cTextBrt
	}
	tw := measure(label, fontSizeSm)
	txt(label, x+(w-tw)/2, y+(h-fontSizeSm)/2, fontSizeSm, tc)

	return hov && rl.IsMouseButtonPressed(rl.MouseButtonLeft)
}

// sectionHeader renders a collapsible panel section header.
func sectionHeader(x, y, w int32, title string, open *bool) {
	h := btnH
	mp := rl.GetMousePosition()
	r := rl.Rectangle{X: float32(x), Y: float32(y), Width: float32(w), Height: float32(h)}
	hov := rl.CheckCollisionPointRec(mp, r)

	if hov {
		rl.DrawRectangle(x, y, w, h, cHover)
	} else {
		rl.DrawRectangle(x, y, w, h, cPanel)
	}

	// Left accent bar — tapers to a gradient effect via 2 rects
	rl.DrawRectangle(x, y+2, 2, h-4, cAccent)
	rl.DrawRectangle(x+2, y+4, 1, h-8,
		rl.Color{R: cAccent.R, G: cAccent.G, B: cAccent.B, A: 60})

	txt(title, x+14, y+(h-fontSize)/2, fontSize, cTextBrt)

	arrow := "›"
	if *open {
		arrow = "∨"
	}
	aw := measure(arrow, fontSize)
	txt(arrow, x+w-aw-10, y+(h-fontSize)/2, fontSize,
		rl.Color{R: cAccent.R, G: cAccent.G, B: cAccent.B, A: 160})

	rl.DrawLine(x, y+h, x+w, y+h, cBorderLo)

	if hov && rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
		*open = !*open
	}
}

// labelRow renders a key/value pair.
func labelRow(x, y int32, key, val string, valColor rl.Color) {
	txt(key, x+pad, y, fontSizeSm, cTextDim)
	txt(val, x+pad+94, y, fontSizeSm, valColor)
}

// slider renders a horizontal drag slider.
func slider(x, y, w int32, label string, val *float32, lo, hi float32) {
	if label != "" {
		txt(label, x, y, fontSizeSm, cTextDim)
		y += fontSizeSm + 4
	}
	slH := int32(4)
	// Track background
	roundRect(x, y, w, slH, 0.5, cBorderLo)
	// Filled portion
	t := (*val - lo) / (hi - lo)
	filled := int32(t * float32(w))
	if filled > 2 {
		roundRect(x, y, filled, slH, 0.5, cAccentDim)
	}
	// Thumb
	hx := x + int32(t*float32(w-10))
	roundRect(hx, y-4, 10, slH+8, 0.5, cAccent)
	// Inner thumb shine
	roundRect(hx+2, y-2, 6, 3, 0.5,
		rl.Color{R: 255, G: 255, B: 255, A: 40})

	mp := rl.GetMousePosition()
	hr := rl.Rectangle{X: float32(x), Y: float32(y - 5), Width: float32(w), Height: float32(slH + 10)}
	if rl.CheckCollisionPointRec(mp, hr) && rl.IsMouseButtonDown(rl.MouseButtonLeft) {
		*val = lo + clamp32((mp.X-float32(x))/float32(w), 0, 1)*(hi-lo)
	}
}

// ── Nav Toolbar ───────────────────────────────────────────────
// drawNavToolbar renders the floating on-screen navigation control bar.
func drawNavToolbar(vp rl.Rectangle) {
	const (
		bw  = navBtnW
		bh  = navBtnH
		gap = int32(2)
		div = int32(1)
	)

	// Button groups:  [Orbit Pan] | [+ -] | [Focus Ortho] | [Front Right Top]
	nBtns := 9          // orbit, pan, +, -, focus, ortho, front, right, top
	nDivs := 3          // 3 dividers
	totalW := int32(nBtns)*bw + int32(nBtns-1)*gap + int32(nDivs)*10 + 24 // inner padding
	totalH := bh + 14

	tx := int32(vp.X) + int32(vp.Width)/2 - totalW/2
	ty := int32(vp.Y) + int32(vp.Height) - totalH - 14

	// Backdrop pill
	rl.DrawRectangleRounded(
		rl.Rectangle{X: float32(tx - 2), Y: float32(ty - 2),
			Width: float32(totalW + 4), Height: float32(totalH + 4)},
		0.45, 8,
		rl.Color{R: 0, G: 0, B: 0, A: 60})

	rl.DrawRectangleRounded(
		rl.Rectangle{X: float32(tx), Y: float32(ty),
			Width: float32(totalW), Height: float32(totalH)},
		0.45, 8, cNavBg)

	rl.DrawRectangleRoundedLines(
		rl.Rectangle{X: float32(tx), Y: float32(ty),
			Width: float32(totalW), Height: float32(totalH)},
		0.45, 8, cBorder)

	// Top-edge highlight
	rl.DrawRectangleRounded(
		rl.Rectangle{X: float32(tx + 6), Y: float32(ty),
			Width: float32(totalW - 12), Height: 1},
		0.5, 4,
		rl.Color{R: 255, G: 255, B: 255, A: 12})

	x := tx + 6
	by := ty + (totalH-bh)/2

	// ── Group 1: Navigate ──
	// Orbit
	if navBtn(x, by, bw, bh, "Orbit", ui.navMode == NavOrbit, cAccent) {
		if ui.navMode == NavOrbit {
			ui.navMode = NavDefault
		} else {
			ui.navMode = NavOrbit
		}
	}
	x += bw + gap

	// Pan
	if navBtn(x, by, bw, bh, "Pan", ui.navMode == NavPan, cAccent) {
		if ui.navMode == NavPan {
			ui.navMode = NavDefault
		} else {
			ui.navMode = NavPan
		}
	}
	x += bw + gap + 4

	// Divider
	rl.DrawLine(x, by+4, x, by+bh-4, cBorder)
	x += div + 6

	// ── Group 2: Zoom ──
	if navBtn(x, by, bw, bh, "+  Zoom", false, cAccent) {
		cam.distance *= 0.82
		if cam.distance < 0.05 {
			cam.distance = 0.05
		}
	}
	x += bw + gap

	if navBtn(x, by, bw, bh, "−  Zoom", false, cAccent) {
		cam.distance *= 1.20
	}
	x += bw + gap + 4

	// Divider
	rl.DrawLine(x, by+4, x, by+bh-4, cBorder)
	x += div + 6

	// ── Group 3: Frame + Projection ──
	if navBtn(x, by, bw, bh, "Focus", false, cAccent) {
		camFocus()
	}
	x += bw + gap

	orthoLabel := "Persp"
	if cam.ortho {
		orthoLabel = "Ortho"
	}
	if navBtn(x, by, bw, bh, orthoLabel, cam.ortho, cAccentDim) {
		cam.ortho = !cam.ortho
	}
	x += bw + gap + 4

	// Divider
	rl.DrawLine(x, by+4, x, by+bh-4, cBorder)
	x += div + 6

	// ── Group 4: View presets ──
	if navBtn(x, by, bw, bh, "Front", ui.view == ViewFront, cPurple) {
		camPreset(ViewFront)
	}
	x += bw + gap

	if navBtn(x, by, bw, bh, "Right", ui.view == ViewRight, cPurple) {
		camPreset(ViewRight)
	}
	x += bw + gap

	if navBtn(x, by, bw, bh, "Top", ui.view == ViewTop, cPurple) {
		camPreset(ViewTop)
	}
}

// ── Top bar ───────────────────────────────────────────────────
func drawTopBar(sw int32) {
	// Base
	rl.DrawRectangle(0, 0, sw, topbarH, cPanel)
	// Top accent line (1px)
	rl.DrawRectangle(0, 0, sw, 1, cAccent)
	// Bottom border
	rl.DrawLine(0, topbarH-1, sw, topbarH-1, cBorder)

	// ── Brand ──
	txt("RENDER", pad, (topbarH-fontSizeXL)/2, fontSizeXL, cAccent)
	tw0 := measure("RENDER", fontSizeXL)
	txt("ZERO", pad+tw0+6, (topbarH-fontSizeXL)/2, fontSizeXL, cTextDim)
	tw1 := measure("ZERO", fontSizeXL)

	x := pad + tw0 + 6 + tw1 + 14

	// Separator
	rl.DrawLine(x, 8, x, topbarH-8, cBorder)
	x += 10

	// Open
	if btn(x, 6, 60, topbarH-12, "Open", false, cAccent) {
		if p := openFileDialog(); p != "" {
			loadModel(p)
		}
	}
	x += 64

	// Separator
	rl.DrawLine(x, 8, x, topbarH-8, cBorder)
	x += 10

	// Shading mode
	for i, lbl := range []string{"Solid", "Wire", "Material"} {
		w2 := int32(56)
		if btn(x, 6, w2, topbarH-12, lbl, ui.shading == ShadingMode(i), cAccent) {
			ui.shading = ShadingMode(i)
		}
		x += w2 + 2
	}

	// Separator
	rl.DrawLine(x+2, 8, x+2, topbarH-8, cBorder)
	x += 14

	// Overlays
	for _, pair := range []struct {
		lbl    string
		active *bool
	}{
		{"Grid", &ui.grid},
		{"Axes", &ui.axes},
		{"+Wire", &ui.wireOver},
	} {
		w2 := int32(46)
		if btn(x, 6, w2, topbarH-12, pair.lbl, *pair.active, cPurple) {
			*pair.active = !*pair.active
		}
		x += w2 + 2
	}

	// Separator
	rl.DrawLine(x+2, 8, x+2, topbarH-8, cBorder)
	x += 14

	// View presets
	viewLabels := []string{"Persp", "Front", "Right", "Top"}
	viewVals := []PresetView{ViewPersp, ViewFront, ViewRight, ViewTop}
	for i, lbl := range viewLabels {
		w2 := int32(48)
		if btn(x, 6, w2, topbarH-12, lbl, ui.view == viewVals[i], cAccentDim) {
			camPreset(viewVals[i])
		}
		x += w2 + 2
	}

	// Panel toggle (right-anchored)
	label := "‹‹"
	if !ui.rightPanel {
		label = "››"
	}
	if btn(sw-42, 6, 34, topbarH-12, label, false, cAccent) {
		ui.rightPanel = !ui.rightPanel
	}

	// Center: filename pill
	if obj.loaded {
		info := obj.name + "." + obj.ext
		tw := measure(info, fontSizeSm)
		pill := tw + 22
		px2 := sw/2 - pill/2
		py2 := int32(6)
		roundRect(px2, py2, pill, topbarH-12, 0.5,
			rl.Color{R: 44, G: 44, B: 44, A: 255})
		roundRectLines(px2, py2, pill, topbarH-12, 0.5, cBorderLo)
		txt(info, px2+(pill-tw)/2, py2+(topbarH-12-fontSizeSm)/2, fontSizeSm, cTextDim)
	}
}

// ── Right panel ───────────────────────────────────────────────
func drawRightPanel(sw, sh int32) {
	pr := panelRect(sw, sh)
	px, py := int32(pr.X), int32(pr.Y)
	pw, ph := int32(pr.Width), int32(pr.Height)

	rl.DrawRectangle(px, py, pw, ph, cPanel)
	// Left border — very subtle
	rl.DrawLine(px, py, px, py+ph, cBorder)

	y := py + 8

	// ── Object ──
	sectionHeader(px, y, pw, "Object", &ui.secObject)
	y += btnH + 2
	if ui.secObject {
		if obj.loaded {
			labelRow(px, y, "File", obj.name, cText)
			y += fontSizeSm + 6
			labelRow(px, y, "Format", strings.ToUpper(obj.ext), cText)
			y += fontSizeSm + 6
			labelRow(px, y, "Meshes", fmt.Sprintf("%d", obj.meshCount), cAccent)
			y += fontSizeSm + 6
			labelRow(px, y, "Vertices", fmt.Sprintf("%d", obj.vertexCount), cAccent)
			y += fontSizeSm + 6
			labelRow(px, y, "Triangles", fmt.Sprintf("%d", obj.triCount), cAccent)
			y += fontSizeSm + 6
		} else {
			txt("No model loaded", px+pad, y, fontSizeSm, cTextDim)
			y += fontSizeSm + 6
		}
		y += 4
	}
	rl.DrawLine(px, y, px+pw, y, cBorderLo)
	y += 6

	// ── Transform ──
	sectionHeader(px, y, pw, "Transform", &ui.secTransform)
	y += btnH + 2
	if ui.secTransform {
		labelRow(px, y, "Azimuth", fmt.Sprintf("%.1f°", cam.azimuth*180/math.Pi), cAccent)
		y += fontSizeSm + 6
		labelRow(px, y, "Elevation", fmt.Sprintf("%.1f°", cam.elevation*180/math.Pi), cAccent)
		y += fontSizeSm + 6
		labelRow(px, y, "Distance", fmt.Sprintf("%.3f", cam.distance), cAccent)
		y += fontSizeSm + 6
		mode := "Perspective"
		if cam.ortho {
			mode = "Orthographic"
		}
		labelRow(px, y, "Projection", mode, cText)
		y += fontSizeSm + 8
		if obj.loaded {
			if btn(px+pad, y, pw-pad*2, btnH, "Focus Object  ( . )", false, cAccent) {
				camFocus()
			}
			y += btnH + 6
		}
		y += 4
	}
	rl.DrawLine(px, y, px+pw, y, cBorderLo)
	y += 6

	// ── Display ──
	sectionHeader(px, y, pw, "Display", &ui.secDisplay)
	y += btnH + 2
	if ui.secDisplay {
		sname := [...]string{"Solid", "Wireframe", "Material"}[ui.shading]
		labelRow(px, y, "Shading", sname, cText)
		y += fontSizeSm + 8

		txt("Overlays", px+pad, y, fontSizeSm, cTextDim)
		y += fontSizeSm + 4
		bw2 := (pw - pad*3) / 2
		bx := px + pad
		if btn(bx, y, bw2, btnH, "Grid", ui.grid, cPurple) {
			ui.grid = !ui.grid
		}
		if btn(bx+bw2+pad, y, bw2, btnH, "Axes", ui.axes, cPurple) {
			ui.axes = !ui.axes
		}
		y += btnH + 4
		if btn(bx, y, bw2, btnH, "+Wire", ui.wireOver, cPurple) {
			ui.wireOver = !ui.wireOver
		}
		if btn(bx+bw2+pad, y, bw2, btnH, "Stats", ui.stats, cPurple) {
			ui.stats = !ui.stats
		}
		y += btnH + 8
	}
	rl.DrawLine(px, y, px+pw, y, cBorderLo)
	y += 6

	// ── Lighting ──
	sectionHeader(px, y, pw, "Lighting", &ui.secLighting)
	y += btnH + 2
	if ui.secLighting {
		slw := pw - pad*2

		slider(px+pad, y, slw, "Light Azimuth", &ui.lightAz, -math.Pi, math.Pi)
		y += fontSizeSm + 4 + 8 + 8
		slider(px+pad, y, slw, "Elevation", &ui.lightEl, -math.Pi/2, math.Pi/2)
		y += fontSizeSm + 4 + 8 + 8
		slider(px+pad, y, slw, "Ambient", &ui.ambient, 0, 1)
		y += fontSizeSm + 4 + 8 + 12
	}

	// ── Shortcuts pinned to bottom ──
	bot := py + ph - 118
	rl.DrawLine(px, bot, px+pw, bot, cBorderLo)
	bot += 8

	txt("Keyboard shortcuts", px+pad, bot, fontSizeSm, cTextDim)
	bot += fontSizeSm + 6

	type shortcut struct{ k, v string }
	shortcuts := []shortcut{
		{"O", "Open file"},
		{".", "Focus object"},
		{"Z", "Cycle shading"},
		{"G / A / W", "Grid / Axes / Wire"},
		{"MMB drag", "Orbit"},
		{"Shift+MMB", "Pan"},
		{"Scroll", "Zoom"},
	}
	for _, s := range shortcuts {
		kw := measure(s.k, fontSizeSm)
		txt(s.k, px+pad, bot, fontSizeSm, cAccentDim)
		txt(s.v, px+pad+kw+8, bot, fontSizeSm,
			rl.Color{R: cTextDim.R, G: cTextDim.G, B: cTextDim.B, A: 180})
		bot += fontSizeSm + 4
	}
}

// ── Bottom bar ────────────────────────────────────────────────
func drawBottomBar(sw, sh int32) {
	y := sh - bottombarH
	rl.DrawRectangle(0, y, sw, bottombarH, cPanel)
	rl.DrawLine(0, y, sw, y, cBorder)
	// Bottom purple accent
	rl.DrawRectangle(0, sh-1, sw, 1, cPurpleDim)

	// Status message
	if ui.statusTimer > 0 {
		tc := cText
		if ui.statusTimer < 1.0 {
			tc.A = uint8(ui.statusTimer * 255)
		}
		txt(ui.statusMsg, pad, y+(bottombarH-fontSizeSm)/2, fontSizeSm, tc)
	} else {
		hint := "No model loaded  —  press O to open or drag & drop a file"
		if obj.loaded {
			hint = "Drop a new file to reload  ·  Drag & drop OBJ / FBX / GLTF"
		}
		txt(hint, pad, y+(bottombarH-fontSizeSm)/2, fontSizeSm, cTextDim)
	}

	// Right: FPS + mesh stats
	rightX := sw - pad

	fps := fmt.Sprintf("%d fps", rl.GetFPS())
	tw := measure(fps, fontSizeSm)
	txt(fps, rightX-tw, y+(bottombarH-fontSizeSm)/2, fontSizeSm, cTextDim)
	rightX -= tw + 16

	if obj.loaded {
		s := fmt.Sprintf("%d verts  ·  %d tris", obj.vertexCount, obj.triCount)
		tw2 := measure(s, fontSizeSm)
		txt(s, rightX-tw2, y+(bottombarH-fontSizeSm)/2, fontSizeSm, cTextDim)
		rightX -= tw2 + 16
	}

	// Nav mode indicator
	if ui.navMode != NavDefault {
		modeStr := "● Orbit mode"
		if ui.navMode == NavPan {
			modeStr = "● Pan mode"
		}
		mw := measure(modeStr, fontSizeSm)
		txt(modeStr, rightX-mw, y+(bottombarH-fontSizeSm)/2, fontSizeSm, cAccent)
	}
}

// ── Stats overlay ─────────────────────────────────────────────
func drawStats(vp rl.Rectangle) {
	if !ui.stats {
		return
	}
	names := [...]string{"Persp", "Front", "Back", "Right", "Left", "Top", "Bottom"}
	proj := "Persp"
	if cam.ortho {
		proj = "Ortho"
	}
	s := fmt.Sprintf("%s  ·  %s  ·  dist %.2f  ·  az %.0f°  el %.0f°",
		names[ui.view], proj, cam.distance,
		cam.azimuth*180/math.Pi, cam.elevation*180/math.Pi)
	tw := measure(s, fontSizeSm)
	bx := int32(vp.X) + pad - 4
	by := int32(vp.Y) + pad - 3

	roundRect(bx, by, tw+12, fontSizeSm+8, 0.4,
		rl.Color{R: 28, G: 28, B: 28, A: 210})
	txt(s, bx+6, by+4, fontSizeSm, cTextDim)
}

// drawNoModelHint is intentionally empty — no on-screen prompt shown.
func drawNoModelHint(sw, sh int32) {}

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
	// Escape cancels nav mode
	if rl.IsKeyPressed(rl.KeyEscape) && ui.navMode != NavDefault {
		ui.navMode = NavDefault
	}

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
	rl.InitWindow(1440, 880, "Render Zero")
	rl.SetTargetFPS(144)
	rl.SetExitKey(0)

	camInit()
	shaderInit()
	initFont()

	ui = UIState{
		shading:      ShadeSolid,
		view:         ViewPersp,
		navMode:      NavDefault,
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
		drawNavToolbar(vp)

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
	if gFontLoaded {
		rl.UnloadFont(gFontSm)
		rl.UnloadFont(gFontMd)
		rl.UnloadFont(gFontLg)
		rl.UnloadFont(gFontXL)
	}
	rl.UnloadShader(shd)
	rl.CloseWindow()
}