# Staydy — Production Deploy (hammasi bitta serverda)

Bitta Linux server + **Nginx Proxy Manager** (GUI, Docker) + Docker. Backend + Postgres + 3 frontend
docker-compose bilan ishlaydi; NPM TLS + marshrutlashni bajaradi.

```
Internet ─► Nginx Proxy Manager (TLS) ─┬─ staydy.uz        → 172.17.0.1:3010  (landing)
                                        ├─ app.staydy.uz    → 172.17.0.1:3011  (markaz app)
                                        ├─ admin.staydy.uz  → 172.17.0.1:3012  (superadmin)
                                        └─ api.staydy.uz    → 172.17.0.1:8090  (Go backend)
docker-compose: postgres (ochilmagan) + migrate + api + landing + app + admin
```

---

## 0. Server tayyorligi (bir marta)
Docker + NPM allaqachon o'rnatilgan. Faqat git kerak bo'lishi mumkin:
```bash
docker --version && docker compose version   # bo'lmasa: curl -fsSL https://get.docker.com | sh
sudo apt update && sudo apt install -y git
```

## 1. DNS (domen provayderda)
staydy.uz uchun **A-yozuvlar** — hammasi server IP'ga:
```
staydy.uz         A   <SERVER_IP>
www.staydy.uz     A   <SERVER_IP>
app.staydy.uz     A   <SERVER_IP>
admin.staydy.uz   A   <SERVER_IP>
api.staydy.uz     A   <SERVER_IP>
```
Tarqalishini kuting (`dig app.staydy.uz +short` → server IP).

## 2. Repolarni klonlash (siblings sifatida)
```bash
sudo mkdir -p /opt/staydy && sudo chown $USER /opt/staydy && cd /opt/staydy
git clone <BACKEND_REPO_URL>      staydy-backend
git clone <LANDING_REPO_URL>      staydy-website
git clone <APP_REPO_URL>          staydy-app
git clone <SUPERADMIN_REPO_URL>   staydy-superadmin
```
> Muhim: 4 papka bir xil `/opt/staydy/` ichida yonma-yon turishi kerak (compose `../` bilan quradi).

## 3. Backend maxfiy kalitlari
```bash
cd /opt/staydy/staydy-backend
cp .env.prod.example .env
# .env'ni tahrirlang:
#   DB_PASSWORD      = openssl rand -base64 24
#   JWT_SECRET       = openssl rand -base64 48
#   GEMINI_API_KEY   = haqiqiy kalit
#   TELEGRAM_BOT_TOKEN = bot tokeni (yoki bo'sh)
#   CORS_ORIGINS     = https://staydy.uz,https://app.staydy.uz,https://admin.staydy.uz
nano .env
```

## 4. Stack'ni ishga tushirish
```bash
docker compose -f docker-compose.prod.yml up -d --build
docker compose -f docker-compose.prod.yml ps      # hammasi "running" / "healthy"
docker compose -f docker-compose.prod.yml logs -f api
```
Bu: Postgres'ni ko'taradi → migratsiyalarni qo'llaydi (+ RLS "app" roli, superadmin seed) → api + 3 frontendni quradi va ishga tushiradi. Portlar `172.17.0.1` (docker gateway) da — Nginx Proxy Manager shu orqali yetadi, tashqi internetga ochiq emas.

## 5. Nginx Proxy Manager — 4 ta Proxy Host qo'shing (GUI)
Serverда NPM bor (raw nginx/certbot ISHLATILMAYDI). NPM GUI → **Hosts → Proxy Hosts → Add Proxy Host**, har domen uchun:

| Domain Names | Forward Host/IP | Forward Port |
|---|---|---|
| `staydy.uz`, `www.staydy.uz` | `172.17.0.1` | `3010` |
| `app.staydy.uz` | `172.17.0.1` | `3011` |
| `admin.staydy.uz` | `172.17.0.1` | `3012` |
| `api.staydy.uz` | `172.17.0.1` | `8090` |

Har host uchun:
- **Details** tab: Scheme = `http`, Forward Hostname/IP = `172.17.0.1`, Forward Port = (jadval), **Websockets Support = ON**, Block Common Exploits = ON
- **SSL** tab: SSL Certificate = **Request a new SSL Certificate** (Let's Encrypt), **Force SSL = ON**, HTTP/2 = ON, email + Agree to TOS
- (api.staydy.uz uchun) **Advanced** tab ixtiyoriy: `client_max_body_size 12m;` (CSV import uchun)

> Portlar mavjud chorbog/uzcode (3000/8081/8083/8085) bilan konfliktsiz tanlangan (3010/3011/3012/8090).

## 7. Tekshirish
```bash
curl -I https://staydy.uz            # 200
curl -s https://api.staydy.uz/api/v1/health   # (yoki superadmin login)
```
Brauzerda: `staydy.uz` (landing forma → superadmin "So'rovlar"), `app.staydy.uz`, `admin.staydy.uz`.

## 8. Xavfsizlik (birinchi kirishdan keyin)
- **Superadmin parolini DARROV o'zgartiring.** Seed hisobi `xadminsup@gmail.com`, boshlang'ich parol migratsiya faylida ochiq turadi (`db/migrations/20260630130000_seed_superadmin.sql`) — shuning uchun uni birinchi kirishdayoq almashtiring. `admin.staydy.uz` panelida yon menyudagi **"Parolni o'zgartirish"** orqali (yoki `POST /api/v1/me/change-password`). Almashtirgach seeddagi qiymat ishlamay qoladi.
- **DB backuplari** avtomatik: `backup` xizmati har kuni `pg_dump` qiladi (`./backups`, 14 kun saqlanadi). Haqiqiy falokatga qarshi backuplarni serverdan tashqariga (rsync/S3) ko'chirib turing.
- Staydy portlari `172.17.0.1` da (docker gateway) — tashqi internetga ochiq emas, faqat NPM yetadi. Postgres umuman port ochmaydi.

---

## Yangilash (har safar kod o'zgarganda)
```bash
cd /opt/staydy/staydy-backend
./deploy/deploy.sh        # 4 repo pull + rebuild + restart + prune
```

## Foydali buyruqlar
```bash
docker compose -f docker-compose.prod.yml logs -f api|landing|app|admin
docker compose -f docker-compose.prod.yml restart api
docker compose -f docker-compose.prod.yml down          # to'xtatish (ma'lumot saqlanadi)
docker system df                                        # disk ishlatilishi
# Postgres backup:
docker compose -f docker-compose.prod.yml exec postgres pg_dump -U ssp ssp > backup_$(date +%F).sql
```

## Disk hajmi (savolingizga javob)
Distroless api (~20MB) + postgres (~80MB) + 3 node-alpine frontend (~150MB har biri, umumiy layerlar bo'lishadi).
Jami ~0.5–1GB — oddiy VPS (20GB+) uchun kichik. `docker image prune -f` eskilarni tozalaydi (deploy.sh avtomatik qiladi).

## Frontendlarni Cloudflare'ga ko'chirmoqchi bo'lsangiz (keyinroq)
Serverni yanada yengil qilish uchun 3 frontendni Cloudflare Pages'ga (bepul) o'tkazib, faqat `api.staydy.uz`ni serverda qoldirishingiz mumkin — compose'dan landing/app/admin xizmatlarini olib tashlang, nginx'da faqat api qoldiring.
