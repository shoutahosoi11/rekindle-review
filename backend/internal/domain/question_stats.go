package domain

type QuestionStats struct {
	QuestionID   string
	AnswerCount  int
	CorrectCount int
}

func (s *QuestionStats) CorrectRate() float64 {
	if s.AnswerCount == 0 {
		return 0
	}
	return float64(s.CorrectCount) / float64(s.AnswerCount)
}
