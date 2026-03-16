//go:build windows

package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	comdlg32        = windows.NewLazySystemDLL("comdlg32.dll")
	getOpenFileName = comdlg32.NewProc("GetOpenFileNameW")
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
	ofnFileMustExist = 0x00001000
	ofnPathMustExist = 0x00000800
	ofnNoChangeDir   = 0x00000008
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

// openFileDialog opens the Windows "Open File" common dialog and
// returns the selected path, or "" if the user cancelled.
func openFileDialog() string {
	filter := utf16Filter(
		"3D Models (*.obj;*.fbx;*.gltf;*.glb)", "*.obj;*.fbx;*.gltf;*.glb",
		"OBJ Files (*.obj)", "*.obj",
		"FBX Files (*.fbx)", "*.fbx",
		"GLTF / GLB Files (*.gltf;*.glb)", "*.gltf;*.glb",
		"All Files (*.*)", "*.*",
	)

	title, _ := windows.UTF16PtrFromString("Open 3D Model")

	buf := make([]uint16, 512)

	ofn := openFileNameW{
		lStructSize:  uint32(unsafe.Sizeof(openFileNameW{})),
		lpstrFilter:  &filter[0],
		lpstrFile:    &buf[0],
		nMaxFile:     uint32(len(buf)),
		lpstrTitle:   title,
		flags:        ofnFileMustExist | ofnPathMustExist | ofnNoChangeDir,
	}

	ret, _, _ := getOpenFileName.Call(uintptr(unsafe.Pointer(&ofn)))
	if ret == 0 {
		return ""
	}
	return windows.UTF16ToString(buf)
}
