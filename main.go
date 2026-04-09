// ============================================================
//  RENDER ZERO — 3D Scene Viewer & Renderer
//  Go + raylib-go | Windows only
//  UI: Void Precision — Zhenya Rynzhuk × Jony Ive
// ============================================================
//
//  KEYBOARD CONTROLS
//  ─────────────────
//  Middle Mouse Drag        Orbit
//  Shift + MMB Drag         Pan
//  Scroll Wheel             Zoom
//  O                        Open / add model to scene
//  H                        Load HDRI environment
//  Drag & Drop              Load OBJ/FBX/GLTF → adds to scene
//                           Drop .hdr          → loads as HDRI
//  Numpad 1/3/7             Front / Right / Top view
//  Ctrl+Numpad 1/3/7        Back / Left / Bottom
//  Numpad 0                 Perspective reset
//  Numpad 5                 Toggle ortho / persp
//  . (Period)               Focus selected / all
//  G / W / A                Grid / Wire overlays / Axes
//  Z                        Cycle shading mode
//  Delete                   Remove selected object/camera
//  C                        Add render camera at current view
//  R                        Render from active camera
//  F11                      Fullscreen
//  Escape                   Close render output / cancel nav
// ============================================================

package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// ── Layout constants ──────────────────────────────────────────
const (
	topbarH        = int32(42)
	bottombarH     = int32(26)
	panelRW        = int32(300)
	outlineW       = int32(224)
	fontSize       = int32(16)
	fontSizeSm     = int32(14)
	fontSizeLg     = int32(20)
	fontSizeXL     = int32(24)
	pad            = int32(10)
	btnH           = int32(28)
	gizmoSize      = float32(68)
	gizmoMargin    = float32(18)
	navBtnW        = int32(62)
	navBtnH        = int32(32)
	rowH           = int32(26)
	maxPointLights = 4
)

// ── Palette ───────────────────────────────────────────────────
var (
	cBG        = rl.Color{R: 28, G: 28, B: 28, A: 255}
	cPanel     = rl.Color{R: 36, G: 36, B: 36, A: 255}
	cPanelDark = rl.Color{R: 30, G: 30, B: 30, A: 255}
	cBorder    = rl.Color{R: 18, G: 18, B: 18, A: 255}
	cBorderLo  = rl.Color{R: 26, G: 26, B: 26, A: 255}
	cAccent    = rl.Color{R: 232, G: 125, B: 13, A: 255}
	cAccentDim = rl.Color{R: 160, G: 84, B: 8, A: 255}
	cPurple    = rl.Color{R: 78, G: 142, B: 166, A: 255}
	cPurpleDim = rl.Color{R: 42, G: 88, B: 106, A: 255}
	cGreen     = rl.Color{R: 80, G: 185, B: 105, A: 255}
	cGold      = rl.Color{R: 215, G: 180, B: 50, A: 255}
	cRed       = rl.Color{R: 200, G: 70, B: 60, A: 255}
	cText      = rl.Color{R: 210, G: 210, B: 210, A: 255}
	cTextDim   = rl.Color{R: 120, G: 120, B: 120, A: 255}
	cTextBrt   = rl.Color{R: 240, G: 240, B: 240, A: 255}
	cHover     = rl.Color{R: 50, G: 50, B: 50, A: 255}
	cSelRow    = rl.Color{R: 232, G: 125, B: 13, A: 30}
	cSelBorder = rl.Color{R: 232, G: 125, B: 13, A: 90}
	cGridMin   = rl.Color{R: 38, G: 38, B: 38, A: 255}
	cGridMaj   = rl.Color{R: 52, G: 52, B: 52, A: 255}
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
	NavDefault NavMode = iota
	NavOrbit
	NavPan
)

type SceneItemType int

const (
	ItemMesh SceneItemType = iota
	ItemCamera
	ItemLight
)

// ── Structs ───────────────────────────────────────────────────
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

type SceneObject struct {
	id          int
	model       rl.Model
	loaded      bool
	name        string
	ext         string
	sourcePath  string
	bounds      rl.BoundingBox
	center      rl.Vector3
	scaleFactor float32
	meshCount   int
	vertexCount int
	triCount    int
	visible     bool
	// User transform
	position  rl.Vector3
	rotY      float32 // degrees, Y-axis
	userScale float32 // multiplier on top of scaleFactor
}

type RenderCamera struct {
	id       int
	name     string
	position rl.Vector3
	target   rl.Vector3
	fovy     float32
	active   bool // designated render camera
}

type SceneLight struct {
	id        int
	name      string
	position  rl.Vector3
	color     rl.Color
	intensity float32
	rangeDist float32
	enabled   bool
}

type HDRIState struct {
	loaded          bool
	name            string
	sourcePath      string
	panorama        rl.Texture2D
	skyboxModel     rl.Model
	skyboxShader    rl.Shader
	intensity       float32
	skyRotation     float32
	useIBL          bool
	locSkyIntensity int32
	locSkyRotation  int32
}

type RenderOutput struct {
	texture    rl.RenderTexture2D
	width      int32
	height     int32
	rendered   bool
	showOutput bool
}

type SavedOrbitCam struct {
	Target    [3]float32 `json:"target"`
	Azimuth   float32    `json:"azimuth"`
	Elevation float32    `json:"elevation"`
	Distance  float32    `json:"distance"`
	Fovy      float32    `json:"fovy"`
	Ortho     bool       `json:"ortho"`
}

type SavedSceneObject struct {
	Path     string     `json:"path"`
	Name     string     `json:"name"`
	Position [3]float32 `json:"position"`
	RotY     float32    `json:"rotY"`
	Scale    float32    `json:"scale"`
	Visible  bool       `json:"visible"`
}

type SavedRenderCamera struct {
	Name     string     `json:"name"`
	Position [3]float32 `json:"position"`
	Target   [3]float32 `json:"target"`
	Fovy     float32    `json:"fovy"`
	Active   bool       `json:"active"`
}

type SavedSceneLight struct {
	Name      string     `json:"name"`
	Position  [3]float32 `json:"position"`
	Color     [4]uint8   `json:"color"`
	Intensity float32    `json:"intensity"`
	RangeDist float32    `json:"range"`
	Enabled   bool       `json:"enabled"`
}

type SavedEnvironment struct {
	Path      string  `json:"path"`
	Intensity float32 `json:"intensity"`
	Rotation  float32 `json:"rotation"`
	UseIBL    bool    `json:"useIBL"`
	IsLoaded  bool    `json:"isLoaded"`
}

type SavedRenderSettings struct {
	Width   int32   `json:"width"`
	Height  int32   `json:"height"`
	LightAz float32 `json:"lightAz"`
	LightEl float32 `json:"lightEl"`
	Ambient float32 `json:"ambient"`
	Shading int     `json:"shading"`
	Grid    bool    `json:"grid"`
	Axes    bool    `json:"axes"`
	Wire    bool    `json:"wire"`
	Stats   bool    `json:"stats"`
}

type SavedSceneFile struct {
	Version     int                 `json:"version"`
	Orbit       SavedOrbitCam       `json:"orbit"`
	Objects     []SavedSceneObject  `json:"objects"`
	Cameras     []SavedRenderCamera `json:"cameras"`
	Lights      []SavedSceneLight   `json:"lights"`
	Environment SavedEnvironment    `json:"environment"`
	Render      SavedRenderSettings `json:"render"`
}

type UIState struct {
	outliner     bool
	rightPanel   bool
	selectedID   int
	selectedType SceneItemType
	// Viewport
	shading  ShadingMode
	view     PresetView
	navMode  NavMode
	grid     bool
	axes     bool
	wireOver bool
	stats    bool
	// Panel sections
	secOutliner    bool
	secObject      bool
	secTransform   bool
	secCamera      bool
	secDisplay     bool
	secLighting    bool
	secEnvironment bool
	secRender      bool
	// Lighting (global)
	lightAz float32
	lightEl float32
	ambient float32
	// Render settings
	renderW int32
	renderH int32
	// Status
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
uniform vec4      colDiffuse;
uniform vec3      uLightDir;
uniform vec3      uLightColor;
uniform float     uAmbient;
uniform vec3      uViewPos;
uniform sampler2D uEnvPano;
uniform float     uUseHDRI;
uniform float     uEnvIntensity;
uniform float     uEnvRotation;
uniform vec4      uPointLightPosIntensity[4];
uniform vec4      uPointLightColorRange[4];
out vec4 fragColor;
vec2 dirToUV(vec3 d) {
    float s = sin(uEnvRotation), c = cos(uEnvRotation);
    d = vec3(d.x*c - d.z*s, d.y, d.x*s + d.z*c);
    d = normalize(d);
    return vec2(atan(d.z, d.x) * 0.15915 + 0.5,
                asin(clamp(d.y, -1.0, 1.0)) * 0.31831 + 0.5);
}
void main() {
    vec4  tex  = texture(texture0, vUV) * colDiffuse;
    vec3  N    = normalize(vNorm);
    vec3  L    = normalize(-uLightDir);
    float diff = max(dot(N, L), 0.0);
    vec3  V    = normalize(uViewPos - vPos);
    vec3  H    = normalize(L + V);
    float spec = pow(max(dot(N, H), 0.0), 48.0) * 0.25;
    float amb  = uAmbient;
    vec3  pointLit = vec3(0.0);
    vec3  envDiff = vec3(0.0);
    vec3  envSpec = vec3(0.0);
    for (int i = 0; i < 4; i++) {
        vec3  lightPos = uPointLightPosIntensity[i].xyz;
        float lightIntensity = uPointLightPosIntensity[i].w;
        float lightRange = uPointLightColorRange[i].w;
        if (lightIntensity <= 0.001 || lightRange <= 0.001) continue;
        vec3 toL = lightPos - vPos;
        float dist = length(toL);
        if (dist >= lightRange) continue;
        vec3 Lp = toL / max(dist, 0.0001);
        float atten = 1.0 - dist/lightRange;
        atten *= atten;
        float pdiff = max(dot(N, Lp), 0.0) * lightIntensity * atten;
        vec3 Hp = normalize(Lp + V);
        float pspec = pow(max(dot(N, Hp), 0.0), 24.0) * 0.18 * lightIntensity * atten;
        pointLit += (pdiff + pspec) * uPointLightColorRange[i].rgb;
    }
    if (uUseHDRI > 0.5) {
        vec3  envN = texture(uEnvPano, dirToUV(N)).rgb * uEnvIntensity;
        vec3  envR = texture(uEnvPano, dirToUV(reflect(-V, N))).rgb * uEnvIntensity;
        envDiff = envN / (envN + vec3(1.0));
        envSpec = envR / (envR + vec3(1.0));
        amb = max(uAmbient, 0.06 + uEnvIntensity * 0.10);
    }
    vec3 lit = (amb + diff) * uLightColor + spec * uLightColor + pointLit + envDiff * 0.45 + envSpec * 0.30;
    fragColor = vec4(clamp(lit, 0.0, 1.0) * tex.rgb, tex.a);
}`

const skyboxVS = `#version 330 core
in vec3 vertexPosition;
uniform mat4 matProjection;
uniform mat4 matView;
out vec3 vDir;
void main() {
    vDir = vertexPosition;
    mat4 rv = mat4(mat3(matView));
    gl_Position = matProjection * rv * vec4(vertexPosition, 1.0);
}`

const skyboxFS = `#version 330 core
in vec3 vDir;
uniform sampler2D environmentMap;
uniform float uSkyIntensity;
uniform float uSkyRotation;
out vec4 fragColor;
void main() {
    float s = sin(uSkyRotation), c = cos(uSkyRotation);
    vec3  d = vec3(vDir.x*c - vDir.z*s, vDir.y, vDir.x*s + vDir.z*c);
    d = normalize(d);
    vec2 uv = vec2(atan(d.z, d.x) * 0.15915 + 0.5,
                   asin(clamp(d.y, -1.0, 1.0)) * 0.31831 + 0.5);
    vec3  col = texture(environmentMap, uv).rgb * uSkyIntensity;
    col = col / (col + vec3(1.0));
    col = pow(col, vec3(1.0/2.2));
    fragColor = vec4(col, 1.0);
}`

// ── Globals ───────────────────────────────────────────────────
var (
	cam             OrbitCam
	scene           []SceneObject
	cameras         []RenderCamera
	lights          []SceneLight
	nextID          = 1
	ui              UIState
	hdri            HDRIState
	renderOut       RenderOutput
	sceneLoadActive bool
	shd             rl.Shader
	locLightDir     int32
	locLightColor   int32
	locAmbient      int32
	locViewPos      int32
	locMatModel     int32
	locEnvPano      int32
	locUseHDRI      int32
	locEnvIntensity int32
	locEnvRotation  int32
	locPointPos     [4]int32
	locPointColor   [4]int32
)

// ── Font helpers ──────────────────────────────────────────────
var (
	gFontLoaded bool
	gFontSm     rl.Font
	gFontMd     rl.Font
	gFontLg     rl.Font
	gFontXL     rl.Font
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
		gFontLoaded = true
		return
	}
}

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

func txt(s string, x, y, sz int32, c rl.Color) {
	if gFontLoaded {
		rl.DrawTextEx(fontFor(sz), s, rl.Vector2{X: float32(x), Y: float32(y)},
			float32(sz), 1.0, c)
	} else {
		rl.DrawText(s, x, y, sz, c)
	}
}

func txtC(s string, cx, y, sz int32, c rl.Color) {
	txt(s, cx-measure(s, sz)/2, y, sz, c)
}

func measure(s string, sz int32) int32 {
	if gFontLoaded {
		return int32(rl.MeasureTextEx(fontFor(sz), s, float32(sz), 1.0).X)
	}
	return rl.MeasureText(s, sz)
}

// ── Misc helpers ──────────────────────────────────────────────
func setStatus(f string, a ...any) {
	ui.statusMsg = fmt.Sprintf(f, a...)
	ui.statusTimer = 4.0
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

func vpRect(sw, sh int32) rl.Rectangle {
	lw := int32(0)
	if ui.outliner {
		lw = outlineW
	}
	pw := int32(0)
	if ui.rightPanel {
		pw = panelRW
	}
	return rl.Rectangle{
		X: float32(lw), Y: float32(topbarH),
		Width:  float32(sw - lw - pw),
		Height: float32(sh - topbarH - bottombarH),
	}
}

func panelRect(sw, sh int32) rl.Rectangle {
	return rl.Rectangle{
		X: float32(sw - panelRW), Y: float32(topbarH),
		Width: float32(panelRW), Height: float32(sh - topbarH - bottombarH),
	}
}

// ── Selection helpers ─────────────────────────────────────────
func getSelectedObject() *SceneObject {
	if ui.selectedType != ItemMesh {
		return nil
	}
	for i := range scene {
		if scene[i].id == ui.selectedID {
			return &scene[i]
		}
	}
	return nil
}

func getSelectedCamera() *RenderCamera {
	if ui.selectedType != ItemCamera {
		return nil
	}
	for i := range cameras {
		if cameras[i].id == ui.selectedID {
			return &cameras[i]
		}
	}
	return nil
}

func getSelectedLight() *SceneLight {
	if ui.selectedType != ItemLight {
		return nil
	}
	for i := range lights {
		if lights[i].id == ui.selectedID {
			return &lights[i]
		}
	}
	return nil
}

func getActiveRenderCamera() *RenderCamera {
	for i := range cameras {
		if cameras[i].active {
			return &cameras[i]
		}
	}
	if len(cameras) > 0 {
		return &cameras[0]
	}
	return nil
}

func vec3ToArray(v rl.Vector3) [3]float32 {
	return [3]float32{v.X, v.Y, v.Z}
}

func arrayToVec3(v [3]float32) rl.Vector3 {
	return rl.Vector3{X: v[0], Y: v[1], Z: v[2]}
}

func colorToArray(c rl.Color) [4]uint8 {
	return [4]uint8{c.R, c.G, c.B, c.A}
}

func arrayToColor(c [4]uint8) rl.Color {
	return rl.Color{R: c[0], G: c[1], B: c[2], A: c[3]}
}

// ── Orbit camera ─────────────────────────────────────────────
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

func camFocusAll() {
	if len(scene) == 0 {
		return
	}
	first := true
	minV := rl.Vector3{}
	maxV := rl.Vector3{}
	for _, o := range scene {
		if !o.loaded || !o.visible {
			continue
		}
		ns := o.scaleFactor * o.userScale
		ext := rl.Vector3Scale(rl.Vector3Subtract(o.bounds.Max, o.bounds.Min), ns*0.5)
		lo := rl.Vector3Subtract(o.position, ext)
		hi := rl.Vector3Add(o.position, ext)
		if first {
			minV, maxV = lo, hi
			first = false
		} else {
			if lo.X < minV.X {
				minV.X = lo.X
			}
			if lo.Y < minV.Y {
				minV.Y = lo.Y
			}
			if lo.Z < minV.Z {
				minV.Z = lo.Z
			}
			if hi.X > maxV.X {
				maxV.X = hi.X
			}
			if hi.Y > maxV.Y {
				maxV.Y = hi.Y
			}
			if hi.Z > maxV.Z {
				maxV.Z = hi.Z
			}
		}
	}
	if first {
		return
	}
	cam.target = rl.Vector3Scale(rl.Vector3Add(minV, maxV), 0.5)
	diag := rl.Vector3Length(rl.Vector3Subtract(maxV, minV))
	cam.distance = float32(math.Max(float64(diag*1.4), 0.5))
}

func camFocusSelected() {
	o := getSelectedObject()
	if o == nil {
		camFocusAll()
		return
	}
	ns := o.scaleFactor * o.userScale
	sz := rl.Vector3Length(rl.Vector3Subtract(o.bounds.Max, o.bounds.Min))
	cam.target = o.position
	cam.distance = float32(math.Max(float64(sz*ns*1.4), 0.5))
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

	if inVP && scroll != 0 {
		cam.distance -= scroll * cam.distance * 0.08
		if cam.distance < 0.05 {
			cam.distance = 0.05
		}
	}

	activeOrbit := (mmb && !shift && inVP) || (lmb && ui.navMode == NavOrbit && inVP)
	activePan := (mmb && shift && inVP) || (lmb && ui.navMode == NavPan && inVP)

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

// ── Shader system ─────────────────────────────────────────────
func shaderInit() {
	shd = rl.LoadShaderFromMemory(vsSource, fsSource)
	locLightDir = rl.GetShaderLocation(shd, "uLightDir")
	locLightColor = rl.GetShaderLocation(shd, "uLightColor")
	locAmbient = rl.GetShaderLocation(shd, "uAmbient")
	locViewPos = rl.GetShaderLocation(shd, "uViewPos")
	locMatModel = rl.GetShaderLocation(shd, "matModel")
	locEnvPano = rl.GetShaderLocation(shd, "uEnvPano")
	locUseHDRI = rl.GetShaderLocation(shd, "uUseHDRI")
	locEnvIntensity = rl.GetShaderLocation(shd, "uEnvIntensity")
	locEnvRotation = rl.GetShaderLocation(shd, "uEnvRotation")
	for i := 0; i < maxPointLights; i++ {
		locPointPos[i] = rl.GetShaderLocation(shd, fmt.Sprintf("uPointLightPosIntensity[%d]", i))
		locPointColor[i] = rl.GetShaderLocation(shd, fmt.Sprintf("uPointLightColorRange[%d]", i))
	}

	setShaderLoc(&shd, rl.ShaderLocMatrixModel, locMatModel)
	setShaderLoc(&shd, rl.ShaderLocVectorView, locViewPos)
}

func setShaderLoc(s *rl.Shader, slot, value int32) {
	if s.Locs == nil {
		return
	}
	p := (*int32)(unsafe.Pointer(
		uintptr(unsafe.Pointer(s.Locs)) + uintptr(slot)*unsafe.Sizeof(int32(0))))
	*p = value
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

	useHDRI := float32(0)
	intensity := float32(0)
	rotation := float32(0)
	if hdri.loaded && hdri.useIBL {
		useHDRI = 1
		intensity = hdri.intensity
		rotation = hdri.skyRotation
		rl.SetShaderValueTexture(shd, locEnvPano, hdri.panorama)
	}
	rl.SetShaderValue(shd, locUseHDRI, []float32{useHDRI}, rl.ShaderUniformFloat)
	rl.SetShaderValue(shd, locEnvIntensity, []float32{intensity}, rl.ShaderUniformFloat)
	rl.SetShaderValue(shd, locEnvRotation, []float32{rotation}, rl.ShaderUniformFloat)

	var pointPos [maxPointLights * 4]float32
	var pointColor [maxPointLights * 4]float32
	lightIndex := 0
	for i := range lights {
		if lightIndex >= maxPointLights {
			break
		}
		l := lights[i]
		if !l.enabled {
			continue
		}
		base := lightIndex * 4
		pointPos[base+0] = l.position.X
		pointPos[base+1] = l.position.Y
		pointPos[base+2] = l.position.Z
		pointPos[base+3] = l.intensity
		pointColor[base+0] = float32(l.color.R) / 255.0
		pointColor[base+1] = float32(l.color.G) / 255.0
		pointColor[base+2] = float32(l.color.B) / 255.0
		pointColor[base+3] = l.rangeDist
		lightIndex++
	}
	if locPointPos[0] >= 0 {
		rl.SetShaderValueV(shd, locPointPos[0], pointPos[:], rl.ShaderUniformVec4, int32(maxPointLights))
	}
	if locPointColor[0] >= 0 {
		rl.SetShaderValueV(shd, locPointColor[0], pointColor[:], rl.ShaderUniformVec4, int32(maxPointLights))
	}
}

// ── HDRI system ───────────────────────────────────────────────
func hdriInit() {
	hdri.skyboxShader = rl.LoadShaderFromMemory(skyboxVS, skyboxFS)
	hdri.locSkyIntensity = rl.GetShaderLocation(hdri.skyboxShader, "uSkyIntensity")
	hdri.locSkyRotation = rl.GetShaderLocation(hdri.skyboxShader, "uSkyRotation")
	hdri.intensity = 1.0
	hdri.useIBL = true
}

func unloadHDRI() {
	if hdri.panorama.ID != 0 {
		rl.UnloadTexture(hdri.panorama)
	}
	if hdri.skyboxModel.MaterialCount != 0 {
		rl.UnloadModel(hdri.skyboxModel)
	}
	hdri.panorama = rl.Texture2D{}
	hdri.skyboxModel = rl.Model{}
	hdri.loaded = false
	hdri.name = ""
	hdri.sourcePath = ""
}

func loadHDRI(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".hdr", ".png", ".jpg", ".jpeg", ".tga":
	default:
		if ext == ".exr" {
			setStatus("HDRI: .exr is not supported by this build, use .hdr or LDR images")
		} else {
			setStatus("HDRI: use .hdr or image formats")
		}
		return false
	}

	if hdri.loaded {
		unloadHDRI()
	}

	img := rl.LoadImage(path)
	if img == nil || img.Data == nil {
		if img != nil {
			rl.UnloadImage(img)
		}
		setStatus("HDRI: failed to load - %s", filepath.Base(path))
		return false

	}
	if ext == ".hdr" {
		rl.ImageFormat(img, rl.UncompressedR32g32b32a32)
	}
	hdri.panorama = rl.LoadTextureFromImage(img)
	rl.UnloadImage(img)
	if hdri.panorama.ID == 0 {
		setStatus("HDRI: texture creation failed")
		return false
	}
	rl.SetTextureFilter(hdri.panorama, rl.FilterBilinear)
	rl.SetTextureWrap(hdri.panorama, rl.WrapClamp)

	// Build skybox cube
	mesh := rl.GenMeshCube(1, 1, 1)
	hdri.skyboxModel = rl.LoadModelFromMesh(mesh)

	// Assign skybox shader + panorama texture to material
	mat := modelMaterial(hdri.skyboxModel, 0)
	mat.Shader = hdri.skyboxShader

	mat.GetMap(rl.MapAlbedo).Texture = hdri.panorama
	panoLoc := rl.GetShaderLocation(hdri.skyboxShader, "environmentMap")
	rl.SetShaderValueTexture(hdri.skyboxShader, panoLoc, hdri.panorama)

	hdri.loaded = true
	hdri.name = filepath.Base(path)
	hdri.sourcePath = path
	setStatus("Environment loaded: %s", hdri.name)
	return true
}

func drawSkybox() {
	if !hdri.loaded {
		return
	}
	rl.SetShaderValue(hdri.skyboxShader, hdri.locSkyIntensity,
		[]float32{hdri.intensity}, rl.ShaderUniformFloat)
	rl.SetShaderValue(hdri.skyboxShader, hdri.locSkyRotation,
		[]float32{hdri.skyRotation}, rl.ShaderUniformFloat)
	rl.DisableDepthTest()
	rl.DrawModel(hdri.skyboxModel, rl.Vector3{}, 1.0, rl.White)
	rl.EnableDepthTest()
}

// ── Model loading ─────────────────────────────────────────────
func addModelToScene(path string) bool {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	switch ext {
	case "obj", "fbx", "gltf", "glb", "iqm", "m3d":
	default:
		setStatus("Unsupported format - use OBJ, FBX, GLTF, IQM, M3D")
		return false
	}

	restoreLogs := false
	if ext == "obj" {
		rl.SetTraceLogLevel(rl.LogError)
		restoreLogs = true
	}
	mdl := rl.LoadModel(path)
	if restoreLogs {
		rl.SetTraceLogLevel(rl.LogInfo)
	}
	if mdl.MeshCount == 0 {
		setStatus("Failed to load: %s", filepath.Base(path))
		return false
	}

	base := filepath.Base(path)
	name := base
	if dot := strings.LastIndex(base, "."); dot >= 0 {
		name = base[:dot]
	}

	var vc, tc int
	mc := int(mdl.MeshCount)
	for i := 0; i < mc; i++ {
		m := modelMesh(mdl, i)
		vc += int(m.VertexCount)
		tc += int(m.TriangleCount)
	}

	bb := rl.GetModelBoundingBox(mdl)
	center := rl.Vector3Scale(rl.Vector3Add(bb.Min, bb.Max), 0.5)
	diag := rl.Vector3Length(rl.Vector3Subtract(bb.Max, bb.Min))
	sf := float32(1)
	if diag > 0.001 {
		sf = 2.0 / diag
	}

	// Assign main shader to all materials
	for i := int32(0); i < mdl.MaterialCount; i++ {
		mat := modelMaterial(mdl, int(i))
		mat.Shader = shd
	}

	obj := SceneObject{
		id:          nextID,
		model:       mdl,
		loaded:      true,
		name:        name,
		ext:         ext,
		sourcePath:  path,
		bounds:      bb,
		center:      center,
		scaleFactor: sf,
		meshCount:   mc,
		vertexCount: vc,
		triCount:    tc,
		visible:     true,
		userScale:   1.0,
	}

	// Offset new objects so they don't stack exactly
	if len(scene) > 0 {
		obj.position.X = float32(len(scene)) * 0.3
	}

	scene = append(scene, obj)
	nextID++

	ui.selectedID = obj.id
	ui.selectedType = ItemMesh

	if len(scene) == 1 && !sceneLoadActive {
		camFocusAll()
	}
	setStatus("Added %s  (%d meshes, %d verts, %d tris)", base, mc, vc, tc)
	return true
}

func removeSelected() {
	if ui.selectedType == ItemMesh {
		for i := range scene {
			if scene[i].id == ui.selectedID {
				rl.UnloadModel(scene[i].model)
				scene = append(scene[:i], scene[i+1:]...)
				ui.selectedID = -1
				setStatus("Object removed")
				return
			}
		}
	} else if ui.selectedType == ItemCamera {
		for i := range cameras {
			if cameras[i].id == ui.selectedID {
				cameras = append(cameras[:i], cameras[i+1:]...)
				ui.selectedID = -1
				setStatus("Camera removed")
				return
			}
		}
	} else if ui.selectedType == ItemLight {
		for i := range lights {
			if lights[i].id == ui.selectedID {
				lights = append(lights[:i], lights[i+1:]...)
				ui.selectedID = -1
				setStatus("Light removed")
				return
			}
		}
	}
}

func addRenderCamera() {
	c := camGet()
	name := fmt.Sprintf("Camera.%03d", nextID)
	rc := RenderCamera{
		id:       nextID,
		name:     name,
		position: c.Position,
		target:   c.Target,
		fovy:     50.0,
		active:   len(cameras) == 0,
	}
	cameras = append(cameras, rc)
	nextID++
	ui.selectedID = rc.id
	ui.selectedType = ItemCamera
	setStatus("Added %s at viewport position", name)
}

func addSceneLight() {
	c := camGet()
	dir := rl.Vector3Normalize(rl.Vector3Subtract(c.Target, c.Position))
	pos := rl.Vector3Add(c.Target, rl.Vector3Scale(dir, -1.5))
	name := fmt.Sprintf("Light.%03d", nextID)
	light := SceneLight{
		id:        nextID,
		name:      name,
		position:  pos,
		color:     rl.Color{R: 255, G: 236, B: 214, A: 255},
		intensity: 1.8,
		rangeDist: 8.0,
		enabled:   true,
	}
	lights = append(lights, light)
	nextID++
	ui.selectedID = light.id
	ui.selectedType = ItemLight
	setStatus("Added %s", name)
}

func clearScene() {
	for i := range scene {
		if scene[i].loaded {
			rl.UnloadModel(scene[i].model)
		}
	}
	scene = nil
	cameras = nil
	lights = nil
	unloadHDRI()
	ui.selectedID = -1
	ui.selectedType = ItemMesh
	renderOut.rendered = false
	renderOut.showOutput = false
	nextID = 1
}

func saveSceneToPath(path string) error {
	data := SavedSceneFile{
		Version: 1,
		Orbit: SavedOrbitCam{
			Target:    vec3ToArray(cam.target),
			Azimuth:   cam.azimuth,
			Elevation: cam.elevation,
			Distance:  cam.distance,
			Fovy:      cam.fovy,
			Ortho:     cam.ortho,
		},
		Render: SavedRenderSettings{
			Width:   ui.renderW,
			Height:  ui.renderH,
			LightAz: ui.lightAz,
			LightEl: ui.lightEl,
			Ambient: ui.ambient,
			Shading: int(ui.shading),
			Grid:    ui.grid,
			Axes:    ui.axes,
			Wire:    ui.wireOver,
			Stats:   ui.stats,
		},
		Environment: SavedEnvironment{
			Path:      hdri.sourcePath,
			Intensity: hdri.intensity,
			Rotation:  hdri.skyRotation,
			UseIBL:    hdri.useIBL,
			IsLoaded:  hdri.loaded,
		},
	}
	for _, obj := range scene {
		data.Objects = append(data.Objects, SavedSceneObject{
			Path:     obj.sourcePath,
			Name:     obj.name,
			Position: vec3ToArray(obj.position),
			RotY:     obj.rotY,
			Scale:    obj.userScale,
			Visible:  obj.visible,
		})
	}
	for _, rc := range cameras {
		data.Cameras = append(data.Cameras, SavedRenderCamera{
			Name:     rc.name,
			Position: vec3ToArray(rc.position),
			Target:   vec3ToArray(rc.target),
			Fovy:     rc.fovy,
			Active:   rc.active,
		})
	}
	for _, l := range lights {
		data.Lights = append(data.Lights, SavedSceneLight{
			Name:      l.name,
			Position:  vec3ToArray(l.position),
			Color:     colorToArray(l.color),
			Intensity: l.intensity,
			RangeDist: l.rangeDist,
			Enabled:   l.enabled,
		})
	}
	buf, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, buf, 0644)
}

func loadSceneFromPath(path string) error {
	buf, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var data SavedSceneFile
	if err := json.Unmarshal(buf, &data); err != nil {
		return err
	}

	clearScene()
	sceneLoadActive = true
	defer func() { sceneLoadActive = false }()

	ui.renderW = data.Render.Width
	ui.renderH = data.Render.Height
	ui.lightAz = data.Render.LightAz
	ui.lightEl = data.Render.LightEl
	ui.ambient = data.Render.Ambient
	ui.shading = ShadingMode(data.Render.Shading)
	ui.grid = data.Render.Grid
	ui.axes = data.Render.Axes
	ui.wireOver = data.Render.Wire
	ui.stats = data.Render.Stats

	cam.target = arrayToVec3(data.Orbit.Target)
	cam.azimuth = data.Orbit.Azimuth
	cam.elevation = data.Orbit.Elevation
	cam.distance = data.Orbit.Distance
	cam.fovy = data.Orbit.Fovy
	cam.ortho = data.Orbit.Ortho

	var missing []string
	for _, so := range data.Objects {
		if so.Path == "" {
			continue
		}
		if !addModelToScene(so.Path) {
			missing = append(missing, filepath.Base(so.Path))
			continue
		}
		obj := &scene[len(scene)-1]
		if so.Name != "" {
			obj.name = so.Name
		}
		obj.position = arrayToVec3(so.Position)
		obj.rotY = so.RotY
		if so.Scale > 0 {
			obj.userScale = so.Scale
		}
		obj.visible = so.Visible
	}
	for _, sc := range data.Cameras {
		name := sc.Name
		if name == "" {
			name = fmt.Sprintf("Camera.%03d", nextID)
		}
		rc := RenderCamera{
			id:       nextID,
			name:     name,
			position: arrayToVec3(sc.Position),
			target:   arrayToVec3(sc.Target),
			fovy:     sc.Fovy,
			active:   sc.Active,
		}
		if rc.fovy <= 0 {
			rc.fovy = 50
		}
		cameras = append(cameras, rc)
		nextID++
	}
	if len(cameras) > 0 {
		hasActive := false
		for i := range cameras {
			if cameras[i].active {
				hasActive = true
				break
			}
		}
		if !hasActive {
			cameras[0].active = true
		}
	}
	for _, sl := range data.Lights {
		name := sl.Name
		if name == "" {
			name = fmt.Sprintf("Light.%03d", nextID)
		}
		light := SceneLight{
			id:        nextID,
			name:      name,
			position:  arrayToVec3(sl.Position),
			color:     arrayToColor(sl.Color),
			intensity: sl.Intensity,
			rangeDist: sl.RangeDist,
			enabled:   sl.Enabled,
		}
		if light.color.A == 0 {
			light.color.A = 255
		}
		if light.rangeDist <= 0 {
			light.rangeDist = 8
		}
		lights = append(lights, light)
		nextID++
	}
	if data.Environment.IsLoaded && data.Environment.Path != "" {
		if !loadHDRI(data.Environment.Path) {
			missing = append(missing, filepath.Base(data.Environment.Path))
		} else {
			hdri.intensity = data.Environment.Intensity
			hdri.skyRotation = data.Environment.Rotation
			hdri.useIBL = data.Environment.UseIBL
		}
	}
	ui.selectedID = -1
	ui.selectedType = ItemMesh
	if len(missing) > 0 {
		setStatus("Scene loaded with missing assets: %s", strings.Join(missing, ", "))
	} else {
		setStatus("Scene loaded: %s", filepath.Base(path))
	}
	return nil
}

func saveScene() {
	if path := saveSceneDialog(); path != "" {
		if err := saveSceneToPath(path); err != nil {
			setStatus("Scene save failed: %v", err)
			return
		}
		setStatus("Scene saved: %s", filepath.Base(path))
	}
}

func loadScene() {
	if path := openSceneDialog(); path != "" {
		if err := loadSceneFromPath(path); err != nil {
			setStatus("Scene load failed: %v", err)
		}
	}
}

func modelMesh(m rl.Model, i int) *rl.Mesh {
	return (*rl.Mesh)(unsafe.Pointer(
		uintptr(unsafe.Pointer(m.Meshes)) + uintptr(i)*unsafe.Sizeof(rl.Mesh{})))
}

func modelMaterial(m rl.Model, i int) *rl.Material {
	return (*rl.Material)(unsafe.Pointer(
		uintptr(unsafe.Pointer(m.Materials)) + uintptr(i)*unsafe.Sizeof(rl.Material{})))
}

// ── Render output ─────────────────────────────────────────────
func performRender() {
	rc := getActiveRenderCamera()
	if rc == nil {
		setStatus("No render camera - press C to add one")
		return
	}

	w, h := ui.renderW, ui.renderH
	if w < 1 || h < 1 {
		setStatus("Render size is invalid")
		return
	}

	// (Re)create texture if size changed or first use
	if renderOut.texture.ID == 0 || renderOut.texture.Texture.Width != w || renderOut.texture.Texture.Height != h {
		if renderOut.texture.ID != 0 {
			rl.UnloadRenderTexture(renderOut.texture)
		}
		renderOut.texture = rl.LoadRenderTexture(w, h)
		if renderOut.texture.ID == 0 || renderOut.texture.Texture.ID == 0 {
			renderOut.texture = rl.RenderTexture2D{}
			setStatus("Failed to create render target")
			return
		}
	}
	renderOut.width, renderOut.height = w, h

	renderCam := rl.Camera3D{
		Position:   rc.position,
		Target:     rc.target,
		Up:         rl.Vector3{Y: 1},
		Fovy:       rc.fovy,
		Projection: rl.CameraProjection(rl.CameraPerspective),
	}

	shaderUpdate(renderCam)

	rl.BeginTextureMode(renderOut.texture)
	rl.ClearBackground(cBG)
	rl.BeginMode3D(renderCam)
	drawSkybox()
	drawSceneGeometry(ui.shading, ui.wireOver, false)
	if ui.grid {
		drawGrid(20, 0.5)
	}
	rl.EndMode3D()
	rl.EndTextureMode()

	renderOut.rendered = true
	renderOut.showOutput = true
	setStatus("Render complete | %s | %dx%d | Esc to close", rc.name, w, h)
}

func saveRender() {
	if !renderOut.rendered {
		return
	}
	img := rl.LoadImageFromTexture(renderOut.texture.Texture)
	rl.ImageFlipVertical(img)
	rl.ExportImage(*img, "render_output.png")
	rl.UnloadImage(img)
	setStatus("Saved -> render_output.png")
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

// ── Scene geometry ────────────────────────────────────────────
func drawSceneGeometry(shading ShadingMode, wireOver bool, showSelection bool) {
	for i := range scene {
		o := &scene[i]
		if !o.loaded || !o.visible {
			continue
		}
		ns := o.scaleFactor * o.userScale
		// Offset mesh so its center sits at o.position
		offset := rl.Vector3Scale(o.center, -ns)
		worldPos := rl.Vector3Add(offset, o.position)
		rotAxis := rl.Vector3{Y: 1}
		scale := rl.Vector3{X: ns, Y: ns, Z: ns}

		switch shading {
		case ShadeSolid, ShadeMaterial:
			rl.DrawModelEx(o.model, worldPos, rotAxis, o.rotY, scale, rl.White)
		case ShadeWire:
			rl.DrawModelWiresEx(o.model, worldPos, rotAxis, o.rotY, scale,
				rl.Color{R: 78, G: 142, B: 166, A: 255})
		}
		if wireOver && shading != ShadeWire {
			rl.DrawModelWiresEx(o.model, worldPos, rotAxis, o.rotY, scale,
				rl.Color{R: 200, G: 130, B: 30, A: 36})
		}

		// Selection highlight
		if showSelection && ui.selectedType == ItemMesh && ui.selectedID == o.id {
			ext := rl.Vector3Scale(rl.Vector3Subtract(o.bounds.Max, o.bounds.Min), ns*0.5)
			bb := rl.BoundingBox{
				Min: rl.Vector3Subtract(o.position, ext),
				Max: rl.Vector3Add(o.position, ext),
			}
			rl.DrawBoundingBox(bb, rl.Color{R: cAccent.R, G: cAccent.G, B: cAccent.B, A: 180})
		}
	}

	// Draw render camera gizmos
	if showSelection {
		for i := range cameras {
			rc := &cameras[i]
			isSelected := ui.selectedType == ItemCamera && ui.selectedID == rc.id
			col := cGold
			if rc.active {
				col = cGreen
			}
			if isSelected {
				col = cAccent
			}
			// Camera body
			rl.DrawCubeWires(rc.position, 0.22, 0.16, 0.12, col)
			// View direction arrow
			dir := rl.Vector3Normalize(rl.Vector3Subtract(rc.target, rc.position))
			tip := rl.Vector3Add(rc.position, rl.Vector3Scale(dir, 0.45))
			rl.DrawLine3D(rc.position, tip, col)
			rl.DrawSphereWires(rc.position, 0.055, 4, 4, col)
		}
		for i := range lights {
			l := &lights[i]
			col := l.color
			if !l.enabled {
				col = cTextDim
			}
			if ui.selectedType == ItemLight && ui.selectedID == l.id {
				col = cAccent
			}
			rl.DrawSphere(l.position, 0.10, rl.Color{R: col.R, G: col.G, B: col.B, A: 180})
			rl.DrawSphereWires(l.position, 0.14, 6, 6, col)
			rl.DrawLine3D(l.position, rl.Vector3Add(l.position, rl.Vector3{Y: 0.5}), col)
		}
	}
}

// ── Navigation gizmo ─────────────────────────────────────────
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
	if ui.outliner {
		cx = float32(sw-pw) - gizmoSize - gizmoMargin
	}
	cy := float32(topbarH) + gizmoSize + gizmoMargin

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
			rl.DrawCircle(ex-1, ey-1, 1.5, rl.Color{R: 255, G: 255, B: 255, A: 60})
			tw := measure(a.label, fontSizeSm)
			txt(a.label, ex-tw/2, ey-fontSizeSm/2, fontSizeSm, rl.White)
		} else {
			rl.DrawCircle(ex, ey, 3.5, a.col)
		}
	}
}

// ── UI primitives ─────────────────────────────────────────────
func roundRect(x, y, w, h int32, r float32, c rl.Color) {
	rl.DrawRectangleRounded(
		rl.Rectangle{X: float32(x), Y: float32(y), Width: float32(w), Height: float32(h)},
		r, 6, c)
}

func roundRectLines(x, y, w, h int32, r float32, c rl.Color) {
	rl.DrawRectangleRoundedLines(
		rl.Rectangle{X: float32(x), Y: float32(y), Width: float32(w), Height: float32(h)},
		r, 6, c)
}

func btn(x, y, w, h int32, label string, active bool, accent rl.Color) bool {
	mp := rl.GetMousePosition()
	r := rl.Rectangle{X: float32(x), Y: float32(y), Width: float32(w), Height: float32(h)}
	hov := rl.CheckCollisionPointRec(mp, r)

	fill := cPanel
	if active {
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
	if active {
		roundRectLines(x, y, w, h, 0.35,
			rl.Color{R: accent.R, G: accent.G, B: accent.B, A: 80})
		rl.DrawRectangle(x+4, y+h-2, w-8, 2,
			rl.Color{R: accent.R, G: accent.G, B: accent.B, A: 200})
	} else if hov {
		roundRectLines(x, y, w, h, 0.35, cBorder)
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

func navBtn(x, y, w, h int32, label string, active bool, accent rl.Color) bool {
	mp := rl.GetMousePosition()
	r := rl.Rectangle{X: float32(x), Y: float32(y), Width: float32(w), Height: float32(h)}
	hov := rl.CheckCollisionPointRec(mp, r)

	if active {
		fill := rl.Color{
			R: uint8(clamp32(float32(accent.R)*0.18, 0, 255)),
			G: uint8(clamp32(float32(accent.G)*0.18, 0, 255)),
			B: uint8(clamp32(float32(accent.B)*0.18, 0, 255)),
			A: 255,
		}
		roundRect(x, y, w, h, 0.40, fill)
		roundRectLines(x, y, w, h, 0.40,
			rl.Color{R: accent.R, G: accent.G, B: accent.B, A: 100})
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
	rl.DrawRectangle(x, y+2, 2, h-4, cAccent)
	rl.DrawRectangle(x+2, y+4, 1, h-8,
		rl.Color{R: cAccent.R, G: cAccent.G, B: cAccent.B, A: 60})
	txt(title, x+14, y+(h-fontSize)/2, fontSize, cTextBrt)

	arrow := ">"
	if *open {
		arrow = "v"
	}
	aw := measure(arrow, fontSize)
	txt(arrow, x+w-aw-10, y+(h-fontSize)/2, fontSize,
		rl.Color{R: cAccent.R, G: cAccent.G, B: cAccent.B, A: 160})
	rl.DrawLine(x, y+h, x+w, y+h, cBorderLo)
	if hov && rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
		*open = !*open
	}
}

func labelRow(x, y int32, key, val string, valColor rl.Color) {
	txt(key, x+pad, y, fontSizeSm, cTextDim)
	txt(val, x+pad+98, y, fontSizeSm, valColor)
}

func slider(x, y, w int32, label string, val *float32, lo, hi float32) {
	if label != "" {
		txt(label, x, y, fontSizeSm, cTextDim)
		y += fontSizeSm + 4
	}
	slH := int32(4)
	roundRect(x, y, w, slH, 0.5, cBorderLo)
	t := (*val - lo) / (hi - lo)
	filled := int32(t * float32(w))
	if filled > 2 {
		roundRect(x, y, filled, slH, 0.5, cAccentDim)
	}
	hx := x + int32(t*float32(w-10))
	roundRect(hx, y-4, 10, slH+8, 0.5, cAccent)
	roundRect(hx+2, y-2, 6, 3, 0.5, rl.Color{R: 255, G: 255, B: 255, A: 40})
	mp := rl.GetMousePosition()
	hr := rl.Rectangle{X: float32(x), Y: float32(y - 5), Width: float32(w), Height: float32(slH + 10)}
	if rl.CheckCollisionPointRec(mp, hr) && rl.IsMouseButtonDown(rl.MouseButtonLeft) {
		*val = lo + clamp32((mp.X-float32(x))/float32(w), 0, 1)*(hi-lo)
	}
}

// ── Nav Toolbar ───────────────────────────────────────────────
func drawNavToolbar(vp rl.Rectangle) {
	const (
		bw  = navBtnW
		bh  = navBtnH
		gap = int32(2)
	)

	nBtns := 9
	totalW := int32(nBtns)*bw + int32(nBtns-1)*gap + 36
	totalH := bh + 14

	tx := int32(vp.X) + int32(vp.Width)/2 - totalW/2
	ty := int32(vp.Y) + int32(vp.Height) - totalH - 14

	rl.DrawRectangleRounded(
		rl.Rectangle{X: float32(tx - 2), Y: float32(ty - 2),
			Width: float32(totalW + 4), Height: float32(totalH + 4)},
		0.45, 8, rl.Color{R: 0, G: 0, B: 0, A: 60})
	rl.DrawRectangleRounded(
		rl.Rectangle{X: float32(tx), Y: float32(ty),
			Width: float32(totalW), Height: float32(totalH)},
		0.45, 8, cNavBg)
	rl.DrawRectangleRoundedLines(
		rl.Rectangle{X: float32(tx), Y: float32(ty),
			Width: float32(totalW), Height: float32(totalH)},
		0.45, 8, cBorder)
	rl.DrawRectangleRounded(
		rl.Rectangle{X: float32(tx + 6), Y: float32(ty),
			Width: float32(totalW - 12), Height: 1},
		0.5, 4, rl.Color{R: 255, G: 255, B: 255, A: 12})

	x := tx + 6
	by := ty + (totalH-bh)/2

	if navBtn(x, by, bw, bh, "Orbit", ui.navMode == NavOrbit, cAccent) {
		if ui.navMode == NavOrbit {
			ui.navMode = NavDefault
		} else {
			ui.navMode = NavOrbit
		}
	}
	x += bw + gap
	if navBtn(x, by, bw, bh, "Pan", ui.navMode == NavPan, cAccent) {
		if ui.navMode == NavPan {
			ui.navMode = NavDefault
		} else {
			ui.navMode = NavPan
		}
	}
	x += bw + gap + 4
	rl.DrawLine(x, by+4, x, by+bh-4, cBorder)
	x += 7

	if navBtn(x, by, bw, bh, "+  Zoom", false, cAccent) {
		cam.distance *= 0.82
		if cam.distance < 0.05 {
			cam.distance = 0.05
		}
	}
	x += bw + gap
	if navBtn(x, by, bw, bh, "- Zoom", false, cAccent) {
		cam.distance *= 1.20
	}
	x += bw + gap + 4
	rl.DrawLine(x, by+4, x, by+bh-4, cBorder)
	x += 7

	if navBtn(x, by, bw, bh, "Focus", false, cAccent) {
		camFocusSelected()
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
	rl.DrawLine(x, by+4, x, by+bh-4, cBorder)
	x += 7

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

// ── Outliner panel ────────────────────────────────────────────
func drawOutliner(sh int32) {
	x := int32(0)
	y := int32(topbarH)
	w := outlineW
	h := sh - topbarH - bottombarH

	rl.DrawRectangle(x, y, w, h, cPanelDark)
	rl.DrawLine(x+w-1, y, x+w-1, y+h, cBorder)

	// Header
	rl.DrawRectangle(x, y, w, btnH, cPanel)
	rl.DrawLine(x, y+btnH, x+w, y+btnH, cBorder)
	txt("Scene", x+10, y+(btnH-fontSize)/2, fontSize, cTextBrt)

	// Quick-add buttons (right side of header)
	bw2 := int32(26)
	bx := x + w - bw2*3 - 10
	if btn(bx, y+4, bw2, btnH-8, "+Obj", false, cAccent) {
		if p := openModelDialog(); p != "" {
			addModelToScene(p)
		}
	}
	bx += bw2 + 2
	if btn(bx, y+4, bw2, btnH-8, "+Cam", false, cGold) {
		addRenderCamera()
	}
	bx += bw2 + 2
	if btn(bx, y+4, bw2, btnH-8, "+Lit", false, cGreen) {
		addSceneLight()
	}

	y += btnH + 1
	iy := y

	// Mesh objects
	if len(scene) > 0 {
		txt("OBJECTS", x+10, iy+4, fontSizeSm-2, cTextDim)
		iy += fontSizeSm + 6
		for i := range scene {
			o := &scene[i]
			isSel := ui.selectedType == ItemMesh && ui.selectedID == o.id
			rowY := iy
			if isSel {
				roundRect(x, rowY, w-1, rowH, 0.15, cSelRow)
				rl.DrawLine(x, rowY, x, rowY+rowH, cAccent)
			}
			mp := rl.GetMousePosition()
			rr := rl.Rectangle{X: float32(x), Y: float32(rowY), Width: float32(w - 1), Height: float32(rowH)}
			hov := rl.CheckCollisionPointRec(mp, rr)
			if hov && !isSel {
				rl.DrawRectangleRec(rr, rl.Color{R: 50, G: 50, B: 50, A: 120})
			}
			if hov && rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
				ui.selectedID = o.id
				ui.selectedType = ItemMesh
			}

			// Visibility dot
			vc := cTextDim
			if o.visible {
				vc = cGreen
			}
			rl.DrawCircle(x+14, rowY+rowH/2, 4, vc)
			vr := rl.Rectangle{X: float32(x + 7), Y: float32(rowY + rowH/2 - 6), Width: 14, Height: 14}
			if rl.CheckCollisionPointRec(mp, vr) && rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
				o.visible = !o.visible
			}

			// Mesh icon ▣
			txt("M", x+24, rowY+(rowH-fontSizeSm)/2, fontSizeSm,
				rl.Color{R: cPurple.R, G: cPurple.G, B: cPurple.B, A: 180})

			// Name
			nameCol := cText
			if isSel {
				nameCol = cAccent
			}
			shortName := o.name
			if measure(shortName, fontSizeSm) > w-52 {
				for measure(shortName+"...", fontSizeSm) > w-52 && len(shortName) > 0 {
					shortName = shortName[:len(shortName)-1]
				}
				shortName += "..."
			}
			txt(shortName, x+40, rowY+(rowH-fontSizeSm)/2, fontSizeSm, nameCol)

			rl.DrawLine(x+8, rowY+rowH, x+w-8, rowY+rowH, cBorderLo)
			iy += rowH
		}
	}

	// Cameras
	if len(cameras) > 0 {
		iy += 4
		txt("CAMERAS", x+10, iy+4, fontSizeSm-2, cTextDim)
		iy += fontSizeSm + 6
		for i := range cameras {
			rc := &cameras[i]
			isSel := ui.selectedType == ItemCamera && ui.selectedID == rc.id
			rowY := iy
			if isSel {
				roundRect(x, rowY, w-1, rowH, 0.15, cSelRow)
				rl.DrawLine(x, rowY, x, rowY+rowH, cAccent)
			}
			mp := rl.GetMousePosition()
			rr := rl.Rectangle{X: float32(x), Y: float32(rowY), Width: float32(w - 1), Height: float32(rowH)}
			hov := rl.CheckCollisionPointRec(mp, rr)
			if hov && !isSel {
				rl.DrawRectangleRec(rr, rl.Color{R: 50, G: 50, B: 50, A: 120})
			}
			if hov && rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
				ui.selectedID = rc.id
				ui.selectedType = ItemCamera
			}

			// Active indicator
			ac := cTextDim
			if rc.active {
				ac = cGreen
			}
			rl.DrawCircle(x+14, rowY+rowH/2, 4, ac)

			// Camera icon ◎
			txt("C", x+24, rowY+(rowH-fontSizeSm)/2, fontSizeSm,
				rl.Color{R: cGold.R, G: cGold.G, B: cGold.B, A: 200})

			nameCol := cText
			if isSel {
				nameCol = cAccent
			}
			txt(rc.name, x+40, rowY+(rowH-fontSizeSm)/2, fontSizeSm, nameCol)

			// Set Active button on hover
			if hov && !rc.active {
				setW := measure("*", fontSizeSm)
				tx2 := x + w - setW - 10
				txt("*", tx2, rowY+(rowH-fontSizeSm)/2, fontSizeSm,
					rl.Color{R: cGreen.R, G: cGreen.G, B: cGreen.B, A: 100})
				sr := rl.Rectangle{X: float32(tx2 - 2), Y: float32(rowY + 2), Width: float32(setW + 4), Height: float32(rowH - 4)}
				if rl.CheckCollisionPointRec(mp, sr) && rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
					for j := range cameras {
						cameras[j].active = cameras[j].id == rc.id
					}
				}
			} else if rc.active {
				tw2 := measure("*", fontSizeSm)
				txt("*", x+w-tw2-10, rowY+(rowH-fontSizeSm)/2, fontSizeSm, cGreen)
			}

			rl.DrawLine(x+8, rowY+rowH, x+w-8, rowY+rowH, cBorderLo)
			iy += rowH
		}
	}

	if len(lights) > 0 {
		iy += 4
		txt("LIGHTS", x+10, iy+4, fontSizeSm-2, cTextDim)
		iy += fontSizeSm + 6
		for i := range lights {
			l := &lights[i]
			isSel := ui.selectedType == ItemLight && ui.selectedID == l.id
			rowY := iy
			if isSel {
				roundRect(x, rowY, w-1, rowH, 0.15, cSelRow)
				rl.DrawLine(x, rowY, x, rowY+rowH, cAccent)
			}
			mp := rl.GetMousePosition()
			rr := rl.Rectangle{X: float32(x), Y: float32(rowY), Width: float32(w - 1), Height: float32(rowH)}
			hov := rl.CheckCollisionPointRec(mp, rr)
			if hov && !isSel {
				rl.DrawRectangleRec(rr, rl.Color{R: 50, G: 50, B: 50, A: 120})
			}
			if hov && rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
				ui.selectedID = l.id
				ui.selectedType = ItemLight
			}

			vc := cTextDim
			if l.enabled {
				vc = cGreen
			}
			rl.DrawCircle(x+14, rowY+rowH/2, 4, vc)
			vr := rl.Rectangle{X: float32(x + 7), Y: float32(rowY + rowH/2 - 6), Width: 14, Height: 14}
			if rl.CheckCollisionPointRec(mp, vr) && rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
				l.enabled = !l.enabled
			}

			txt("L", x+24, rowY+(rowH-fontSizeSm)/2, fontSizeSm,
				rl.Color{R: cGreen.R, G: cGreen.G, B: cGreen.B, A: 200})
			nameCol := cText
			if isSel {
				nameCol = cAccent
			}
			txt(l.name, x+40, rowY+(rowH-fontSizeSm)/2, fontSizeSm, nameCol)
			rl.DrawLine(x+8, rowY+rowH, x+w-8, rowY+rowH, cBorderLo)
			iy += rowH
		}
	}

	// Empty scene hint
	if len(scene) == 0 && len(cameras) == 0 && len(lights) == 0 {
		iy += 12
		txtC("Drop a model here", x+w/2, iy, fontSizeSm, cTextDim)
		iy += fontSizeSm + 6
		txtC("or press O", x+w/2, iy, fontSizeSm, cTextDim)
	}
}

// ── Top bar ───────────────────────────────────────────────────
func drawTopBar(sw int32) {
	rl.DrawRectangle(0, 0, sw, topbarH, cPanel)
	rl.DrawRectangle(0, 0, sw, 1, cAccent)
	rl.DrawLine(0, topbarH-1, sw, topbarH-1, cBorder)

	// Brand
	txt("RENDER", pad, (topbarH-fontSizeXL)/2, fontSizeXL, cAccent)
	tw0 := measure("RENDER", fontSizeXL)
	txt("ZERO", pad+tw0+6, (topbarH-fontSizeXL)/2, fontSizeXL, cTextDim)
	tw1 := measure("ZERO", fontSizeXL)
	x := pad + tw0 + 6 + tw1 + 14

	rl.DrawLine(x, 8, x, topbarH-8, cBorder)
	x += 10

	// Open model
	if btn(x, 6, 52, topbarH-12, "Open", false, cAccent) {
		if p := openModelDialog(); p != "" {
			addModelToScene(p)
		}
	}
	x += 56

	// Load HDRI
	if btn(x, 6, 52, topbarH-12, "HDRI", hdri.loaded, cGold) {
		if p := openHDRIDialog(); p != "" {
			loadHDRI(p)
		}
	}
	x += 56

	// Add camera
	if btn(x, 6, 52, topbarH-12, "+Cam", false, cGold) {
		addRenderCamera()
	}
	x += 56

	if btn(x, 6, 56, topbarH-12, "+Light", false, cGreen) {
		addSceneLight()
	}
	x += 60

	if btn(x, 6, 58, topbarH-12, "LoadScn", false, cPurple) {
		loadScene()
	}
	x += 62

	if btn(x, 6, 58, topbarH-12, "SaveScn", false, cPurple) {
		saveScene()
	}
	x += 62

	rl.DrawLine(x, 8, x, topbarH-8, cBorder)
	x += 10

	// Shading
	for i, lbl := range []string{"Solid", "Wire", "Material"} {
		w2 := int32(56)
		if btn(x, 6, w2, topbarH-12, lbl, ui.shading == ShadingMode(i), cAccent) {
			ui.shading = ShadingMode(i)
		}
		x += w2 + 2
	}

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

	rl.DrawLine(x+2, 8, x+2, topbarH-8, cBorder)
	x += 14

	// Render button
	rc := getActiveRenderCamera()
	renderLabel := "Render"
	if rc != nil {
		renderLabel = "Render >"
	}
	renderCol := cGreen
	if rc == nil {
		renderCol = cTextDim
	}
	if btn(x, 6, 68, topbarH-12, renderLabel, false, renderCol) {
		performRender()
	}

	// Panel toggles (right-anchored)
	if btn(sw-42, 6, 34, topbarH-12, ">>", !ui.rightPanel, cAccent) {
		ui.rightPanel = !ui.rightPanel
	}
	if btn(sw-80, 6, 34, topbarH-12, "<<", !ui.outliner, cPurple) {
		ui.outliner = !ui.outliner
	}

	// Center: scene info pill
	if len(scene) > 0 || len(cameras) > 0 || len(lights) > 0 || hdri.loaded {
		info := fmt.Sprintf("%d objects", len(scene))
		if len(cameras) > 0 {
			info += fmt.Sprintf(" | %d cameras", len(cameras))
		}
		if len(lights) > 0 {
			info += fmt.Sprintf(" | %d lights", len(lights))
		}
		if hdri.loaded {
			info += " | HDR"
		}
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
	rl.DrawLine(px, py, px, py+ph, cBorder)

	y := py + 8

	// ── Object / Camera section ──────────────────────────────
	selObj := getSelectedObject()
	selCam := getSelectedCamera()
	selLight := getSelectedLight()

	sectionHeader(px, y, pw, "Object", &ui.secObject)
	y += btnH + 2
	if ui.secObject {
		if selObj != nil {
			labelRow(px, y, "Name", selObj.name, cText)
			y += fontSizeSm + 6
			labelRow(px, y, "Format", strings.ToUpper(selObj.ext), cText)
			y += fontSizeSm + 6
			labelRow(px, y, "Meshes", fmt.Sprintf("%d", selObj.meshCount), cAccent)
			y += fontSizeSm + 6
			labelRow(px, y, "Vertices", fmt.Sprintf("%d", selObj.vertexCount), cAccent)
			y += fontSizeSm + 6
			labelRow(px, y, "Triangles", fmt.Sprintf("%d", selObj.triCount), cAccent)
			y += fontSizeSm + 8

			// Visibility toggle
			bw2 := (pw - pad*3) / 2
			if btn(px+pad, y, bw2, btnH, "Visible", selObj.visible, cGreen) {
				selObj.visible = !selObj.visible
			}
			if btn(px+pad+bw2+pad, y, bw2, btnH, "Focus", false, cAccent) {
				camFocusSelected()
			}
			y += btnH + 4
		} else if selCam != nil {
			labelRow(px, y, "Name", selCam.name, cText)
			y += fontSizeSm + 6
			labelRow(px, y, "FoV", fmt.Sprintf("%.0f deg", selCam.fovy), cAccent)
			y += fontSizeSm + 6
			isActive := "No"
			if selCam.active {
				isActive = "Yes *"
			}
			labelRow(px, y, "Active", isActive, cGreen)
			y += fontSizeSm + 8
		} else if selLight != nil {
			labelRow(px, y, "Name", selLight.name, cText)
			y += fontSizeSm + 6
			state := "Off"
			if selLight.enabled {
				state = "On"
			}
			labelRow(px, y, "Enabled", state, cGreen)
			y += fontSizeSm + 6
			labelRow(px, y, "Intensity", fmt.Sprintf("%.2f", selLight.intensity), cAccent)
			y += fontSizeSm + 6
			labelRow(px, y, "Range", fmt.Sprintf("%.2f", selLight.rangeDist), cAccent)
			y += fontSizeSm + 8
			bw2 := (pw - pad*3) / 2
			if btn(px+pad, y, bw2, btnH, "Enabled", selLight.enabled, cGreen) {
				selLight.enabled = !selLight.enabled
			}
			if btn(px+pad+bw2+pad, y, bw2, btnH, "Focus", false, cAccent) {
				cam.target = selLight.position
			}
			y += btnH + 4
		} else {
			txt(fmt.Sprintf("%d objects | %d cameras | %d lights", len(scene), len(cameras), len(lights)),
				px+pad, y, fontSizeSm, cTextDim)
			y += fontSizeSm + 6
			if btn(px+pad, y, pw-pad*2, btnH, "Focus All  ( . )", false, cAccent) {
				camFocusAll()
			}
			y += btnH + 4
		}
		y += 2
	}
	rl.DrawLine(px, y, px+pw, y, cBorderLo)
	y += 6

	// ── Transform ─────────────────────────────────────────────
	sectionHeader(px, y, pw, "Transform", &ui.secTransform)
	y += btnH + 2
	if ui.secTransform {
		slw := pw - pad*2

		if selObj != nil {
			slider(px+pad, y, slw, "Position X", &selObj.position.X, -8, 8)
			y += fontSizeSm + 18
			slider(px+pad, y, slw, "Position Y", &selObj.position.Y, -8, 8)
			y += fontSizeSm + 18
			slider(px+pad, y, slw, "Position Z", &selObj.position.Z, -8, 8)
			y += fontSizeSm + 18
			slider(px+pad, y, slw, "Rotation Y", &selObj.rotY, -180, 180)
			y += fontSizeSm + 18
			slider(px+pad, y, slw, "Scale", &selObj.userScale, 0.05, 5)
			y += fontSizeSm + 16
			if btn(px+pad, y, (slw-4)/2, btnH, "Reset Pos", false, cPurple) {
				selObj.position = rl.Vector3{}
				selObj.rotY = 0
				selObj.userScale = 1
			}
			if btn(px+pad+(slw-4)/2+4, y, (slw-4)/2, btnH, "Focus", false, cAccent) {
				camFocusSelected()
			}
			y += btnH + 8
			txt("Arrows move X/Z | PgUp/PgDn move Y", px+pad, y, fontSizeSm, cTextDim)
			y += fontSizeSm + 4
			txt("[ ] rotate | - / = scale", px+pad, y, fontSizeSm, cTextDim)
			y += fontSizeSm + 8
		} else if selCam != nil {
			slider(px+pad, y, slw, "FoV", &selCam.fovy, 10, 120)
			y += fontSizeSm + 18
			labelRow(px, y, "Pos X", fmt.Sprintf("%.2f", selCam.position.X), cAccent)
			y += fontSizeSm + 6
			labelRow(px, y, "Pos Y", fmt.Sprintf("%.2f", selCam.position.Y), cAccent)
			y += fontSizeSm + 6
			labelRow(px, y, "Pos Z", fmt.Sprintf("%.2f", selCam.position.Z), cAccent)
			y += fontSizeSm + 8
			// Sync from viewport
			if btn(px+pad, y, pw-pad*2, btnH, "Sync from Viewport", false, cAccent) {
				c := camGet()
				selCam.position = c.Position
				selCam.target = c.Target
				setStatus("Camera synced from viewport")
			}
			y += btnH + 6
			bw2 := (pw - pad*3) / 2
			activeLabel := "Set Active"
			if selCam.active {
				activeLabel = "* Active"
			}
			if btn(px+pad, y, bw2, btnH, activeLabel, selCam.active, cGreen) {
				for j := range cameras {
					cameras[j].active = cameras[j].id == selCam.id
				}
			}
			if btn(px+pad+bw2+pad, y, bw2, btnH, "Render >", false, cGreen) {
				if !selCam.active {
					for j := range cameras {
						cameras[j].active = cameras[j].id == selCam.id
					}
				}
				performRender()
			}
			y += btnH + 8
		} else if selLight != nil {
			slider(px+pad, y, slw, "Position X", &selLight.position.X, -8, 8)
			y += fontSizeSm + 18
			slider(px+pad, y, slw, "Position Y", &selLight.position.Y, -8, 8)
			y += fontSizeSm + 18
			slider(px+pad, y, slw, "Position Z", &selLight.position.Z, -8, 8)
			y += fontSizeSm + 18
			if btn(px+pad, y, pw-pad*2, btnH, "Reset Position", false, cPurple) {
				selLight.position = rl.Vector3{}
			}
			y += btnH + 8
			txt("Arrows move light | PgUp/PgDn move Y", px+pad, y, fontSizeSm, cTextDim)
			y += fontSizeSm + 8
		} else {
			labelRow(px, y, "Az", fmt.Sprintf("%.1f deg", cam.azimuth*180/math.Pi), cAccent)
			y += fontSizeSm + 6
			labelRow(px, y, "El", fmt.Sprintf("%.1f deg", cam.elevation*180/math.Pi), cAccent)
			y += fontSizeSm + 6
			labelRow(px, y, "Dist", fmt.Sprintf("%.3f", cam.distance), cAccent)
			y += fontSizeSm + 8
		}
		y += 2
	}
	rl.DrawLine(px, y, px+pw, y, cBorderLo)
	y += 6

	// ── Display ───────────────────────────────────────────────
	sectionHeader(px, y, pw, "Display", &ui.secDisplay)
	y += btnH + 2
	if ui.secDisplay {
		sname := [...]string{"Solid", "Wireframe", "Material"}[ui.shading]
		labelRow(px, y, "Shading", sname, cText)
		y += fontSizeSm + 8
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

	// ── Lighting ──────────────────────────────────────────────
	sectionHeader(px, y, pw, "Lighting", &ui.secLighting)
	y += btnH + 2
	if ui.secLighting {
		slw := pw - pad*2
		if btn(px+pad, y, pw-pad*2, btnH, "Add Light", false, cGreen) {
			addSceneLight()
		}
		y += btnH + 6
		slider(px+pad, y, slw, "Light Azimuth", &ui.lightAz, -math.Pi, math.Pi)
		y += fontSizeSm + 20
		slider(px+pad, y, slw, "Light Elevation", &ui.lightEl, -math.Pi/2, math.Pi/2)
		y += fontSizeSm + 20
		slider(px+pad, y, slw, "Ambient", &ui.ambient, 0, 1)
		y += fontSizeSm + 20
		if selLight != nil {
			slider(px+pad, y, slw, "Intensity", &selLight.intensity, 0, 8)
			y += fontSizeSm + 20
			slider(px+pad, y, slw, "Range", &selLight.rangeDist, 0.5, 20)
			y += fontSizeSm + 20
			rv := float32(selLight.color.R) / 255.0
			gv := float32(selLight.color.G) / 255.0
			bv := float32(selLight.color.B) / 255.0
			slider(px+pad, y, slw, "Color R", &rv, 0, 1)
			selLight.color.R = uint8(clamp32(rv, 0, 1) * 255)
			y += fontSizeSm + 20
			slider(px+pad, y, slw, "Color G", &gv, 0, 1)
			selLight.color.G = uint8(clamp32(gv, 0, 1) * 255)
			y += fontSizeSm + 20
			slider(px+pad, y, slw, "Color B", &bv, 0, 1)
			selLight.color.B = uint8(clamp32(bv, 0, 1) * 255)
			selLight.color.A = 255
			y += fontSizeSm + 20
		}
	}
	rl.DrawLine(px, y, px+pw, y, cBorderLo)
	y += 6

	// ── Environment (HDRI) ────────────────────────────────────
	sectionHeader(px, y, pw, "Environment", &ui.secEnvironment)
	y += btnH + 2
	if ui.secEnvironment {
		slw := pw - pad*2
		if hdri.loaded {
			labelRow(px, y, "HDRI", hdri.name, cGold)
			y += fontSizeSm + 6
			slider(px+pad, y, slw, "Intensity", &hdri.intensity, 0, 4)
			y += fontSizeSm + 20
			slider(px+pad, y, slw, "Rotation", &hdri.skyRotation, -math.Pi, math.Pi)
			y += fontSizeSm + 20
			bw2 := (pw - pad*3) / 2
			if btn(px+pad, y, bw2, btnH, "IBL", hdri.useIBL, cGold) {
				hdri.useIBL = !hdri.useIBL
			}
			if btn(px+pad+bw2+pad, y, bw2, btnH, "Unload", false, cRed) {
				unloadHDRI()
				setStatus("Environment unloaded")
			}
			y += btnH + 8
		} else {
			txt("No environment loaded", px+pad, y, fontSizeSm, cTextDim)
			y += fontSizeSm + 8
			if btn(px+pad, y, pw-pad*2, btnH, "Load HDRI / Image  ( H )", false, cGold) {
				if p := openHDRIDialog(); p != "" {
					loadHDRI(p)
				}
			}
			y += btnH + 8
		}
	}
	rl.DrawLine(px, y, px+pw, y, cBorderLo)
	y += 6

	// ── Render ────────────────────────────────────────────────
	sectionHeader(px, y, pw, "Render", &ui.secRender)
	y += btnH + 2
	if ui.secRender {
		slw := pw - pad*2
		labelRow(px, y, "Resolution",
			fmt.Sprintf("%d x %d", ui.renderW, ui.renderH), cText)
		y += fontSizeSm + 8

		// Resolution presets
		bw2 := (slw - 4) / 3
		if btn(px+pad, y, bw2, btnH, "720p", ui.renderW == 1280, cPurple) {
			ui.renderW, ui.renderH = 1280, 720
		}
		if btn(px+pad+bw2+2, y, bw2, btnH, "1080p", ui.renderW == 1920, cPurple) {
			ui.renderW, ui.renderH = 1920, 1080
		}
		if btn(px+pad+bw2*2+4, y, bw2, btnH, "4K", ui.renderW == 3840, cPurple) {
			ui.renderW, ui.renderH = 3840, 2160
		}
		y += btnH + 8

		rc := getActiveRenderCamera()
		camLabel := "No camera active"
		if rc != nil {
			camLabel = rc.name
		}
		labelRow(px, y, "Camera", camLabel, cGold)
		y += fontSizeSm + 8

		if btn(px+pad, y, pw-pad*2, btnH, "Render  ( R )", false, cGreen) {
			performRender()
		}
		y += btnH + 4
		if renderOut.rendered {
			bw3 := (pw - pad*3) / 2
			if btn(px+pad, y, bw3, btnH, "View Output", renderOut.showOutput, cPurple) {
				renderOut.showOutput = true
			}
			if btn(px+pad+bw3+pad, y, bw3, btnH, "Save PNG", false, cAccent) {
				saveRender()
			}
			y += btnH + 4
		}
		y += 4
	}

	// ── Shortcuts ─────────────────────────────────────────────
	bot := py + ph - 132
	if bot > y {
		rl.DrawLine(px, bot, px+pw, bot, cBorderLo)
		bot += 8
		txt("Shortcuts", px+pad, bot, fontSizeSm, cTextDim)
		bot += fontSizeSm + 6

		type sc struct{ k, v string }
		shortcuts := []sc{
			{"O", "Open model"}, {"H", "Load HDRI"},
			{"C", "Add camera"}, {"L", "Add light"},
			{"Ctrl+S/L", "Save/Load scene"}, {"R", "Render"},
			{".", "Focus"}, {"Del", "Remove"},
			{"Z", "Cycle shading"}, {"G/A/W", "Grid/Axes/Wire"},
			{"Arrows/[ ]", "Move/Rotate sel"}, {"-/=", "Scale sel"},
			{"MMB", "Orbit"}, {"Shift+MMB", "Pan"},
		}
		for _, s := range shortcuts {
			kw := measure(s.k, fontSizeSm)
			txt(s.k, px+pad, bot, fontSizeSm, cAccentDim)
			txt(s.v, px+pad+kw+8, bot, fontSizeSm,
				rl.Color{R: cTextDim.R, G: cTextDim.G, B: cTextDim.B, A: 180})
			bot += fontSizeSm + 4
		}
	}
}

// ── Render output overlay ─────────────────────────────────────
func drawRenderOutput(sw, sh int32) {
	if !renderOut.showOutput || !renderOut.rendered {
		return
	}

	// Dim backdrop
	rl.DrawRectangle(0, 0, sw, sh, rl.Color{R: 0, G: 0, B: 0, A: 200})

	// Calculate preview size maintaining aspect ratio
	maxW := sw - 120
	maxH := sh - 120
	rw := renderOut.width
	rh := renderOut.height
	scaleX := float32(maxW) / float32(rw)
	scaleY := float32(maxH) / float32(rh)
	scale := scaleX
	if scaleY < scale {
		scale = scaleY
	}
	dispW := int32(float32(rw) * scale)
	dispH := int32(float32(rh) * scale)
	ox := (sw - dispW) / 2
	oy := (sh - dispH) / 2

	// Frame
	rl.DrawRectangle(ox-2, oy-2, dispW+4, dispH+4, cBorder)
	rl.DrawRectangle(ox-1, oy-1, dispW+2, dispH+2, cAccent)

	// Rendered texture (flip Y: OpenGL renders upside down)
	src := rl.Rectangle{
		X:      0,
		Y:      float32(rh),
		Width:  float32(rw),
		Height: -float32(rh),
	}
	dst := rl.Rectangle{
		X:      float32(ox),
		Y:      float32(oy),
		Width:  float32(dispW),
		Height: float32(dispH),
	}
	rl.DrawTexturePro(renderOut.texture.Texture, src, dst, rl.Vector2{}, 0, rl.White)

	// Header bar
	headerH := int32(34)
	rl.DrawRectangle(ox, oy-headerH-1, dispW, headerH, cPanel)
	roundRectLines(ox, oy-headerH-1, dispW, headerH, 0.2, cBorder)

	rc := getActiveRenderCamera()
	camName := "Render Output"
	if rc != nil {
		camName = rc.name + " | Render Output"
	}
	txt(camName, ox+10, oy-headerH-1+(headerH-fontSizeSm)/2, fontSizeSm, cTextBrt)

	// Buttons
	bx := ox + dispW - 4
	bx -= 70
	if btn(bx, oy-headerH-1+3, 66, headerH-6, "Save PNG", false, cAccent) {
		saveRender()
	}
	bx -= 58
	if btn(bx, oy-headerH-1+3, 54, headerH-6, "Close", false, cPurple) {
		renderOut.showOutput = false
	}

	// Resolution label
	resLabel := fmt.Sprintf("%d x %d", renderOut.width, renderOut.height)
	rw2 := measure(resLabel, fontSizeSm)
	txt(resLabel, ox+dispW-rw2-10-(70+58+8), oy-headerH-1+(headerH-fontSizeSm)/2,
		fontSizeSm, cTextDim)
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
	totalV, totalT := 0, 0
	for _, o := range scene {
		if o.loaded && o.visible {
			totalV += o.vertexCount
			totalT += o.triCount
		}
	}
	s := fmt.Sprintf("%s | %s | dist %.2f | %dv %dt",
		names[ui.view], proj, cam.distance, totalV, totalT)
	tw := measure(s, fontSizeSm)
	bx := int32(vp.X) + pad - 4
	by := int32(vp.Y) + pad - 3
	roundRect(bx, by, tw+12, fontSizeSm+8, 0.4,
		rl.Color{R: 28, G: 28, B: 28, A: 210})
	txt(s, bx+6, by+4, fontSizeSm, cTextDim)
}

// ── Bottom bar ────────────────────────────────────────────────
func drawBottomBar(sw, sh int32) {
	y := sh - bottombarH
	rl.DrawRectangle(0, y, sw, bottombarH, cPanel)
	rl.DrawLine(0, y, sw, y, cBorder)
	rl.DrawRectangle(0, sh-1, sw, 1, cPurpleDim)

	if ui.statusTimer > 0 {
		tc := cText
		if ui.statusTimer < 1.0 {
			tc.A = uint8(ui.statusTimer * 255)
		}
		txt(ui.statusMsg, pad, y+(bottombarH-fontSizeSm)/2, fontSizeSm, tc)
	} else {
		hint := "Drop model or HDRI | O = model | H = HDRI | C = camera | L = light | Ctrl+S/L = scene save/load"
		if len(scene) > 0 {
			hint = fmt.Sprintf("%d objects | %d cameras | %d lights | R = render from active camera",
				len(scene), len(cameras), len(lights))
		}
		txt(hint, pad, y+(bottombarH-fontSizeSm)/2, fontSizeSm, cTextDim)
	}

	rightX := sw - pad
	fps := fmt.Sprintf("%d fps", rl.GetFPS())
	tw := measure(fps, fontSizeSm)
	txt(fps, rightX-tw, y+(bottombarH-fontSizeSm)/2, fontSizeSm, cTextDim)
	rightX -= tw + 16

	if ui.navMode != NavDefault {
		modeStr := "Orbit mode"
		if ui.navMode == NavPan {
			modeStr = "Pan mode"
		}
		mw := measure(modeStr, fontSizeSm)
		txt(modeStr, rightX-mw, y+(bottombarH-fontSizeSm)/2, fontSizeSm, cAccent)
		rightX -= mw + 16
	}

	if hdri.loaded {
		s := "HDR: " + hdri.name
		sw2 := measure(s, fontSizeSm)
		txt(s, rightX-sw2, y+(bottombarH-fontSizeSm)/2, fontSizeSm,
			rl.Color{R: cGold.R, G: cGold.G, B: cGold.B, A: 160})
	}
}

// ── Input ─────────────────────────────────────────────────────
func handleInput() {
	ctrl := rl.IsKeyDown(rl.KeyLeftControl) || rl.IsKeyDown(rl.KeyRightControl)

	// Escape: close render output / cancel nav
	if rl.IsKeyPressed(rl.KeyEscape) {
		if renderOut.showOutput {
			renderOut.showOutput = false
		} else if ui.navMode != NavDefault {
			ui.navMode = NavDefault
		}
	}

	// Open model
	if rl.IsKeyPressed(rl.KeyO) {
		if p := openModelDialog(); p != "" {
			addModelToScene(p)
		}
	}

	if ctrl && rl.IsKeyPressed(rl.KeyS) {
		saveScene()
	}
	if ctrl && rl.IsKeyPressed(rl.KeyL) {
		loadScene()
	}

	// Load HDRI
	if rl.IsKeyPressed(rl.KeyH) {
		if p := openHDRIDialog(); p != "" {
			loadHDRI(p)
		}
	}

	// Add camera
	if rl.IsKeyPressed(rl.KeyC) {
		addRenderCamera()
	}
	if rl.IsKeyPressed(rl.KeyL) && !ctrl {
		addSceneLight()
	}

	// Render
	if rl.IsKeyPressed(rl.KeyR) {
		performRender()
	}

	// Focus
	if rl.IsKeyPressed(rl.KeyPeriod) {
		camFocusSelected()
	}

	// Shading
	if rl.IsKeyPressed(rl.KeyZ) {
		ui.shading = (ui.shading + 1) % ShadeCount
	}

	// Overlays
	if rl.IsKeyPressed(rl.KeyG) {
		ui.grid = !ui.grid
	}
	if rl.IsKeyPressed(rl.KeyW) {
		ui.wireOver = !ui.wireOver
	}
	if rl.IsKeyPressed(rl.KeyA) {
		ui.axes = !ui.axes
	}

	// Delete selected
	if rl.IsKeyPressed(rl.KeyDelete) {
		removeSelected()
	}

	// Selected object transforms
	if selObj := getSelectedObject(); selObj != nil {
		moveStep := float32(0.04)
		rotStep := float32(1.5)
		scaleStep := float32(0.02)
		if rl.IsKeyDown(rl.KeyLeftShift) || rl.IsKeyDown(rl.KeyRightShift) {
			moveStep = 0.12
			rotStep = 4.0
			scaleStep = 0.05
		}

		if rl.IsKeyDown(rl.KeyLeft) {
			selObj.position.X -= moveStep
		}
		if rl.IsKeyDown(rl.KeyRight) {
			selObj.position.X += moveStep
		}
		if rl.IsKeyDown(rl.KeyUp) {
			selObj.position.Z -= moveStep
		}
		if rl.IsKeyDown(rl.KeyDown) {
			selObj.position.Z += moveStep
		}
		if rl.IsKeyDown(rl.KeyPageUp) {
			selObj.position.Y += moveStep
		}
		if rl.IsKeyDown(rl.KeyPageDown) {
			selObj.position.Y -= moveStep
		}
		if rl.IsKeyDown(rl.KeyLeftBracket) {
			selObj.rotY -= rotStep
		}
		if rl.IsKeyDown(rl.KeyRightBracket) {
			selObj.rotY += rotStep
		}
		if rl.IsKeyDown(rl.KeyMinus) {
			selObj.userScale = float32(math.Max(float64(selObj.userScale-scaleStep), 0.05))
		}
		if rl.IsKeyDown(rl.KeyEqual) {
			selObj.userScale = float32(math.Min(float64(selObj.userScale+scaleStep), 5.0))
		}
	} else if selLight := getSelectedLight(); selLight != nil {
		moveStep := float32(0.05)
		if rl.IsKeyDown(rl.KeyLeftShift) || rl.IsKeyDown(rl.KeyRightShift) {
			moveStep = 0.12
		}
		if rl.IsKeyDown(rl.KeyLeft) {
			selLight.position.X -= moveStep
		}
		if rl.IsKeyDown(rl.KeyRight) {
			selLight.position.X += moveStep
		}
		if rl.IsKeyDown(rl.KeyUp) {
			selLight.position.Z -= moveStep
		}
		if rl.IsKeyDown(rl.KeyDown) {
			selLight.position.Z += moveStep
		}
		if rl.IsKeyDown(rl.KeyPageUp) {
			selLight.position.Y += moveStep
		}
		if rl.IsKeyDown(rl.KeyPageDown) {
			selLight.position.Y -= moveStep
		}
	}

	// Numpad view presets
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

	// Drag & drop — models go to scene, HDR goes to environment
	if rl.IsFileDropped() {
		files := rl.LoadDroppedFiles()
		for _, f := range files {
			ext := strings.ToLower(filepath.Ext(f))
			if ext == ".hdr" || ext == ".exr" {
				loadHDRI(f)
			} else {
				addModelToScene(f)
			}
		}
		rl.UnloadDroppedFiles()
	}

	// Status timer
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
	rl.InitWindow(1600, 920, "Render Zero")
	rl.SetTargetFPS(144)
	rl.SetExitKey(0)

	camInit()
	shaderInit()
	hdriInit()
	initFont()

	ui = UIState{
		outliner:       true,
		rightPanel:     true,
		selectedID:     -1,
		shading:        ShadeSolid,
		view:           ViewPersp,
		navMode:        NavDefault,
		grid:           true,
		axes:           true,
		stats:          true,
		secOutliner:    true,
		secObject:      true,
		secTransform:   true,
		secCamera:      true,
		secDisplay:     true,
		secLighting:    true,
		secEnvironment: true,
		secRender:      true,
		lightAz:        -0.8,
		lightEl:        0.9,
		ambient:        0.18,
		renderW:        1280,
		renderH:        720,
	}
	setStatus("Welcome - drag and drop models / HDRI, press O for models, H for environment")

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

		// 3D scene
		rl.BeginMode3D(c)
		drawSkybox()
		if ui.grid {
			drawGrid(20, 0.5)
		}
		if ui.axes {
			rl.DrawLine3D(rl.Vector3{X: -5}, rl.Vector3{X: 5}, cAxisX)
			rl.DrawLine3D(rl.Vector3{Y: -5}, rl.Vector3{Y: 5}, cAxisY)
			rl.DrawLine3D(rl.Vector3{Z: -5}, rl.Vector3{Z: 5}, cAxisZ)
		}
		drawSceneGeometry(ui.shading, ui.wireOver, true)
		rl.EndMode3D()

		// 2D overlays
		drawStats(vp)
		drawGizmo(sw, sh)
		drawNavToolbar(vp)
		drawTopBar(sw)
		if ui.outliner {
			drawOutliner(sh)
		}
		if ui.rightPanel {
			drawRightPanel(sw, sh)
		}
		drawBottomBar(sw, sh)

		// Render output modal (drawn last, on top)
		drawRenderOutput(sw, sh)

		rl.EndDrawing()
	}

	// Cleanup
	for i := range scene {
		if scene[i].loaded {
			rl.UnloadModel(scene[i].model)
		}
	}
	if hdri.loaded {
		unloadHDRI()
	}
	rl.UnloadShader(hdri.skyboxShader)
	if renderOut.texture.ID != 0 {
		rl.UnloadRenderTexture(renderOut.texture)
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
