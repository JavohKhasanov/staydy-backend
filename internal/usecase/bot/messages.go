package bot

// All bot copy is Uzbek (the students' language). Markdown is enabled on send.

const (
	msgWelcome = "Assalomu alaykum, *%s*! 🎓\n\n" +
		"Siz o'quv markazingizning Student Success tizimiga ulandingiz. " +
		"Har hafta holatingizni baholab borasiz — bu mentorlaringizga sizga yaxshiroq yordam berishda yordam beradi."

	msgAlreadyLinked = "Siz allaqachon ulangansiz. ✅\nHaftalik holatni belgilash uchun /checkin yuboring."

	msgNotLinked = "Siz hali ulanmagansiz. 🔗\n" +
		"O'quv markazingiz bergan maxsus havola orqali ulaning."

	msgBadToken     = "Havola noto'g'ri yoki topilmadi. Markazdan to'g'ri havola so'rang."
	msgUsedToken    = "Bu havola allaqachon ishlatilgan. Markazdan yangi havola so'rang."
	msgExpiredToken = "Havola muddati tugagan. ⏳ Markazdan yangi havola so'rang."

	msgAskMotivation = "1️⃣ Bu hafta *motivatsiyangiz* qanchalik baland?\n_(1 — juda past, 5 — juda baland)_"
	msgAskProgress   = "2️⃣ *Progress* (o'zlashtirish) hissingiz qanday?\n_(1 — umuman yo'q, 5 — a'lo)_"
	msgAskObstacle   = "3️⃣ Bu hafta eng katta *to'sig'ingiz* nima edi?"
	msgAskComment    = "4️⃣ Qo'shimcha izoh yozmoqchimisiz? Yozing yoki /skip yuboring."

	msgDone = "Rahmat! ✅ Javoblaringiz qabul qilindi.\nKeyingi hafta yana /checkin bilan holatingizni belgilang. 💪"

	msgUseCheckin = "Holatni belgilash uchun /checkin yuboring."
	msgHelp       = "📋 *Buyruqlar:*\n/checkin — haftalik holatni belgilash\n/profil — holatim va tarixim\n/help — yordam"
	msgError      = "Kechirasiz, xatolik yuz berdi. Birozdan so'ng qayta urinib ko'ring."
)
