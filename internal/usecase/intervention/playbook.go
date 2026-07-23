package intervention

import "strings"

// playbookRule maps a risk-reason keyword to a concrete next action for staff. The EWS turns a
// "why at risk" into a "what to do" so an administrator doesn't have to guess the intervention.
type playbookRule struct {
	keyword string
	action  string
}

// Ordered so the most urgent/actionable advice surfaces first. Matched case-insensitively as a
// substring of each reason (reasons are the Uzbek risk-factor labels).
var playbookRules = []playbookRule{
	{"keskin oshdi", "Zudlik bilan bog'laning — holat keskin yomonlashdi, sababini bugun aniqlang."},
	{"ketma-ket", "Talaba va ota-onasiga darhol qo'ng'iroq qiling; kelmayotgan sababini va qaytish sanasini aniqlang."},
	{"davomat", "Talaba/ota-ona bilan bog'lanib davomat pasayishi sababini so'rang; qatnashish rejasini kelishing."},
	{"qarz", "Moliya bo'limi bilan bog'laning; to'lov jadvali yoki bo'lib-to'lashni taklif qiling."},
	{"uy vazifa", "O'qituvchi bilan vazifa qiyinchiligini muhokama qiling; qo'shimcha mashg'ulot yoki yordam bering."},
	{"motivatsiya", "Talaba bilan qisqa maqsad-suhbat o'tkazing; kichik erishiladigan yutuqlarni belgilang."},
	{"progress", "O'qituvchidan progress hisobotini oling; kutilma bilan real natijani moslang."},
	{"ishonch", "Mentor biriktiring; talabaning uzoq-muddatli maqsadini qayta aniqlang."},
}

// SuggestedActions returns concrete, de-duplicated next steps for a task's reasons. Falls back to a
// generic action when no keyword matches so a task is never left without guidance.
func SuggestedActions(reasons []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, rule := range playbookRules {
		for _, r := range reasons {
			if strings.Contains(strings.ToLower(r), rule.keyword) && !seen[rule.action] {
				seen[rule.action] = true
				out = append(out, rule.action)
				break
			}
		}
	}
	if len(out) == 0 {
		out = append(out, "Talaba bilan bog'laning va holatini aniqlang.")
	}
	return out
}
