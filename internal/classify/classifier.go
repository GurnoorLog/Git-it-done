package classify

import (
	"regexp"
	"strings"
	"unicode"
)

// Category represents one of the 8 task categories.
type Category int

const (
	CategoryMath Category = iota
	CategorySentiment
	CategorySummarization
	CategoryNER
	CategoryCodeDebugging
	CategoryCodeGeneration
	CategoryLogical
	CategoryFactual // fallback default
)

// String returns a human-readable label for structured logging.
func (c Category) String() string {
	switch c {
	case CategoryMath:
		return "math"
	case CategorySentiment:
		return "sentiment"
	case CategorySummarization:
		return "summarization"
	case CategoryNER:
		return "ner"
	case CategoryCodeDebugging:
		return "code_debugging"
	case CategoryCodeGeneration:
		return "code_generation"
	case CategoryLogical:
		return "logical"
	default:
		return "factual"
	}
}

// ---- compiled regexes (initialised once at package init) ----

var (
	// Math: numbers + arithmetic/percentage/ratio signal words.
	// FIXED: tighter — requires at least 2 numbers to reduce false positives
	// from summarization tasks that mention "2 sentences" or "3 bullet points".
	reMathNumbers = regexp.MustCompile(`\b\d+(?:\.\d+)?\b`)
	// FIXED: removed "if\s+\w+\s+then" (logic) and generic "rate" (ambiguous)
	reMathKeywords = regexp.MustCompile(`(?i)\b(percent|%|total cost|how many|how much|calculate|compute|final (?:price|cost|amount)|sum|ratio|increase by|decrease by|profit|loss|average|mean|discount|tax|interest|price|cost|revenue|growth|projection|divided by|multiplied by|times|minus|plus|per item|per unit|per year|salary|earn|spend|owe|budget)`)

	// Sentiment: prompt itself talks about tone/feeling/opinion.
	reSentiment = regexp.MustCompile(`(?i)\b(classify the sentiment|analyze the sentiment|detect the sentiment|what is the (?:overall )?sentiment|what is the (?:emotional )?tone|how does .{0,40} feel|is .{0,40} (?:positive|negative|neutral)|overall sentiment|emotional tone|how would you characterize the (?:attitude|tone)|characterize the (?:attitude|tone)|does .{0,30} express (?:positivity|negativity)|leans more toward|positive or negative|emotional (?:tone|attitude)|what (?:emotion|attitude|tone)|classify the opinion|what is the mood|describe the sentiment)`)

	// Summarization: summarize instruction + long source text indicator.
	reSummarizeInstruction = regexp.MustCompile(`(?i)\b(summarize|summarise|summary|condense|tl;?dr|abstract|boil this down|key (?:point|takeaway)|main point|in (?:one|two|three|\d+) sentences?|under \d+ words?|no more than|bullet points|shorten(?:ed)?|concise overview|brief (?:summary|abstract)|extract the main|write a brief)`)

	// reSummarizeSourceBlock matches prompts containing a block of text to compress (≥150 chars).
	reSummarizeSourceBlock = regexp.MustCompile(`(?s).{150,}`)

	// NER: extraction instruction or explicit mention of entity types.
	reNERInstruction = regexp.MustCompile(`(?i)\b(extract|identify|list all|find all|find every|find each|named entit|entity extraction|pull out|who are the people|what are the organizations|which locations|perform named entity)`)
	reNEREntityTypes = regexp.MustCompile(`(?i)\b(persons?|organizations?|locations?|places?|people|companies|compan|entities|entity type)`)

	// Code debugging: code block + error/bug signal.
	// Must have BOTH a code block AND an explicit debug signal.
	reCodeBlock   = regexp.MustCompile("(?s)(```[a-zA-Z]*\\n?.+?```|def |func |class |import |#include |public static|void |int main)")
	reDebugSignal = regexp.MustCompile(`(?i)\b(bug|fix|error|broken|doesn'?t work|doesn'?t compile|exception|crash|fails?|wrong output|debug|traceback|unexpected|incorrect|issue|problem|help me fix|what'?s wrong|why (?:is|does)|not working|find the (?:bug|error|problem|issue)|something is wrong)`)

	// Code generation: spec/write instruction, no existing buggy code to fix.
	reCodeGenInstruction = regexp.MustCompile(`(?i)\b(?:write a (?:function|class|program|script|method|module|python (?:function|script|class))|implement (?:a |an |the |this |that |the following |the given )?(?:\w+\s+){0,3}(?:function|class|algorithm|method|program|script|module|queue|stack|tree|sort|search|cache|list|map|heap)\b|create a (?:\w+\s+){0,2}(?:function|class|module|generator|script)\b|generate (?:code|a function|a class)|build a (?:function|class|api|module)|code that|function (?:that|which|to)|make a function|write (?:code|a generator))`)

	// Logical/deductive: constraint puzzle framing.
	reLogicConstraint = regexp.MustCompile(`(?i)\b(exactly one|if and only if|neither|either .{1,40} or \w+|not both|immediately (?:to the )?(?:left|right)|next to|adjacent to|must be (?:in|at|lying|sitting|playing)|cannot be (?:in|at|lying|sitting|playing)|always|never|constraint|puzzle|seating arrangement|deduce|conclude|follows that|therefore|thus|hence|who is in position|position \d+|seated in a (?:row|line)|standing in a (?:row|line)|stacked|tower of|barber|paradox|every card with|\bevery\b.{0,20}\bmust\b.{0,20}\bexactly\b|all labels are wrong|must (?:cross|transport|get all)|if it is raining|which of these conclusions must be true|every player must play|or both\b)`)
	reLogicPuzzleFrame = regexp.MustCompile(`(?i)(five people|six people|four people|three people|\w+ is (?:not )?(?:next to|in position|immediately)|who is in position \d+|list the order from|order from (?:top|bottom)|which sport does|what does \w+ prefer|exactly one of the following|if it is raining|all mammals are|every card with|which of these conclusions|how does he get all three)`)
)

// Classify assigns a Category to a prompt using combined signal matching.
// The order of checks matters: more specific categories are checked before
// the broad fallback (Factual).
func Classify(prompt string) Category {
	lower := strings.ToLower(prompt)

	// ── Code debugging (check before code-gen: bug in existing code takes priority)
	hasCodeBlock := reCodeBlock.MatchString(prompt)
	hasDebugSignal := reDebugSignal.MatchString(prompt)
	if hasCodeBlock && hasDebugSignal {
		return CategoryCodeDebugging
	}

	// ── Code generation (spec language + no buggy block to fix)
	hasCodeGenInstruction := reCodeGenInstruction.MatchString(prompt)
	if hasCodeGenInstruction && !hasDebugSignal {
		return CategoryCodeGeneration
	}

	// ── Summarization (check before Math to avoid catching '3 sentences' as Math)
	hasSummarizeInstruction := reSummarizeInstruction.MatchString(prompt)
	hasLongText := len([]rune(prompt)) > 300 || reSummarizeSourceBlock.MatchString(prompt)
	if hasSummarizeInstruction && hasLongText {
		return CategorySummarization
	}
	// Catch explicit summarize instructions even on medium-length text.
	if hasSummarizeInstruction && countWords(prompt) > 40 {
		return CategorySummarization
	}

	// ── Logical/deductive: strong puzzle framing required.
	// FIXED: check logic BEFORE math — a logic puzzle may mention numbers (positions)
	// but the key signal is the constraint/puzzle framing, not the numbers themselves.
	constraintCount := len(reLogicConstraint.FindAllString(prompt, -1))
	hasPuzzleFrame := reLogicPuzzleFrame.MatchString(prompt)
	if constraintCount >= 2 || (constraintCount >= 1 && hasPuzzleFrame) {
		return CategoryLogical
	}

	// ── Math: numbers present AND math keyword present.
	// FIXED: require at least 2 numbers to avoid classifying "3 sentences" as Math.
	numberMatches := reMathNumbers.FindAllString(lower, -1)
	hasMathKeyword := reMathKeywords.MatchString(prompt)
	if len(numberMatches) >= 2 && hasMathKeyword {
		return CategoryMath
	}

	// ── Sentiment: classification instruction required.
	if reSentiment.MatchString(prompt) {
		return CategorySentiment
	}

	// ── NER: extraction instruction + entity type mention.
	hasNERInstruction := reNERInstruction.MatchString(prompt)
	hasEntityTypes := reNEREntityTypes.MatchString(prompt)
	if hasNERInstruction && hasEntityTypes {
		return CategoryNER
	}

	_ = lower // suppress unused warning
	// ── Factual knowledge: default fallback.
	return CategoryFactual
}

// countWords returns a rough word count for the prompt.
func countWords(s string) int {
	count := 0
	inWord := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			inWord = false
		} else if !inWord {
			count++
			inWord = true
		}
	}
	return count
}
