package persistence

import "testing"

func TestAnswerStatDeltas(t *testing.T) {
	tests := []struct {
		name              string
		hadPreviousAnswer bool
		previousCorrect   bool
		currentCorrect    bool
		wantAnswerDelta   int
		wantCorrectDelta  int
	}{
		{name: "new incorrect", currentCorrect: false, wantAnswerDelta: 1, wantCorrectDelta: 0},
		{name: "new correct", currentCorrect: true, wantAnswerDelta: 1, wantCorrectDelta: 1},
		{name: "repeat incorrect", hadPreviousAnswer: true, previousCorrect: false, currentCorrect: false, wantAnswerDelta: 0, wantCorrectDelta: 0},
		{name: "repeat correct", hadPreviousAnswer: true, previousCorrect: true, currentCorrect: true, wantAnswerDelta: 0, wantCorrectDelta: 0},
		{name: "incorrect to correct", hadPreviousAnswer: true, previousCorrect: false, currentCorrect: true, wantAnswerDelta: 0, wantCorrectDelta: 1},
		{name: "correct to incorrect", hadPreviousAnswer: true, previousCorrect: true, currentCorrect: false, wantAnswerDelta: 0, wantCorrectDelta: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAnswerDelta, gotCorrectDelta := answerStatDeltas(tt.hadPreviousAnswer, tt.previousCorrect, tt.currentCorrect)
			if gotAnswerDelta != tt.wantAnswerDelta || gotCorrectDelta != tt.wantCorrectDelta {
				t.Fatalf("answerStatDeltas() = (%d, %d), want (%d, %d)", gotAnswerDelta, gotCorrectDelta, tt.wantAnswerDelta, tt.wantCorrectDelta)
			}
		})
	}
}
