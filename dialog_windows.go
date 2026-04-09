//go:build windows

package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	comdlg32        = windows.NewLazySystemDLL("comdlg32.dll")
	getOpenFileName = comdlg32.NewProc("GetOpenFileNameW")
	getSaveFileName = comdlg32.NewProc("GetSaveFileNameW")
)

// OPENFILENAMEW matches the Win32 OPENFILENAMEW layout exactly.
type openFileNameW struct {
	lStructSize       uint32
	hwndOwner         uintptr
	hInstance         uintptr
	lpstrFilter       *uint16
	lpstrCustomFilter *uint16
	nMaxCustFilter    uint32
	nFilterIndex      uint32
	lpstrFile         *uint16
	nMaxFile          uint32
	lpstrFileTitle    *uint16
	nMaxFileTitle     uint32
	lpstrInitialDir   *uint16
	lpstrTitle        *uint16
	flags             uint32
	nFileOffset       uint16
	nFileExtension    uint16
	lpstrDefExt       *uint16
	lCustData         uintptr
	lpfnHook          uintptr
	lpTemplateName    *uint16
}

const (
	ofnFileMustExist   = 0x00001000
	ofnPathMustExist   = 0x00000800
	ofnNoChangeDir     = 0x00000008
	ofnOverwritePrompt = 0x00000002
)

// utf16Filter builds a double-null-terminated UTF-16 filter string
// from pairs of (description, pattern) strings.
func utf16Filter(pairs ...string) []uint16 {
	var out []uint16
	for _, s := range pairs {
		for _, r := range s {
			out = append(out, uint16(r))
		}
		out = append(out, 0) // null terminator for each string
	}
	out = append(out, 0) // final double-null
	return out
}

func openDialog(titleText string, filter []uint16) string {
	title, _ := windows.UTF16PtrFromString(titleText)

	buf := make([]uint16, 512)

	ofn := openFileNameW{
		lStructSize: uint32(unsafe.Sizeof(openFileNameW{})),
		lpstrFilter: &filter[0],
		lpstrFile:   &buf[0],
		nMaxFile:    uint32(len(buf)),
		lpstrTitle:  title,
		flags:       ofnFileMustExist | ofnPathMustExist | ofnNoChangeDir,
	}

	ret, _, _ := getOpenFileName.Call(uintptr(unsafe.Pointer(&ofn)))
	if ret == 0 {
		return ""
	}
	return windows.UTF16ToString(buf)
}

func saveDialog(titleText string, filter []uint16, defExt string) string {
	title, _ := windows.UTF16PtrFromString(titleText)
	buf := make([]uint16, 512)
	defExtPtr, _ := windows.UTF16PtrFromString(defExt)

	ofn := openFileNameW{
		lStructSize: uint32(unsafe.Sizeof(openFileNameW{})),
		lpstrFilter: &filter[0],
		lpstrFile:   &buf[0],
		nMaxFile:    uint32(len(buf)),
		lpstrTitle:  title,
		lpstrDefExt: defExtPtr,
		flags:       ofnPathMustExist | ofnNoChangeDir | ofnOverwritePrompt,
	}

	ret, _, _ := getSaveFileName.Call(uintptr(unsafe.Pointer(&ofn)))
	if ret == 0 {
		return ""
	}
	return windows.UTF16ToString(buf)
}

func openModelDialog() string {
	filter := utf16Filter(
		"3D Models (*.obj;*.fbx;*.gltf;*.glb;*.iqm;*.m3d)", "*.obj;*.fbx;*.gltf;*.glb;*.iqm;*.m3d",
		"OBJ Files (*.obj)", "*.obj",
		"FBX Files (*.fbx)", "*.fbx",
		"GLTF / GLB Files (*.gltf;*.glb)", "*.gltf;*.glb",
		"IQM Files (*.iqm)", "*.iqm",
		"M3D Files (*.m3d)", "*.m3d",
		"All Files (*.*)", "*.*",
	)
	return openDialog("Open 3D Model", filter)
}

func openHDRIDialog() string {
	filter := utf16Filter(
		"HDRI / Images (*.hdr;*.exr;*.png;*.jpg;*.jpeg;*.tga)", "*.hdr;*.exr;*.png;*.jpg;*.jpeg;*.tga",
		"HDR Files (*.hdr;*.exr)", "*.hdr;*.exr",
		"Image Files (*.png;*.jpg;*.jpeg;*.tga)", "*.png;*.jpg;*.jpeg;*.tga",
		"All Files (*.*)", "*.*",
	)
	return openDialog("Open HDRI / Image", filter)
}

func openSceneDialog() string {
	filter := utf16Filter(
		"Render Zero Scene (*.rzs)", "*.rzs",
		"JSON Files (*.json)", "*.json",
		"All Files (*.*)", "*.*",
	)
	return openDialog("Load Scene", filter)
}

func saveSceneDialog() string {
	filter := utf16Filter(
		"Render Zero Scene (*.rzs)", "*.rzs",
		"JSON Files (*.json)", "*.json",
		"All Files (*.*)", "*.*",
	)
	return saveDialog("Save Scene", filter, "rzs")
}
