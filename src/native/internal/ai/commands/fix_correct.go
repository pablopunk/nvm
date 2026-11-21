package commands

import "strings"

// Asker sends one proofread request and returns the model's reply. The system prompt and the user message travel
// apart, so the instruction never reads as part of the data.
type Asker func(system, user string) (string, error)

// CorrectionAttempts is how many replies a piece gets: the first, and one retry that says the answer was too far off.
const CorrectionAttempts = 2

// CorrectText corrects text piece by piece and returns the joined result. progress is told after each piece with the
// number done and the number of pieces. A piece whose replies are all refused fails the whole correction, so a
// selection is either fully corrected or left alone.
func CorrectText(text string, ask Asker, progress func(done, total int)) (string, error) {
	pieces := SplitForCorrection(text, MaxChunkRunes)
	var out strings.Builder
	for index, piece := range pieces {
		corrected, err := correctPiece(piece, ask)
		if err != nil {
			return "", err
		}
		out.WriteString(corrected)
		if progress != nil {
			progress(index+1, len(pieces))
		}
	}
	return out.String(), nil
}

// correctPiece asks for a correction of one piece until a reply passes the guard, or the attempts run out. Whitespace
// only pieces need no reply.
func correctPiece(piece string, ask Asker) (string, error) {
	if strings.TrimSpace(piece) == "" {
		return piece, nil
	}
	var refusal error = ErrCorrectionRefused
	for attempt := range CorrectionAttempts {
		reply, err := ask(FixSystemPrompt, FixUserMessage(piece, attempt > 0))
		if err != nil {
			return "", err
		}
		candidate := CleanReply(piece, reply)
		switch JudgeCorrection(piece, candidate) {
		case Accept:
			return candidate, nil
		case RejectEmpty:
			refusal = ErrNoCorrection
		default:
			refusal = ErrCorrectionRefused
		}
	}
	return "", refusal
}
