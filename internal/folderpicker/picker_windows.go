//go:build windows

package folderpicker

import (
	"syscall"
	"unsafe"
)

const (
	bifReturnOnlyFileSystemDirs = 0x0001
	bifEditBox                  = 0x0010
	bifNewDialogStyle           = 0x0040
)

type browseInfo struct {
	hwndOwner      uintptr
	pidlRoot       uintptr
	pszDisplayName *uint16
	lpszTitle      *uint16
	ulFlags        uint32
	lpfn           uintptr
	lParam         uintptr
	iImage         int32
}

var (
	shell32           = syscall.NewLazyDLL("shell32.dll")
	shBrowseForFolder = shell32.NewProc("SHBrowseForFolderW")
	shGetPathFromID   = shell32.NewProc("SHGetPathFromIDListW")
	ole32             = syscall.NewLazyDLL("ole32.dll")
	coTaskMemFree     = ole32.NewProc("CoTaskMemFree")
)

func pick() (string, error) {
	title, err := syscall.UTF16PtrFromString("Select a folder to scan for Git repositories")
	if err != nil {
		return "", err
	}
	display := make([]uint16, syscall.MAX_PATH)
	info := browseInfo{
		pszDisplayName: &display[0],
		lpszTitle:      title,
		ulFlags:        bifReturnOnlyFileSystemDirs | bifEditBox | bifNewDialogStyle,
	}
	pidl, _, _ := shBrowseForFolder.Call(uintptr(unsafe.Pointer(&info)))
	if pidl == 0 {
		return "", ErrCancelled
	}
	defer coTaskMemFree.Call(pidl)
	if ok, _, _ := shGetPathFromID.Call(pidl, uintptr(unsafe.Pointer(&display[0]))); ok == 0 {
		return "", ErrCancelled
	}
	path := syscall.UTF16ToString(display)
	if path == "" {
		return "", ErrCancelled
	}
	return path, nil
}
