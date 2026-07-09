package solvers

import (
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

// MathResult holds the outcome of a deterministic math solve attempt.
type MathResult struct {
	Answer    string
	Solved    bool   // false → caller should escalate
	Reasoning string // brief explanation for logging
}

// SolveMath attempts to parse and solve a math word problem deterministically.
// It returns Solved=false (with no answer) when the problem can't be
// confidently parsed — the caller MUST escalate rather than guess.
func SolveMath(prompt string) MathResult {
	lower := strings.ToLower(prompt)

	// Try each solver pattern in decreasing specificity.
	if r := solvePercentageOf(lower, prompt); r.Solved {
		// Phase 3 sanity: "X% of Y" result must be non-negative and <= Y.
		if !sanityCheckNonNegative(r.Answer) {
			return MathResult{Solved: false, Reasoning: "sanity check failed: result of X% of Y was negative; escalating"}
		}
		return r
	}
	if r := solvePercentageChange(lower, prompt); r.Solved {
		// Phase 3 sanity: discount/increase result must be non-negative.
		if !sanityCheckNonNegative(r.Answer) {
			return MathResult{Solved: false, Reasoning: "sanity check failed: percentage change result was negative; escalating"}
		}
		return r
	}
	if r := solveSimpleArithmetic(lower, prompt); r.Solved {
		if !sanityCheckNonNegative(r.Answer) {
			return MathResult{Solved: false, Reasoning: "sanity check failed: arithmetic result was negative; escalating"}
		}
		return r
	}
	if r := solveWorkRate(lower, prompt); r.Solved {
		if !sanityCheckNonNegative(r.Answer) {
			return MathResult{Solved: false, Reasoning: "sanity check failed: work-rate days was negative; escalating"}
		}
		return r
	}
	if r := solvePercentConsumed(lower, prompt); r.Solved {
		if !sanityCheckNonNegative(r.Answer) {
			return MathResult{Solved: false, Reasoning: "sanity check failed: percent consumed was negative; escalating"}
		}
		return r
	}
	if r := solveGrowthProjection(lower, prompt); r.Solved {
		if !sanityCheckNonNegative(r.Answer) {
			return MathResult{Solved: false, Reasoning: "sanity check failed: growth projection was negative; escalating"}
		}
		return r
	}

	return MathResult{Solved: false, Reasoning: "could not confidently parse problem structure"}
}

// sanityCheckNonNegative returns false if the answer string parses to a negative number.
// This catches wrong regex matches (e.g. subtracting the wrong pair of numbers).
func sanityCheckNonNegative(ans string) bool {
	// Strip currency symbols and commas before parsing.
	clean := strings.ReplaceAll(ans, "$", "")
	clean = strings.ReplaceAll(clean, ",", "")
	clean = strings.TrimSpace(strings.Fields(clean)[0]) // take first token (handles "45 days" etc)
	clean = strings.TrimSuffix(clean, ":")              // answers are formatted "<value>: <explanation>"
	var f float64
	if _, err := fmt.Sscanf(clean, "%f", &f); err != nil {
		return true // can't parse → don't block on it
	}
	return f >= 0
}

// ── Number extraction helpers ─────────────────────────────────────────────

var reNumber = regexp.MustCompile(`\$?([\d,]+(?:\.\d+)?)%?`)

// extractNumbers pulls all numeric values from text as *big.Rat.
func extractNumbers(s string) []*big.Rat {
	matches := reNumber.FindAllStringSubmatch(s, -1)
	var result []*big.Rat
	for _, m := range matches {
		clean := strings.ReplaceAll(m[1], ",", "")
		f, err := strconv.ParseFloat(clean, 64)
		if err != nil {
			continue
		}
		r := new(big.Rat).SetFloat64(f)
		result = append(result, r)
	}
	return result
}

// parseNumber parses a single numeric string to *big.Rat; nil on failure.
func parseNumber(s string) *big.Rat {
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimPrefix(s, "$")
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return new(big.Rat).SetFloat64(f)
}

// formatRat converts a *big.Rat to a human-friendly decimal string (up to 6 dp, trailing zeros trimmed).
func formatRat(r *big.Rat) string {
	f, _ := r.Float64()
	s := strconv.FormatFloat(f, 'f', 6, 64)
	// Trim trailing zeros and unnecessary decimal point.
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return s
}

// ── Pattern: "X% of Y" or "X percent of Y" ───────────────────────────────

var rePercentOf = regexp.MustCompile(`(?i)([\d,]+(?:\.\d+)?)\s*%?\s*(?:percent|%)\s+of\s+\$?([\d,]+(?:\.\d+)?)`)

func solvePercentageOf(lower, original string) MathResult {
	m := rePercentOf.FindStringSubmatch(original)
	if len(m) < 3 {
		return MathResult{}
	}
	pct := parseNumber(m[1])
	base := parseNumber(m[2])
	if pct == nil || base == nil {
		return MathResult{}
	}

	hundred := new(big.Rat).SetInt64(100)
	result := new(big.Rat).Mul(pct, base)
	result.Quo(result, hundred)

	ans := fmt.Sprintf("%.2f", mustFloat(result))
	return MathResult{
		Solved:    true,
		Answer:    fmt.Sprintf("%s: %s%% of %s = (%s/100) × %s = %s.", ans, m[1], m[2], m[1], m[2], ans),
		Reasoning: fmt.Sprintf("%s%% of %s = %s", m[1], m[2], ans),
	}
}

// ── Pattern: percentage increase/decrease ────────────────────────────────

var (
	reDiscountPrice  = regexp.MustCompile(`(?i)\$?([\d,]+(?:\.\d+)?).*?([\d,]+(?:\.\d+)?)\s*%\s*(?:discount|off|reduction)`)
	rePercentIncrease = regexp.MustCompile(`(?i)\$?([\d,]+(?:\.\d+)?).*?(?:raise|increas\w*|growth|more|by)\s*(?:a\s+)?([\d,]+(?:\.\d+)?)\s*%`)
)

// formatMoney formats a float as money with proper thousands separators.
func formatMoney(f float64) string {
	s := fmt.Sprintf("%.2f", f)
	parts := strings.SplitN(s, ".", 2)
	intPart := parts[0]
	neg := strings.HasPrefix(intPart, "-")
	intPart = strings.TrimPrefix(intPart, "-")
	var out []byte
	for i, d := range []byte(intPart) {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, d)
	}
	res := "$" + string(out) + "." + parts[1]
	if neg {
		res = "-" + res
	}
	return res
}

// hasMultiplePctOps returns true if the prompt mentions more than one percentage value.
// This guards against the discount/increase patterns incorrectly matching compound problems
// (e.g., markup THEN discount) where the regex grabs the wrong base number.
func hasMultiplePctOps(s string) bool {
	rePctVal := regexp.MustCompile(`\d+(?:\.\d+)?\s*%`)
	matches := rePctVal.FindAllString(s, -1)
	return len(matches) >= 2
}

func solvePercentageChange(lower, original string) MathResult {
	// Guard: if there are multiple distinct percentage values in the prompt (e.g., markup AND
	// discount), the simple discount/increase patterns can't reliably extract the right numbers.
	// Decline and let the model solver handle the compound problem correctly.
	if hasMultiplePctOps(original) {
		return MathResult{}
	}

	// Discount
	if m := reDiscountPrice.FindStringSubmatch(original); len(m) >= 3 {
		base := parseNumber(m[1])
		pct := parseNumber(m[2])
		if base != nil && pct != nil {
			hundred := new(big.Rat).SetInt64(100)
			discount := new(big.Rat).Mul(base, pct)
			discount.Quo(discount, hundred)
			result := new(big.Rat).Sub(base, discount)
			ans := formatMoney(mustFloat(result))
			discountAmt := formatMoney(mustFloat(discount))
			return MathResult{
				Solved:    true,
				Answer:    fmt.Sprintf("%s: a %s%% discount on $%s is %s, so the final price is $%s − %s = %s.", ans, m[2], m[1], discountAmt, m[1], discountAmt, ans),
				Reasoning: fmt.Sprintf("%s - %s%% = %s", m[1], m[2], ans),
			}
		}
	}
	// Increase / raise
	// Try the standard pattern where the word is before the percent ("increases by 20%")
	if m := rePercentIncrease.FindStringSubmatch(original); len(m) >= 3 {
		base := parseNumber(m[1])
		pct := parseNumber(m[2])
		if base != nil && pct != nil {
			hundred := new(big.Rat).SetInt64(100)
			inc := new(big.Rat).Mul(base, pct)
			inc.Quo(inc, hundred)
			result := new(big.Rat).Add(base, inc)
			ans := formatMoney(mustFloat(result))
			incAmt := formatMoney(mustFloat(inc))
			return MathResult{
				Solved:    true,
				Answer:    fmt.Sprintf("%s: a %s%% increase on $%s adds %s, giving $%s + %s = %s.", ans, m[2], m[1], incAmt, m[1], incAmt, ans),
				Reasoning: fmt.Sprintf("%s + %s%% = %s", m[1], m[2], ans),
			}
		}
	}
	
	// Try pattern where word is after percent ("10% raise")
	rePercentIncreaseAfter := regexp.MustCompile(`(?i)\$?([\d,]+(?:\.\d+)?).*?([\d,]+(?:\.\d+)?)\s*%\s*(?:raise|increas\w*|growth|more)`)
	if m := rePercentIncreaseAfter.FindStringSubmatch(original); len(m) >= 3 {
		base := parseNumber(m[1])
		pct := parseNumber(m[2])
		if base != nil && pct != nil {
			hundred := new(big.Rat).SetInt64(100)
			inc := new(big.Rat).Mul(base, pct)
			inc.Quo(inc, hundred)
			result := new(big.Rat).Add(base, inc)
			ans := formatMoney(mustFloat(result))
			incAmt := formatMoney(mustFloat(inc))
			return MathResult{
				Solved:    true,
				Answer:    fmt.Sprintf("%s: a %s%% increase on $%s adds %s, giving $%s + %s = %s.", ans, m[2], m[1], incAmt, m[1], incAmt, ans),
				Reasoning: fmt.Sprintf("%s + %s%% = %s", m[1], m[2], ans),
			}
		}
	}
	return MathResult{}
}

// ── Pattern: simple multiplication (N items at $P each) ──────────────────

var reItemsAtPrice = regexp.MustCompile(`(?i)([\d,]+)\s+(?:items?|units?|things?|products?)\s+at\s+\$?([\d,]+(?:\.\d+)?)\s+each`)

func solveSimpleArithmetic(lower, original string) MathResult {
	matches := reItemsAtPrice.FindAllStringSubmatch(original, -1)
	if len(matches) == 0 {
		return MathResult{}
	}
	total := new(big.Rat)
	for _, m := range matches {
		qty := parseNumber(m[1])
		price := parseNumber(m[2])
		if qty == nil || price == nil {
			return MathResult{}
		}
		sub := new(big.Rat).Mul(qty, price)
		total.Add(total, sub)
	}
	// Check for tax
	reTax := regexp.MustCompile(`(?i)([\d,]+(?:\.\d+)?)\s*%\s*tax`)
	if tm := reTax.FindStringSubmatch(original); len(tm) >= 2 {
		taxPct := parseNumber(tm[1])
		if taxPct != nil {
			hundred := new(big.Rat).SetInt64(100)
			tax := new(big.Rat).Mul(total, taxPct)
			tax.Quo(tax, hundred)
			total.Add(total, tax)
		}
	}
	ans := formatMoney(mustFloat(total))
	return MathResult{
		Solved:    true,
		Answer:    fmt.Sprintf("%s: multiplying each quantity by its unit price (and applying any stated tax) gives a total of %s.", ans, ans),
		Reasoning: "item-at-price multiplication",
	}
}

// ── Pattern: work-rate (N workers in D days → how many days for M workers) ──

var (
	reWorkRate = regexp.MustCompile(`(?i)([\d,]+)\s+workers?\s+(?:can|finish|complete).*?in\s+([\d,]+)\s+days?`)
	reWorkNew  = regexp.MustCompile(`(?i)([\d,]+)\s+workers?`)
)

func solveWorkRate(lower, original string) MathResult {
	m1 := reWorkRate.FindStringSubmatch(original)
	if len(m1) < 3 {
		return MathResult{}
	}
	w1 := parseNumber(m1[1])
	d1 := parseNumber(m1[2])
	if w1 == nil || d1 == nil {
		return MathResult{}
	}

	// Find the new number of workers.
	nums := extractNumbers(original)
	if len(nums) < 3 {
		return MathResult{}
	}
	// The third distinct number should be the new worker count.
	w2 := nums[2]
	if mustFloat(w2) == 0 {
		return MathResult{}
	}

	// Total work = w1 * d1; new days = total / w2
	totalWork := new(big.Rat).Mul(w1, d1)
	newDays := new(big.Rat).Quo(totalWork, w2)
	ans := formatRat(newDays) + " days"
	return MathResult{
		Solved:    true,
		Answer:    fmt.Sprintf("%s: the total work is %s workers × %s days = %s worker-days, so %s workers need %s.", ans, m1[1], m1[2], formatRat(totalWork), formatRat(w2), ans),
		Reasoning: fmt.Sprintf("%s workers × %s days ÷ %s workers = %s", m1[1], m1[2], formatRat(w2), ans),
	}
}

// ── Pattern: percent consumed (remove X% from total, possibly plus more) ──

var (
	rePctConsumed   = regexp.MustCompile(`(?i)([\d,]+(?:\.\d+)?)\s*%\s*(?:on|of|in|during|per|a\s+week|a\s+day|this\s+week|this\s+month|monday|tuesday|wednesday|thursday|friday|saturday|sunday)`)
	reTotalItems    = regexp.MustCompile(`(?i)(?:has|with|of|from|out of|starting|originally|initially|total\s+of)\s+([\d,]+(?:\.\d+)?)\s*(?:items?|units?|things?|products?|people|customers|members|total|overall)`)
	reRemainingWord = regexp.MustCompile(`(?i)(?:remain|left|remaining|still have|how many|how much)`)
	reSubNumber     = regexp.MustCompile(`(?i)(?:and|then|plus|also)\s+([\d,]+(?:\.\d+)?)\s+(?:more|additional|extra|\w+)`)
)

func solvePercentConsumed(lower, original string) MathResult {
	// Need a total/base number.
	totalM := reTotalItems.FindStringSubmatch(original)
	if len(totalM) < 2 {
		return MathResult{}
	}
	total := parseNumber(totalM[1])
	if total == nil {
		return MathResult{}
	}

	if !reRemainingWord.MatchString(lower) {
		return MathResult{}
	}

	// Find all percentage consumption events.
	pctMatches := rePctConsumed.FindAllStringSubmatch(original, -1)
	if len(pctMatches) == 0 {
		return MathResult{}
	}

	remaining := new(big.Rat).Set(total)
	hundred := new(big.Rat).SetInt64(100)

	for _, m := range pctMatches {
		pct := parseNumber(m[1])
		if pct == nil {
			return MathResult{}
		}
		consumed := new(big.Rat).Mul(total, pct)
		consumed.Quo(consumed, hundred)
		remaining.Sub(remaining, consumed)
		if mustFloat(remaining) < 0 {
			return MathResult{}
		}
	}

	// Find any additional absolute subtractions.
	for _, m := range reSubNumber.FindAllStringSubmatch(original, -1) {
		sub := parseNumber(m[1])
		if sub != nil {
			remaining.Sub(remaining, sub)
			if mustFloat(remaining) < 0 {
				return MathResult{}
			}
		}
	}

	ft := mustFloat(remaining)
	ans := fmt.Sprintf("%.0f", ft)

	var steps []string
	for _, m := range pctMatches {
		pctVal := m[1]
		amtRat := new(big.Rat).Mul(total, parseNumber(m[1]))
		amtRat.Quo(amtRat, hundred)
		steps = append(steps, fmt.Sprintf("%s%% of %s is %s", pctVal, totalM[1], formatRat(amtRat)))
	}

	extraSubs := reSubNumber.FindAllStringSubmatch(original, -1)
	if len(extraSubs) > 0 {
		for _, m := range extraSubs {
			steps = append(steps, fmt.Sprintf("then %s more are removed", m[1]))
		}
	}

	stepStr := strings.Join(steps, ", ")
	return MathResult{
		Solved:    true,
		Answer:    fmt.Sprintf("%s: starting with %s, %s, leaving %s.", ans, totalM[1], stepStr, ans),
		Reasoning: fmt.Sprintf("percent consumed from %s: %s", totalM[1], stepStr),
	}
}

// ── Pattern: compound growth projection (P * (1 + r%)^n) ─────────────────

var reGrowthBase = regexp.MustCompile(`(?i)([\d,]+(?:\.\d+)?)\s*(?:million|billion|thousand|people|users|customers)?.*?([\d,]+(?:\.\d+)?)\s*%\s*(?:per year|annually|annual|a year|growth|rate).*?(\d+)\s+years?`)

func solveGrowthProjection(lower, original string) MathResult {
	m := reGrowthBase.FindStringSubmatch(original)
	if len(m) < 4 {
		return MathResult{}
	}
	base := parseNumber(m[1])
	ratePct := parseNumber(m[2])
	years := parseNumber(m[3])
	if base == nil || ratePct == nil || years == nil {
		return MathResult{}
	}

	n, _ := years.Float64()
	r, _ := ratePct.Float64()
	b, _ := base.Float64()

	result := b
	for i := 0; i < int(n); i++ {
		result *= (1 + r/100)
	}

	// Detect scale word.
	scale := ""
	if strings.Contains(strings.ToLower(original), "million") {
		scale = " million"
	} else if strings.Contains(strings.ToLower(original), "billion") {
		scale = " billion"
	}

	ans := fmt.Sprintf("%.4g%s", result, scale)
	return MathResult{
		Solved:    true,
		Answer:    fmt.Sprintf("%s: compounding %.4g%s at %.4g%% per year for %v years gives %.4g × (1 + %.4g/100)^%v = %s.", ans, b, scale, r, n, b, r, n, ans),
		Reasoning: fmt.Sprintf("compound growth: %.4g × (1+%.4g%%)^%v = %s", b, r, n, ans),
	}
}

// mustFloat converts *big.Rat → float64 (panics only for nil, which would be a programming error).
func mustFloat(r *big.Rat) float64 {
	f, _ := r.Float64()
	return f
}
