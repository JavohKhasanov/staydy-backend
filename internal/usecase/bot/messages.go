package bot

// All bot copy is Uzbek (the students' language). Markdown is enabled on send.

// miniAppURL is the student Mini App the bot's "open app" buttons launch. (TODO: move to config
// when a second deployment needs a different domain.)
const miniAppURL = "https://student.staydy.uz"

// Push-only bot copy: the bot only sends reminders/nudges and always points back to the Mini App.
const (
	msgStart = "Assalomu alaykum! 🎓 *Staydy* — o'quv sarguzashtingiz shu yerda.\n\n" +
		"Darslar, uy vazifalar, reyting va haftalik holat — hammasi ilovada. Quyidagi tugma bilan oching 👇"
	msgUseApp         = "Bu bot faqat eslatma yuboradi. 📲 Hamma amal ilovada — quyidagi tugma bilan oching."
	msgWeeklyReminder = "🗓 Haftalik holatni belgilash vaqti keldi! Ilovaga kirib, check-in qiling. 💪"
	// %s = assignment title
	msgHomeworkDeadline = "⏰ *%s* — topshirish muddatiga *2 soat* qoldi!\nHali topshirmadingiz. Ilovaga kirib topshiring 👇"
)
