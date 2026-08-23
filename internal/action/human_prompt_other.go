//go:build !windows

package action

import "context"

type unavailableHumanDecisionPrompt struct{}

func newHumanDecisionPrompt() HumanDecisionPrompt { return unavailableHumanDecisionPrompt{} }

func (unavailableHumanDecisionPrompt) Decide(_ context.Context, _ HumanDecisionRequest) (HumanDecision, error) {
	return HumanDecisionCancel, ErrHumanDecisionUnavailable
}
