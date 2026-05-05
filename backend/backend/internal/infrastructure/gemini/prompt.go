package gemini

import (
	"fmt"

	"github.com/shout/ai-study-tool/backend/internal/domain"
)

func BuildBatchGeneratorPrompt(points []domain.ExtractedPoint, questionType domain.QuestionType, customInstruction string) string {
	typeInstruction := ""
	switch questionType {
	case domain.QuestionTypeMultipleChoice:
		typeInstruction = "すべて4択選択問題にしてください。各 questions[i].options には必ず4つの選択肢を含めてください。"
	default:
		typeInstruction = "すべて4択選択問題にしてください。"
	}

	customPart := ""
	if customInstruction != "" {
		customPart = fmt.Sprintf("\n追加指示: %s", customInstruction)
	}

	var pointsSection string
	for index, point := range points {
		pointsSection += fmt.Sprintf("\n%d.\nハイライト本文: %s\nユーザー解説: %s\n", index+1, point.Point, point.Context)
	}

	return fmt.Sprintf(`以下の複数ハイライトから学習用の問題をまとめて作成してください。
各ハイライトにつき1問ずつ作り、順番を保ってください。
「ハイライト本文」を主情報として使い、「ユーザー解説」がある場合は補助情報として使ってください。

素材一覧:%s

%s%s

以下のJSON形式で回答してください。他のテキストは含めないでください:
{
  "questions": [
    {
      "content": "問題文",
      "options": ["選択肢A", "選択肢B", "選択肢C", "選択肢D"],
      "correct_answer": "正解の選択肢（選択問題の場合）または模範解答（記述の場合）",
      "explanation": "解説文"
    }
  ]
}`, pointsSection, typeInstruction, customPart)
}
