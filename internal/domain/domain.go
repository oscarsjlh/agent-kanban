package domain

import "fmt"

const (
	Inbox      = "Inbox"
	Ready      = "Ready"
	InProgress = "In Progress"
	Waiting    = "Waiting"
	Done       = "Done"
	Wontfix    = "wontfix"
)

var validColumns = map[string]string{
	"inbox": Inbox, "ready": Ready, "in-progress": InProgress, "in_progress": InProgress, "waiting": Waiting, "done": Done, "wontfix": Wontfix,
	Inbox: Inbox, Ready: Ready, InProgress: InProgress, Waiting: Waiting, Done: Done,
}

func CanonicalColumn(s string) (string, bool) { c, ok := validColumns[s]; return c, ok }

func CanMove(from, to string, hasWaitingReason bool, claimed bool) error {
	if to == Waiting && !hasWaitingReason {
		return fmt.Errorf("moving to Waiting requires --reason or --blocked-by")
	}
	if from == Done || from == Wontfix {
		return fmt.Errorf("terminal issue cannot be moved")
	}
	if from == InProgress && claimed {
		return fmt.Errorf("claimed issue cannot be moved manually; stop the claim first")
	}
	switch to {
	case Ready, Waiting, Done, Wontfix:
		return nil
	}
	return fmt.Errorf("illegal move to %s", to)
}

func CanClaim(claimedBy string) error {
	if claimedBy != "" {
		return fmt.Errorf("issue is already claimed by %s", claimedBy)
	}
	return nil
}

func CanRelease(current, caller string) error {
	if current == "" {
		return fmt.Errorf("issue is not claimed")
	}
	if current != caller {
		return fmt.Errorf("issue is claimed by %s, not %s", current, caller)
	}
	return nil
}
