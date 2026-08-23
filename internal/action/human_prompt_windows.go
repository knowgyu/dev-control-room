//go:build windows

package action

import (
	"context"
	"fmt"
	"syscall"
	"unsafe"
)

const (
	messageBoxYesNoCancel = 0x00000003 | 0x00000020
	messageBoxYes         = 6
	messageBoxNo          = 7
)

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	messageBox       = user32.NewProc("MessageBoxW")
	openInputDesktop = user32.NewProc("OpenInputDesktop")
	closeDesktop     = user32.NewProc("CloseDesktop")
)

type windowsHumanDecisionPrompt struct{}

func newHumanDecisionPrompt() HumanDecisionPrompt { return windowsHumanDecisionPrompt{} }

func (windowsHumanDecisionPrompt) Decide(ctx context.Context, request HumanDecisionRequest) (HumanDecision, error) {
	if err := ctx.Err(); err != nil {
		return HumanDecisionCancel, err
	}
	desktop, _, desktopErr := openInputDesktop.Call(0, 0, 1)
	if desktop == 0 {
		return HumanDecisionCancel, fmt.Errorf("%w: %v", ErrHumanDecisionUnavailable, desktopErr)
	}
	defer closeDesktop.Call(desktop)
	text := fmt.Sprintf("Approve this action?\n\nPlan: %s\nDigest: %s\nWorktree: %s\nExecutable: %s\nExpires: %s\n\nYes grants approval. No rejects it. Cancel makes no decision.", request.Plan, request.Digest, request.Worktree, request.Executable, request.ExpiresAt.Local().Format("2006-01-02 15:04:05 MST"))
	text16, err := syscall.UTF16PtrFromString(text)
	if err != nil {
		return HumanDecisionCancel, err
	}
	title16, err := syscall.UTF16PtrFromString("Dev Control Room approval")
	if err != nil {
		return HumanDecisionCancel, err
	}
	result, _, callErr := messageBox.Call(0, uintptr(unsafe.Pointer(text16)), uintptr(unsafe.Pointer(title16)), messageBoxYesNoCancel)
	switch result {
	case messageBoxYes:
		return HumanDecisionGrant, nil
	case messageBoxNo:
		return HumanDecisionReject, nil
	case 0:
		return HumanDecisionCancel, callErr
	default:
		return HumanDecisionCancel, nil
	}
}
