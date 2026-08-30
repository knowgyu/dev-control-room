//go:build windows

package folderpicker

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

// The legacy shell tree dialog looks unlike the rest of Windows Explorer and
// has awkward keyboard behavior. IFileOpenDialog is the Explorer-backed native
// picker used by current Windows applications; FOS_PICKFOLDERS changes its
// file-open surface into a folder chooser without introducing a custom UI.
const (
	clsctxInprocServer             = 1
	coinitApartmentThreaded        = 2
	fosPickFolders          uint32 = 0x20
	fosForceFileSystem      uint32 = 0x40
	fosPathMustExist        uint32 = 0x800
	sigdnFilePath           uint32 = 0x80058000
	hresultOK                      = 0
	hresultSFalse                  = 1
	hresultCancelled        int32  = -2147023673 // 0x800704C7, ERROR_CANCELLED
)

// IFileOpenDialog inherits IFileDialog, which in turn inherits IModalWindow
// and IUnknown. These are COM vtable slots, kept named so changes remain easy
// to review against the Windows SDK interface definition.
const (
	fileDialogSetOptionsSlot    = 9
	fileDialogSetTitleSlot      = 17
	fileDialogSetOKButtonSlot   = 18
	fileDialogGetResultSlot     = 20
	shellItemGetDisplayNameSlot = 5
	comReleaseSlot              = 2
)

type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

var (
	ole32            = syscall.NewLazyDLL("ole32.dll")
	coCreateInstance = ole32.NewProc("CoCreateInstance")
	coInitializeEx   = ole32.NewProc("CoInitializeEx")
	coUninitialize   = ole32.NewProc("CoUninitialize")
	coTaskMemFree    = ole32.NewProc("CoTaskMemFree")

	clsidFileOpenDialog = guid{0xdc1c5a9c, 0xe88a, 0x4dde, [8]byte{0xa5, 0xa1, 0x60, 0xf8, 0x2a, 0x20, 0xae, 0xf7}}
	iidFileOpenDialog   = guid{0xd57c7288, 0xd4ad, 0x4768, [8]byte{0xbe, 0x02, 0x9d, 0x96, 0x95, 0x32, 0xd9, 0x60}}
)

func pick() (string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if result, _, _ := coInitializeEx.Call(0, coinitApartmentThreaded); result != hresultOK && result != hresultSFalse {
		return "", hresultError(result)
	}
	defer coUninitialize.Call()

	var dialog unsafe.Pointer
	result, _, _ := coCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidFileOpenDialog)),
		0,
		clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidFileOpenDialog)),
		uintptr(unsafe.Pointer(&dialog)),
	)
	if err := checkHRESULT(result); err != nil {
		return "", err
	}
	if dialog == nil {
		return "", fmt.Errorf("native folder picker returned an empty dialog")
	}
	defer comRelease(dialog)

	if err := fileDialogSetOptions(dialog, fosPickFolders|fosForceFileSystem|fosPathMustExist); err != nil {
		return "", err
	}
	title, err := syscall.UTF16PtrFromString("저장소 폴더 선택")
	if err != nil {
		return "", err
	}
	if err := fileDialogSetString(dialog, fileDialogSetTitleSlot, title); err != nil {
		return "", err
	}
	okLabel, err := syscall.UTF16PtrFromString("이 폴더 선택")
	if err != nil {
		return "", err
	}
	if err := fileDialogSetString(dialog, fileDialogSetOKButtonSlot, okLabel); err != nil {
		return "", err
	}

	// A nil owner lets Windows place the native Explorer dialog normally even
	// though the server itself is a console process without a window handle.
	vtable := *(**[21]uintptr)(dialog)
	result, _, _ = syscall.SyscallN(vtable[3], uintptr(dialog), 0)
	if int32(result) == hresultCancelled {
		return "", ErrCancelled
	}
	if err := checkHRESULT(result); err != nil {
		return "", err
	}

	var item unsafe.Pointer
	result, _, _ = syscall.SyscallN(vtable[fileDialogGetResultSlot], uintptr(dialog), uintptr(unsafe.Pointer(&item)))
	if err := checkHRESULT(result); err != nil {
		return "", err
	}
	if item == nil {
		return "", ErrCancelled
	}
	defer comRelease(item)

	itemVtable := *(**[6]uintptr)(item)
	var pathPointer *uint16
	result, _, _ = syscall.SyscallN(
		itemVtable[shellItemGetDisplayNameSlot],
		uintptr(item),
		uintptr(sigdnFilePath),
		uintptr(unsafe.Pointer(&pathPointer)),
	)
	if err := checkHRESULT(result); err != nil {
		return "", err
	}
	if pathPointer == nil {
		return "", ErrCancelled
	}
	defer coTaskMemFree.Call(uintptr(unsafe.Pointer(pathPointer)))

	path := syscall.UTF16ToString((*[1 << 20]uint16)(unsafe.Pointer(pathPointer))[:])
	if path == "" {
		return "", ErrCancelled
	}
	return path, nil
}

func fileDialogSetOptions(dialog unsafe.Pointer, options uint32) error {
	vtable := *(**[10]uintptr)(dialog)
	result, _, _ := syscall.SyscallN(vtable[fileDialogSetOptionsSlot], uintptr(dialog), uintptr(options))
	return checkHRESULT(result)
}

func fileDialogSetString(dialog unsafe.Pointer, slot int, value *uint16) error {
	vtable := *(**[19]uintptr)(dialog)
	result, _, _ := syscall.SyscallN(vtable[slot], uintptr(dialog), uintptr(unsafe.Pointer(value)))
	return checkHRESULT(result)
}

func comRelease(object unsafe.Pointer) {
	if object == nil {
		return
	}
	vtable := *(**[3]uintptr)(object)
	syscall.SyscallN(vtable[comReleaseSlot], uintptr(object))
}

func checkHRESULT(result uintptr) error {
	if int32(result) == hresultOK {
		return nil
	}
	return hresultError(result)
}

type hresultErrorValue int32

func (e hresultErrorValue) Error() string {
	return fmt.Sprintf("native Windows folder picker failed (HRESULT 0x%08X)", uint32(int32(e)))
}

func hresultError(result uintptr) error { return hresultErrorValue(int32(result)) }
